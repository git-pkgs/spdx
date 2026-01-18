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
			case "AND", "OR", "WITH":
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

// normalizeTokens processes tokens and normalizes informal license names.
func normalizeTokens(tokens []tokenForNorm) (string, error) {
	var result strings.Builder
	var licenseWords []string
	expectException := false // true if we just saw WITH

	flushLicense := func() error {
		if len(licenseWords) == 0 {
			return nil
		}

		normalized, err := normalizeLicenseWords(licenseWords)
		if err != nil {
			return err
		}

		if result.Len() > 0 && !strings.HasSuffix(result.String(), "(") {
			result.WriteString(" ")
		}
		result.WriteString(normalized)
		licenseWords = nil
		return nil
	}

	flushException := func() error {
		if len(licenseWords) == 0 {
			return nil
		}

		// Exception should be a single valid exception ID
		exc := strings.Join(licenseWords, "-")
		if lookupException(exc) == "" {
			// Try the original form
			exc = strings.Join(licenseWords, " ")
			if lookupException(exc) == "" {
				return &LicenseError{License: exc, Err: ErrInvalidException}
			}
		}

		result.WriteString(" ")
		result.WriteString(lookupException(exc))
		licenseWords = nil
		return nil
	}

	for _, tok := range tokens {
		if tok.isOp {
			if expectException {
				if err := flushException(); err != nil {
					return "", err
				}
				expectException = false
			} else {
				if err := flushLicense(); err != nil {
					return "", err
				}
			}
			result.WriteString(" ")
			result.WriteString(tok.value)
			if tok.value == "WITH" {
				expectException = true
			}
		} else if tok.isParen {
			if expectException {
				if err := flushException(); err != nil {
					return "", err
				}
				expectException = false
			} else {
				if err := flushLicense(); err != nil {
					return "", err
				}
			}
			if tok.value == "(" {
				if result.Len() > 0 && !strings.HasSuffix(result.String(), "(") && !strings.HasSuffix(result.String(), " ") {
					result.WriteString(" ")
				}
				result.WriteString("(")
			} else {
				result.WriteString(")")
			}
		} else if tok.isPlus {
			// Plus attaches to previous license word
			if len(licenseWords) > 0 {
				licenseWords[len(licenseWords)-1] += "+"
			}
		} else {
			// License word (or exception word if expectException)
			licenseWords = append(licenseWords, tok.value)
		}
	}

	if expectException {
		if err := flushException(); err != nil {
			return "", err
		}
	} else {
		if err := flushLicense(); err != nil {
			return "", err
		}
	}

	return strings.TrimSpace(result.String()), nil
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
