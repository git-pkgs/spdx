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
	value   string
	isOp    bool // AND, OR, WITH
	isParen bool // ( or )
	isPlus  bool // +
}

// tokenizeForNormalization splits the expression into tokens, identifying operators and parens.
func tokenizeForNormalization(expr string) []tokenForNorm {
	tokens := make([]tokenForNorm, 0, strings.Count(expr, " ")+1)
	wordStart := -1

	flush := func(end int) {
		if wordStart < 0 {
			return
		}
		word := expr[wordStart:end]
		switch {
		case strings.EqualFold(word, opAND):
			tokens = append(tokens, tokenForNorm{value: opAND, isOp: true})
		case strings.EqualFold(word, opOR):
			tokens = append(tokens, tokenForNorm{value: opOR, isOp: true})
		case strings.EqualFold(word, opWITH):
			tokens = append(tokens, tokenForNorm{value: opWITH, isOp: true})
		default:
			tokens = append(tokens, tokenForNorm{value: word})
		}
		wordStart = -1
	}

	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		switch {
		case ch == '(':
			flush(i)
			tokens = append(tokens, tokenForNorm{value: "(", isParen: true})
		case ch == ')':
			flush(i)
			tokens = append(tokens, tokenForNorm{value: ")", isParen: true})
		case ch == '+':
			flush(i)
			tokens = append(tokens, tokenForNorm{value: "+", isPlus: true})
		case isSpaceByte(ch):
			flush(i)
		default:
			if wordStart < 0 {
				wordStart = i
			}
		}
	}
	flush(len(expr))

	return tokens
}

func isSpaceByte(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return unicode.IsSpace(rune(character))
	}
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
	canonical := lookupException(exc)
	if canonical == "" {
		// Try the original form
		exc = strings.Join(n.licenseWords, " ")
		canonical = lookupException(exc)
		if canonical == "" {
			return &LicenseError{License: exc, Err: ErrInvalidException}
		}
	}

	n.result.WriteString(" ")
	n.result.WriteString(canonical)
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
const maxLicenseWords = 256

func normalizeLicenseWords(words []string) (string, error) {
	if len(words) == 0 {
		return "", ErrMissingOperand
	}
	if len(words) > maxLicenseWords {
		return "", &LicenseError{License: words[0], Err: ErrInvalidLicenseID}
	}

	// Check for special values, LicenseRef or DocumentRef first
	if len(words) == 1 {
		// Pass through special values
		if strings.EqualFold(words[0], "NONE") {
			return "NONE", nil
		}
		if strings.EqualFold(words[0], "NOASSERTION") {
			return "NOASSERTION", nil
		}
		if hasPrefixFold(words[0], "LicenseRef-") || hasPrefixFold(words[0], "DocumentRef-") {
			return words[0], nil
		}
	}

	joined := strings.Join(words, " ")
	var starts [maxLicenseWords]int
	offset := 0
	for i, word := range words {
		starts[i] = offset
		offset += len(word) + 1
	}

	// Try to match progressively longer spans from the start.
	var result strings.Builder
	var firstResult string
	i := 0

	for i < len(words) {
		matched := false

		// Try longest span first, working backwards
		for end := len(words); end > i; end-- {
			candidateEnd := starts[end-1] + len(words[end-1])
			candidate := joined[starts[i]:candidateEnd]

			// Try direct normalization
			normalized, err := Normalize(candidate)
			if err == nil {
				firstResult = appendNormalizedResult(&result, firstResult, normalized)
				i = end
				matched = true
				break
			}

			// Try with + suffix handling
			if strings.HasSuffix(candidate, "+") {
				base := strings.TrimSuffix(candidate, "+")
				normalized, err := Normalize(base)
				if err == nil {
					firstResult = appendNormalizedResult(
						&result,
						firstResult,
						upgradeGPL(normalized+"+"),
					)
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

	if result.Len() == 0 {
		return firstResult, nil
	}
	return result.String(), nil
}

func appendNormalizedResult(result *strings.Builder, first, normalized string) string {
	if first == "" {
		return normalized
	}
	if result.Len() == 0 {
		result.Grow(len(first) + len(normalized) + 1)
		result.WriteString(first)
	}
	result.WriteByte(' ')
	result.WriteString(normalized)
	return first
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
