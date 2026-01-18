// Package spdx provides SPDX license expression parsing, normalization, and validation.
// It normalizes informal license strings (like "Apache 2" or "MIT License") to valid
// SPDX identifiers (like "Apache-2.0" or "MIT"), and validates/parses SPDX expressions.
package spdx

import (
	"errors"
	"strings"

	"github.com/github/go-spdx/v2/spdxexp"
)

// ErrInvalidLicense is returned when a license string cannot be normalized or validated.
var ErrInvalidLicense = errors.New("invalid license")

// Normalize converts an informal license string to a valid SPDX identifier.
// It handles common variations like "Apache 2", "MIT License", "GPL v3", etc.
// Returns the normalized SPDX identifier or an error if normalization fails.
//
// Example:
//
//	Normalize("Apache 2")           // returns "Apache-2.0", nil
//	Normalize("MIT License")        // returns "MIT", nil
//	Normalize("GPL v3")             // returns "GPL-3.0-or-later", nil
//	Normalize("UNKNOWN-LICENSE")    // returns "", ErrInvalidLicense
func Normalize(license string) (string, error) {
	license = strings.TrimSpace(license)
	if license == "" {
		return "", ErrInvalidLicense
	}

	// Try exact match first (case-insensitive)
	if id := lookupLicense(license); id != "" {
		return upgradeGPL(id), nil
	}

	// Try with trailing + removed, then upgrade the result
	noPlus := strings.TrimSuffix(strings.TrimSpace(license), "+")
	if noPlus != license {
		if id := lookupLicense(noPlus); id != "" {
			return upgradeGPL(id + "+"), nil
		}
	}

	// Apply transforms
	if result := tryTransforms(license); result != "" {
		return result, nil
	}

	// Apply transpositions with transforms
	if result := tryTranspositions(license); result != "" {
		return result, nil
	}

	// Last resort: substring matching
	if result := tryLastResorts(license); result != "" {
		return result, nil
	}

	// Transpositions with last resorts
	if result := tryTranspositionsWithLastResorts(license); result != "" {
		return result, nil
	}

	return "", ErrInvalidLicense
}

// NormalizeExpression normalizes an SPDX expression, converting each license
// identifier to its canonical form and ensuring proper operator precedence.
// This only handles case normalization of already-valid SPDX identifiers.
// For informal license names like "Apache 2", use NormalizeExpressionLax.
//
// Example:
//
//	NormalizeExpression("mit OR apache-2.0")
//	// returns "MIT OR Apache-2.0", nil
//
//	NormalizeExpression("mit OR gpl-2.0 AND apache-2.0")
//	// returns "MIT OR (GPL-2.0 AND Apache-2.0)", nil
func NormalizeExpression(expression string) (string, error) {
	expr, err := Parse(expression)
	if err != nil {
		return "", err
	}
	return expr.String(), nil
}

// NormalizeExpressionLax normalizes an SPDX expression with lax handling of
// informal license names. It converts informal names like "Apache 2" or
// "MIT License" to their canonical SPDX forms within expressions.
//
// Example:
//
//	NormalizeExpressionLax("Apache 2 OR MIT License")
//	// returns "Apache-2.0 OR MIT", nil
//
//	NormalizeExpressionLax("GPL v3 AND BSD 3-Clause")
//	// returns "GPL-3.0-or-later AND BSD-3-Clause", nil
func NormalizeExpressionLax(expression string) (string, error) {
	expr, err := ParseLax(expression)
	if err != nil {
		return "", err
	}
	return expr.String(), nil
}

// Valid checks if the given string is a valid SPDX expression.
// This performs strict validation - informal license names like "Apache 2" are not valid.
// Returns true if valid, false otherwise.
func Valid(expression string) bool {
	_, err := ParseStrict(expression)
	return err == nil
}

// ValidLicense checks if the given string is a valid SPDX license identifier.
// Returns true if valid, false otherwise.
func ValidLicense(license string) bool {
	return lookupLicense(license) != ""
}

// Satisfies checks if the allowed licenses satisfy the given SPDX expression.
// This is a convenience wrapper around github.com/github/go-spdx/v2/spdxexp.Satisfies.
func Satisfies(expression string, allowed []string) (bool, error) {
	return spdxexp.Satisfies(expression, allowed)
}

// ExtractLicenses extracts all unique license identifiers from an SPDX expression.
// Returns a slice of license identifiers or an error if parsing fails.
//
// Example:
//
//	ExtractLicenses("MIT OR Apache-2.0")
//	// returns ["MIT", "Apache-2.0"], nil
//
//	ExtractLicenses("(MIT AND GPL-2.0) OR Apache-2.0")
//	// returns ["Apache-2.0", "GPL-2.0", "MIT"], nil
func ExtractLicenses(expression string) ([]string, error) {
	return spdxexp.ExtractLicenses(expression)
}

// ValidateLicenses checks if all given license identifiers are valid SPDX identifiers.
// Returns true and nil if all are valid, or false and the list of invalid licenses.
func ValidateLicenses(licenses []string) (bool, []string) {
	return spdxexp.ValidateLicenses(licenses)
}
