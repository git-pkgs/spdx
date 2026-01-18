package spdx

import (
	"testing"
)

// Test cases for lax parsing - informal license strings within expressions
var laxParseTests = map[string]string{
	// Simple cases - already valid SPDX
	"MIT":                        "MIT",
	"MIT OR Apache-2.0":          "MIT OR Apache-2.0",
	"MIT AND Apache-2.0":         "MIT AND Apache-2.0",

	// Case normalization (strict mode handles this too)
	"mit OR apache-2.0":          "MIT OR Apache-2.0",

	// Informal license names in expressions
	"Apache 2 OR MIT":                        "Apache-2.0 OR MIT",
	"MIT OR Apache 2":                        "MIT OR Apache-2.0",
	"Apache 2 OR MIT License":                "Apache-2.0 OR MIT",
	"MIT License OR Apache 2":                "MIT OR Apache-2.0",
	"GPL v3 OR MIT":                          "GPL-3.0-or-later OR MIT",
	"MIT OR GPL v3":                          "MIT OR GPL-3.0-or-later",
	"BSD 3-Clause OR MIT":                    "BSD-3-Clause OR MIT",
	"MIT OR BSD 3-Clause":                    "MIT OR BSD-3-Clause",

	// Multiple informal licenses
	"Apache 2 OR GPL v3":                     "Apache-2.0 OR GPL-3.0-or-later",
	"MIT License OR Apache License 2.0":      "MIT OR Apache-2.0",
	"GPL v2 OR LGPL v3":                      "GPL-2.0-only OR LGPL-3.0-or-later",

	// Long informal names
	"GNU General Public License v3 OR MIT":  "GPL-3.0-or-later OR MIT",
	"MIT OR GNU General Public License v2":  "MIT OR GPL-2.0-only",
	"Apache License 2.0 OR BSD 3-Clause":    "Apache-2.0 OR BSD-3-Clause",

	// AND expressions
	"Apache 2 AND MIT":                       "Apache-2.0 AND MIT",
	"GPL v3 AND BSD 3-Clause":                "GPL-3.0-or-later AND BSD-3-Clause",
	"MIT License AND Apache License 2.0":     "MIT AND Apache-2.0",

	// Mixed AND/OR with precedence
	"MIT OR Apache 2 AND GPL v3":             "MIT OR (Apache-2.0 AND GPL-3.0-or-later)",
	"Apache 2 AND MIT OR GPL v3":             "(Apache-2.0 AND MIT) OR GPL-3.0-or-later",

	// Parentheses
	"(Apache 2 OR MIT)":                      "Apache-2.0 OR MIT",
	"(GPL v3 OR MIT) AND BSD":                "(GPL-3.0-or-later OR MIT) AND BSD-2-Clause",
	"MIT AND (Apache 2 OR GPL v3)":           "MIT AND (Apache-2.0 OR GPL-3.0-or-later)",
	"(MIT License) OR (Apache 2)":            "MIT OR Apache-2.0",

	// Plus suffix
	"GPL v2+ OR MIT":                         "GPL-2.0-or-later OR MIT",
	"MIT OR GPLv3+":                          "MIT OR GPL-3.0-or-later",
	"LGPL 2.1+ AND MIT":                      "LGPL-2.1-or-later AND MIT",

	// WITH exceptions (exception names should stay as-is since they're valid)
	"GPL-2.0-only WITH Classpath-exception-2.0 OR MIT": "(GPL-2.0-only WITH Classpath-exception-2.0) OR MIT",

	// Weird spacing
	"  Apache 2   OR   MIT  ":                "Apache-2.0 OR MIT",
	"MIT    OR    Apache 2":                  "MIT OR Apache-2.0",

	// Common typos and variations
	"Apache2 OR MIT":                         "Apache-2.0 OR MIT",
	"GPLv2 OR MIT":                           "GPL-2.0-only OR MIT",
	"LGPL3 OR MIT":                           "LGPL-3.0-or-later OR MIT",
	"BSD OR MIT":                             "BSD-2-Clause OR MIT",

	// URL-like stuff that gets normalized
	"MIT OR Unlicense":                       "MIT OR Unlicense",
	"WTFPL OR MIT":                           "WTFPL OR MIT",

	// Creative Commons
	"CC BY 4.0 OR MIT":                       "CC-BY-4.0 OR MIT",
	"MIT OR CC0":                             "MIT OR CC0-1.0",

	// Mixed valid and informal
	"Apache-2.0 OR GPL v3":                   "Apache-2.0 OR GPL-3.0-or-later",
	"GPL v3 OR Apache-2.0":                   "GPL-3.0-or-later OR Apache-2.0",

	// Edge cases with numbers
	"BSD 2 Clause OR MIT":                    "BSD-2-Clause OR MIT",
	"MIT OR 3 Clause BSD":                    "MIT OR BSD-3-Clause",

	// LicenseRef should pass through
	"LicenseRef-custom OR MIT":               "LicenseRef-custom OR MIT",
	"MIT OR LicenseRef-my-license":           "MIT OR LicenseRef-my-license",

	// Complex nested
	"(Apache 2 OR MIT) AND (GPL v3 OR BSD)":  "(Apache-2.0 OR MIT) AND (GPL-3.0-or-later OR BSD-2-Clause)",
}

func TestParseLax(t *testing.T) {
	for input, expected := range laxParseTests {
		t.Run(input, func(t *testing.T) {
			expr, err := ParseLax(input)
			if err != nil {
				t.Errorf("ParseLax(%q) returned error: %v", input, err)
				return
			}
			result := expr.String()
			if result != expected {
				t.Errorf("ParseLax(%q) = %q, want %q", input, result, expected)
			}
		})
	}
}

// Test that invalid stuff still fails
func TestParseLaxInvalid(t *testing.T) {
	invalidCases := []string{
		"",
		"   ",
		"TOTALLYINVALIDLICENSE",
		"MIT OR NOTAREALLICENSE",
		"AND OR",
		"MIT AND",
		"OR MIT",
		"((MIT)",
	}

	for _, input := range invalidCases {
		t.Run(input, func(t *testing.T) {
			_, err := ParseLax(input)
			if err == nil {
				t.Errorf("ParseLax(%q) should return error", input)
			}
		})
	}
}

// Benchmark lax vs strict parsing
func BenchmarkParseLax(b *testing.B) {
	expressions := []string{
		"MIT",
		"Apache 2 OR MIT",
		"GPL v3 AND BSD 3-Clause OR MIT",
		"(Apache License 2.0 OR MIT) AND (GPL v2 OR BSD)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			ParseLax(expr)
		}
	}
}
