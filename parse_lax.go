package spdx

import (
	"strings"
	"unicode"
)

// ParseLax parses an SPDX expression with lax handling of informal license names.
// It normalizes informal license strings like "Apache 2", "MIT License", "GPL v3".
//
// Deprecated: Use Parse instead, which now handles informal license names automatically.
// ParseLax is kept for backwards compatibility.
//
// Example:
//
//	ParseLax("Apache 2 OR MIT License")  // "Apache-2.0 OR MIT"
//	ParseLax("GPL v3 AND BSD 3-Clause")  // "GPL-3.0-or-later AND BSD-3-Clause"
func ParseLax(expression string) (Expression, error) {
	return Parse(expression)
}

// normalizeExpressionString normalizes informal license names in an expression string.
// It preserves AND, OR, WITH operators and parentheses.
func normalizeExpressionString(expr string) (string, error) {
	tokens := tokenizeForNormalization(expr)
	return normalizeTokens(tokens)
}

// tokenForNorm represents a token during normalization.
type tokenForNorm struct {
	value    string
	isOp     bool // AND, OR, WITH
	isParen  bool // ( or )
	isPlus   bool // +
}

// tokenizeForNormalization splits the expression into tokens, identifying operators and parens.
func tokenizeForNormalization(expr string) []tokenForNorm {
	var tokens []tokenForNorm
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			word := current.String()
			upper := strings.ToUpper(word)
			switch upper {
			case opAND, opOR, opWITH:
				tokens = append(tokens, tokenForNorm{value: upper, isOp: true})
			default:
				tokens = append(tokens, tokenForNorm{value: word})
			}
			current.Reset()
		}
	}

	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		switch {
		case ch == '(':
			flush()
			tokens = append(tokens, tokenForNorm{value: "(", isParen: true})
		case ch == ')':
			flush()
			tokens = append(tokens, tokenForNorm{value: ")", isParen: true})
		case ch == '+':
			flush()
			tokens = append(tokens, tokenForNorm{value: "+", isPlus: true})
		case unicode.IsSpace(rune(ch)):
			flush()
		default:
			current.WriteByte(ch)
		}
	}
	flush()

	return tokens
}

// tokenNormalizer holds state for normalizing a stream of tokens.
type tokenNormalizer struct {
	result          strings.Builder
	licenseWords    []string
	expectException bool
}

func (n *tokenNormalizer) flushPending() error {
	if n.expectException {
		return n.flushException()
	}
	return n.flushLicense()
}

func (n *tokenNormalizer) flushLicense() error {
	if len(n.licenseWords) == 0 {
		return nil
	}

	normalized, err := normalizeLicenseWords(n.licenseWords)
	if err != nil {
		return err
	}

	if n.result.Len() > 0 && !strings.HasSuffix(n.result.String(), "(") {
		n.result.WriteString(" ")
	}
	n.result.WriteString(normalized)
	n.licenseWords = nil
	return nil
}

func (n *tokenNormalizer) flushException() error {
	if len(n.licenseWords) == 0 {
		return nil
	}

	// Exception should be a single valid exception ID
	exc := strings.Join(n.licenseWords, "-")
	if lookupException(exc) == "" {
		// Try the original form
		exc = strings.Join(n.licenseWords, " ")
		if lookupException(exc) == "" {
			return &LicenseError{License: exc, Err: ErrInvalidException}
		}
	}

	n.result.WriteString(" ")
	n.result.WriteString(lookupException(exc))
	n.licenseWords = nil
	return nil
}

func (n *tokenNormalizer) handleOp(tok tokenForNorm) error {
	if err := n.flushPending(); err != nil {
		return err
	}
	n.expectException = false
	n.result.WriteString(" ")
	n.result.WriteString(tok.value)
	if tok.value == opWITH {
		n.expectException = true
	}
	return nil
}

func (n *tokenNormalizer) handleParen(tok tokenForNorm) error {
	if err := n.flushPending(); err != nil {
		return err
	}
	n.expectException = false
	if tok.value == "(" {
		if n.result.Len() > 0 && !strings.HasSuffix(n.result.String(), "(") && !strings.HasSuffix(n.result.String(), " ") {
			n.result.WriteString(" ")
		}
		n.result.WriteString("(")
	} else {
		n.result.WriteString(")")
	}
	return nil
}

// normalizeTokens processes tokens and normalizes informal license names.
func normalizeTokens(tokens []tokenForNorm) (string, error) {
	n := &tokenNormalizer{}

	for _, tok := range tokens {
		var err error
		switch {
		case tok.isOp:
			err = n.handleOp(tok)
		case tok.isParen:
			err = n.handleParen(tok)
		case tok.isPlus:
			if len(n.licenseWords) > 0 {
				n.licenseWords[len(n.licenseWords)-1] += "+"
			}
		default:
			n.licenseWords = append(n.licenseWords, tok.value)
		}
		if err != nil {
			return "", err
		}
	}

	if err := n.flushPending(); err != nil {
		return "", err
	}

	return strings.TrimSpace(n.result.String()), nil
}

// normalizeLicenseWords takes a slice of words that should form a license name
// and tries to normalize them. It uses greedy matching from the start.
func normalizeLicenseWords(words []string) (string, error) {
	if len(words) == 0 {
		return "", ErrMissingOperand
	}

	// Check for special values, LicenseRef or DocumentRef first
	if len(words) == 1 {
		upper := strings.ToUpper(words[0])
		// Pass through special values
		if upper == "NONE" || upper == "NOASSERTION" {
			return upper, nil
		}
		if strings.HasPrefix(upper, "LICENSEREF-") || strings.HasPrefix(upper, "DOCUMENTREF-") {
			return words[0], nil
		}
	}

	// Try to match progressively longer spans from the start
	var results []string
	i := 0

	for i < len(words) {
		matched := false

		// Try longest span first, working backwards
		for end := len(words); end > i; end-- {
			candidate := strings.Join(words[i:end], " ")

			// Try direct normalization
			normalized, err := Normalize(candidate)
			if err == nil {
				results = append(results, normalized)
				i = end
				matched = true
				break
			}

			// Try with + suffix handling
			if strings.HasSuffix(candidate, "+") {
				base := strings.TrimSuffix(candidate, "+")
				normalized, err := Normalize(base)
				if err == nil {
					results = append(results, upgradeGPL(normalized+"+"))
					i = end
					matched = true
					break
				}
			}
		}

		if !matched {
			// Single word didn't normalize - it's invalid
			return "", &LicenseError{License: words[i], Err: ErrInvalidLicenseID}
		}
	}

	return strings.Join(results, " "), nil
}

// LicenseError wraps an error with the license that caused it.
type LicenseError struct {
	License string
	Err     error
}

func (e *LicenseError) Error() string {
	return e.Err.Error() + ": " + e.License
}

func (e *LicenseError) Unwrap() error {
	return e.Err
}
