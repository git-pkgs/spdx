package spdx

import (
	"testing"
)

// Test cases from spdx-correct.js
var normalizeTestCases = map[string]string{
	// Simple identifiers (case normalization)
	"MIT":                "MIT",
	"mit":                "MIT",
	"Mit":                "MIT",
	"MiT":                "MIT",
	"Apache-2.0":         "Apache-2.0",
	"apache-2.0":         "Apache-2.0",
	"GPL-3.0-only":       "GPL-3.0-only",
	"gpl-3.0-only":       "GPL-3.0-only",
	"BSD-3-Clause":       "BSD-3-Clause",
	"bsd-3-clause":       "BSD-3-Clause",
	"ISC":                "ISC",
	"isc":                "ISC",
	"Isc":                "ISC",

	// Apache variations
	"Apache 2":                                     "Apache-2.0",
	"Apache 2.0":                                   "Apache-2.0",
	"APACHE 2":                                     "Apache-2.0",
	"APACHE 2.0":                                   "Apache-2.0",
	"APACHE-2":                                     "Apache-2.0",
	"APACHE-2.0":                                   "Apache-2.0",
	"Apache":                                       "Apache-2.0",
	"APACHE":                                       "Apache-2.0",
	"Apache License":                               "Apache-2.0",
	"Apache License 2.0":                           "Apache-2.0",
	"Apache License, Version 2.0":                  "Apache-2.0",
	"Apache License Version 2.0":                   "Apache-2.0",
	"Apache License v2":                            "Apache-2.0",
	"Apache License v2.0":                          "Apache-2.0",
	"Apache License V2":                            "Apache-2.0",
	"Apache License V2.0":                          "Apache-2.0",
	"Apache V2":                                    "Apache-2.0",
	"Apache V2.0":                                  "Apache-2.0",
	"Apache v2":                                    "Apache-2.0",
	"Apache v2.0":                                  "Apache-2.0",
	"Apache2":                                      "Apache-2.0",
	"Apache2.0":                                    "Apache-2.0",
	"Apache-v2":                                    "Apache-2.0",
	"Apache-v2.0":                                  "Apache-2.0",
	"APL 2.0":                                      "Apache-2.0",
	"APL":                                          "Apache-2.0",
	"APL2":                                         "Apache-2.0",
	"Apache Software License 2.0":                  "Apache-2.0",

	// MIT variations
	"MIT License":                                  "MIT",
	"MIT Licence":                                  "MIT",
	"MIT license":                                  "MIT",
	"MIT-License":                                  "MIT",
	"MIT-LICENSE":                                  "MIT",
	"M.I.T":                                        "MIT",
	"M.I.T.":                                       "MIT",
	"MTI":                                          "MIT",

	// GPL variations
	"GPL":                                          "GPL-3.0-or-later",
	"GPL 2":                                        "GPL-2.0-only",
	"GPL 2.0":                                      "GPL-2.0-only",
	"GPL 3":                                        "GPL-3.0-or-later",
	"GPL 3.0":                                      "GPL-3.0-or-later",
	"GPL v2":                                       "GPL-2.0-only",
	"GPL v3":                                       "GPL-3.0-or-later",
	"GPL V2":                                       "GPL-2.0-only",
	"GPL V3":                                       "GPL-3.0-or-later",
	"GPL-2":                                        "GPL-2.0-only",
	"GPL-3":                                        "GPL-3.0-or-later",
	"GPL2":                                         "GPL-2.0-only",
	"GPL3":                                         "GPL-3.0-or-later",
	"GPLv2":                                        "GPL-2.0-only",
	"GPLv3":                                        "GPL-3.0-or-later",
	"GPLV2":                                        "GPL-2.0-only",
	"GPLV3":                                        "GPL-3.0-or-later",
	"Gpl":                                          "GPL-3.0-or-later",
	"GNU GPL":                                      "GPL-3.0-or-later",
	"GNU GPL v2":                                   "GPL-2.0-only",
	"GNU GPL v3":                                   "GPL-3.0-or-later",
	"GNU GPLv2":                                    "GPL-2.0-only",
	"GNU GPLv3":                                    "GPL-3.0-or-later",
	"GNU GENERAL PUBLIC LICENSE":                   "GPL-3.0-or-later",
	"GNU General Public License":                   "GPL-3.0-or-later",
	"GNU General Public License v2.0":              "GPL-2.0-only",
	"GNU General Public License v3":                "GPL-3.0-or-later",
	"GNU":                                          "GPL-3.0-or-later",

	// LGPL variations
	"LGPL":                                         "LGPL-3.0-or-later",
	"LGPL 2.1":                                     "LGPL-2.1-only",
	"LGPL 3":                                       "LGPL-3.0-or-later",
	"LGPL 3.0":                                     "LGPL-3.0-or-later",
	"LGPL v2":                                      "LGPL-2.0-only",
	"LGPL v3":                                      "LGPL-3.0-or-later",
	"LGPL-2":                                       "LGPL-2.0-only",
	"LGPL-3":                                       "LGPL-3.0-or-later",
	"LGPL2":                                        "LGPL-2.0-only",
	"LGPL3":                                        "LGPL-3.0-or-later",
	"LGPLv2.1":                                     "LGPL-2.1-only",
	"LGPLv3":                                       "LGPL-3.0-or-later",
	"GNU LGPL":                                     "LGPL-3.0-or-later",
	"GNU Lesser General Public License v2.1":       "LGPL-2.1-only",
	"GNU Lesser General Public License v3":         "LGPL-3.0-or-later",

	// AGPL variations
	"AGPL":                                         "AGPL-3.0-or-later",
	"AGPL 3":                                       "AGPL-3.0-or-later",
	"AGPL 3.0":                                     "AGPL-3.0-or-later",
	"AGPL v3":                                      "AGPL-3.0-or-later",
	"AGPL-3":                                       "AGPL-3.0-or-later",
	"AGPL3":                                        "AGPL-3.0-or-later",
	"AGPLv3":                                       "AGPL-3.0-or-later",
	"GNU Affero GPL v3":                            "AGPL-3.0-or-later",
	"Affero GPL v3":                                "AGPL-3.0-or-later",

	// BSD variations
	"BSD":                                          "BSD-2-Clause",
	"BSD 2-Clause":                                 "BSD-2-Clause",
	"BSD 3-Clause":                                 "BSD-3-Clause",
	"BSD 3":                                        "BSD-3-Clause",
	"BSD-3":                                        "BSD-3-Clause",
	"BSD3":                                         "BSD-3-Clause",
	"2-clause-BSD":                                 "BSD-2-Clause",
	"3-Clause BSD":                                 "BSD-3-Clause",
	"3-Clause-BSD":                                 "BSD-3-Clause",
	"2 clause BSD":                                 "BSD-2-Clause",
	"BSD clause 3":                                 "BSD-3-Clause",
	"New BSD":                                      "BSD-3-Clause",
	"Modified BSD":                                 "BSD-3-Clause",
	"Simplified BSD":                               "BSD-2-Clause",
	"BSD 4-Clause":                                 "BSD-4-Clause",
	"BSD-4-Clause":                                 "BSD-4-Clause",
	"Old BSD":                                      "BSD-4-Clause",
	"Clear BSD License":                            "BSD-3-Clause-Clear",

	// MPL variations
	"MPL":                                          "MPL-2.0",
	"MPL 2":                                        "MPL-2.0",
	"MPL 2.0":                                      "MPL-2.0",
	"MPL-2":                                        "MPL-2.0",
	"MPL2":                                         "MPL-2.0",
	"MPLv2":                                        "MPL-2.0",
	"Mozilla Public License":                       "MPL-2.0",
	"Mozilla Public License 2.0":                   "MPL-2.0",
	"Mozilla Public License, v. 2.0":               "MPL-2.0",

	// ISC variations
	"ISD":                                          "ISC",
	"IST":                                          "ISC",

	// CC variations
	"CC0":                                          "CC0-1.0",
	"CC BY 3.0":                                    "CC-BY-3.0",
	"CC BY 4.0":                                    "CC-BY-4.0",
	"CC-BY 3.0":                                    "CC-BY-3.0",
	"CC-BY 4.0 International":                      "CC-BY-4.0",
	"Attribution-NonCommercial":                    "CC-BY-NC-4.0",

	// Unlicense variations
	"UNLICENSE":                                    "Unlicense",
	"Unlicense":                                    "Unlicense",
	"Unlicensed":                                   "Unlicense",
	"Public Domain (Unlicense)":                    "Unlicense",
	"The Unlicense":                                "Unlicense",

	// WTFPL variations
	"WTFPL":                                        "WTFPL",
	"WTF":                                          "WTFPL",
	"DWTFYW":                                       "WTFPL",

	// Other licenses
	"Beerware":                                     "Beerware",
	"BEER":                                         "Beerware",
	"Boost":                                        "BSL-1.0",
	"BOOST":                                        "BSL-1.0",
	"Eclipse":                                      "EPL-1.0",
	"Eclipse Public License":                       "EPL-1.0",
	"Eclipse Public License 1.0":                   "EPL-1.0",
	"Artistic":                                     "Artistic-2.0",
	"Artistic License":                             "Artistic-2.0",
	"Artistic 2.0":                                 "Artistic-2.0",
	"Zlib":                                         "Zlib",
	"ZLIB":                                         "Zlib",
	"CDDL":                                         "CDDL-1.1",
	"UPL":                                          "UPL-1.0",

	// With trailing/leading whitespace
	" MIT ":                                        "MIT",
	"MIT ":                                         "MIT",
	" MIT":                                         "MIT",

	// Plus variations (or-later)
	"GPL-2.0+":                                     "GPL-2.0-or-later",
	"GPL-3.0+":                                     "GPL-3.0-or-later",
	"LGPL-2.1+":                                    "LGPL-2.1-or-later",
	"LGPL-3.0+":                                    "LGPL-3.0-or-later",
	"AGPL-3.0+":                                    "AGPL-3.0-or-later",
	"GPLv2+":                                       "GPL-2.0-or-later",
	"GPLv3+":                                       "GPL-3.0-or-later",
	"GPL2+":                                        "GPL-2.0-or-later",

	// URLs (should extract the license)
	"Http://opensource.org/licenses/MIT":           "MIT",
	"Http://www.apache.org/licenses/LICENSE-2.0":   "Apache-2.0",
}

func TestNormalize(t *testing.T) {
	for input, expected := range normalizeTestCases {
		t.Run(input, func(t *testing.T) {
			result, err := Normalize(input)
			if err != nil {
				t.Errorf("Normalize(%q) returned error: %v", input, err)
				return
			}
			if result != expected {
				t.Errorf("Normalize(%q) = %q, want %q", input, result, expected)
			}
		})
	}
}

func TestNormalizeInvalid(t *testing.T) {
	invalidCases := []string{
		"",
		"   ",
		"UNKNOWN-LICENSE",
		"FAKEYLICENSE",
		"NOT-A-LICENSE",
	}

	for _, input := range invalidCases {
		t.Run(input, func(t *testing.T) {
			_, err := Normalize(input)
			if err == nil {
				t.Errorf("Normalize(%q) should return error", input)
			}
		})
	}
}

func TestValid(t *testing.T) {
	validCases := []string{
		"MIT",
		"mit",
		"Apache-2.0",
		"GPL-3.0-only",
		"MIT OR Apache-2.0",
		"MIT AND Apache-2.0",
		"MIT OR GPL-2.0-only AND Apache-2.0",
		"(MIT OR Apache-2.0)",
		"((MIT OR Apache-2.0))",
		"MIT OR (GPL-2.0-only AND Apache-2.0)",
		"(MIT OR GPL-2.0-only) AND Apache-2.0",
		"AGPL-3.0+",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"LicenseRef-custom",
		"DocumentRef-doc:LicenseRef-custom",
		"NONE",
		"NOASSERTION",
	}

	for _, expr := range validCases {
		t.Run(expr, func(t *testing.T) {
			if !Valid(expr) {
				t.Errorf("Valid(%q) = false, want true", expr)
			}
		})
	}
}

func TestInvalid(t *testing.T) {
	invalidCases := []string{
		"",
		"AND AND",
		" AND ",
		" WITH ",
		"MIT AND ",
		"MIT OR FAKEYLICENSE",
		"MIT (MIT)",
		"MIT OR MIT AND OR",
		"((MIT)",
		"(MIT))",
	}

	for _, expr := range invalidCases {
		t.Run(expr, func(t *testing.T) {
			if Valid(expr) {
				t.Errorf("Valid(%q) = true, want false", expr)
			}
		})
	}
}

// Test cases from Ruby spdx library for normalization
func TestNormalizeExpression(t *testing.T) {
	testCases := map[string]string{
		// Simple licenses
		"MIT":                              "MIT",
		"mit":                              "MIT",
		"MiT":                              "MIT",
		"(MiT)":                            "MIT",
		"(((MiT)))":                        "MIT",
		"Apache-2.0+":                      "Apache-2.0+",
		"apache-2.0+":                      "Apache-2.0+",

		// Boolean expressions
		"mit AND gPL-2.0-only":             "MIT AND GPL-2.0-only",
		"mit OR gPL-2.0-only":              "MIT OR GPL-2.0-only",

		// Semantic grouping (AND binds tighter than OR)
		"mit OR gPL-2.0-only AND apAcHe-2.0+": "MIT OR (GPL-2.0-only AND Apache-2.0+)",

		// Preserves original groups
		"(mit OR gPL-2.0-only) AND apAcHe-2.0+": "(MIT OR GPL-2.0-only) AND Apache-2.0+",

		// WITH expressions
		"GPL-2.0-only WITH Classpath-exception-2.0": "GPL-2.0-only WITH Classpath-exception-2.0",
		"Gpl-2.0-ONLY WITH ClassPath-exception-2.0": "GPL-2.0-only WITH Classpath-exception-2.0",
		"epl-2.0 OR gpl-2.0-only WITH classpath-exception-2.0": "EPL-2.0 OR (GPL-2.0-only WITH Classpath-exception-2.0)",

		// License refs (preserved as-is)
		"LicenseRef-MIT-style-1": "LicenseRef-MIT-style-1",
		"DocumentRef-something-1:LicenseRef-MIT-style-1": "DocumentRef-something-1:LicenseRef-MIT-style-1",
	}

	for input, expected := range testCases {
		t.Run(input, func(t *testing.T) {
			result, err := NormalizeExpression(input)
			if err != nil {
				t.Errorf("NormalizeExpression(%q) returned error: %v", input, err)
				return
			}
			if result != expected {
				t.Errorf("NormalizeExpression(%q) = %q, want %q", input, result, expected)
			}
		})
	}
}

func TestParseLicenses(t *testing.T) {
	testCases := map[string][]string{
		"MIT":                              {"MIT"},
		"MIT OR Apache-2.0":                {"MIT", "Apache-2.0"},
		"MIT AND Apache-2.0":               {"MIT", "Apache-2.0"},
		"MIT OR Apache-2.0 AND GPL-2.0-only": {"MIT", "Apache-2.0", "GPL-2.0-only"},
		"GPL-2.0-only WITH Classpath-exception-2.0": {"GPL-2.0-only"},
		"LicenseRef-custom":                {"LicenseRef-custom"},
	}

	for input, expected := range testCases {
		t.Run(input, func(t *testing.T) {
			expr, err := Parse(input)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", input, err)
				return
			}
			licenses := expr.Licenses()
			if len(licenses) != len(expected) {
				t.Errorf("Parse(%q).Licenses() = %v, want %v", input, licenses, expected)
				return
			}
			for i, lic := range licenses {
				if lic != expected[i] {
					t.Errorf("Parse(%q).Licenses()[%d] = %q, want %q", input, i, lic, expected[i])
				}
			}
		})
	}
}

func TestSpecialValues(t *testing.T) {
	// NONE and NOASSERTION are valid standalone
	for _, val := range []string{"NONE", "NOASSERTION"} {
		t.Run(val, func(t *testing.T) {
			expr, err := Parse(val)
			if err != nil {
				t.Errorf("Parse(%q) returned error: %v", val, err)
				return
			}
			special, ok := expr.(*SpecialValue)
			if !ok {
				t.Errorf("Parse(%q) did not return SpecialValue", val)
				return
			}
			if special.Value != val {
				t.Errorf("Parse(%q).Value = %q, want %q", val, special.Value, val)
			}
			if len(special.Licenses()) != 0 {
				t.Errorf("Parse(%q).Licenses() should be empty", val)
			}
		})
	}
}

func TestValidateLicenses(t *testing.T) {
	valid, invalid := ValidateLicenses([]string{"MIT", "Apache-2.0", "GPL-3.0-only"})
	if !valid {
		t.Errorf("ValidateLicenses with valid licenses returned false")
	}
	if len(invalid) != 0 {
		t.Errorf("ValidateLicenses with valid licenses returned invalid: %v", invalid)
	}

	valid, invalid = ValidateLicenses([]string{"MIT", "FAKEYLICENSE", "Apache-2.0"})
	if valid {
		t.Errorf("ValidateLicenses with invalid license returned true")
	}
	if len(invalid) != 1 || invalid[0] != "FAKEYLICENSE" {
		t.Errorf("ValidateLicenses returned wrong invalid: %v", invalid)
	}
}

func TestExtractLicenses(t *testing.T) {
	licenses, err := ExtractLicenses("MIT OR Apache-2.0 AND GPL-2.0-only")
	if err != nil {
		t.Errorf("ExtractLicenses returned error: %v", err)
		return
	}
	expected := []string{"Apache-2.0", "GPL-2.0-only", "MIT"} // sorted/deduped
	if len(licenses) != len(expected) {
		t.Errorf("ExtractLicenses returned %v, want %v", licenses, expected)
	}
}

// Benchmark normalization performance
func BenchmarkNormalize(b *testing.B) {
	inputs := []string{
		"MIT",
		"Apache 2.0",
		"GPL v3",
		"GNU General Public License v3",
		"BSD 3-Clause",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			Normalize(input)
		}
	}
}

func BenchmarkNormalizeBatch(b *testing.B) {
	// Simulate processing many licenses
	inputs := make([]string, 1000)
	variations := []string{
		"MIT", "Apache 2.0", "GPL v3", "BSD", "ISC", "Unlicense",
		"Apache License 2.0", "GNU GPL v2", "LGPL 3.0", "MPL 2.0",
	}
	for i := range inputs {
		inputs[i] = variations[i%len(variations)]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			Normalize(input)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	expressions := []string{
		"MIT",
		"MIT OR Apache-2.0",
		"MIT AND Apache-2.0 OR GPL-3.0-only",
		"(MIT OR Apache-2.0) AND (GPL-2.0-only OR BSD-3-Clause)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			Parse(expr)
		}
	}
}

func BenchmarkValid(b *testing.B) {
	expressions := []string{
		"MIT",
		"MIT OR Apache-2.0",
		"MIT AND Apache-2.0 OR GPL-3.0-only",
		"invalid-license",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			Valid(expr)
		}
	}
}

// TestParseNormalizesInformalLicenses tests that Parse handles informal license names.
func TestParseNormalizesInformalLicenses(t *testing.T) {
	tests := map[string]string{
		// Single informal licenses
		"Apache 2":        "Apache-2.0",
		"MIT License":     "MIT",
		"GPL v3":          "GPL-3.0-or-later",
		"BSD 3-Clause":    "BSD-3-Clause",

		// Expressions with informal licenses
		"Apache 2 OR MIT":              "Apache-2.0 OR MIT",
		"mit OR apache 2":              "MIT OR Apache-2.0",
		"GPL v3 AND BSD":               "GPL-3.0-or-later AND BSD-2-Clause",
		"Apache 2 OR MIT License":      "Apache-2.0 OR MIT",
		"(Apache 2 OR MIT) AND GPL v3": "(Apache-2.0 OR MIT) AND GPL-3.0-or-later",

		// Mixed strict and informal
		"MIT OR Apache 2":       "MIT OR Apache-2.0",
		"Apache-2.0 OR GPL v3":  "Apache-2.0 OR GPL-3.0-or-later",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			expr, err := Parse(input)
			if err != nil {
				t.Errorf("Parse(%q) failed: %v", input, err)
				return
			}
			if expr.String() != expected {
				t.Errorf("Parse(%q) = %q, want %q", input, expr.String(), expected)
			}
		})
	}
}

// TestParseStrictRejectsInformalLicenses tests that ParseStrict rejects informal license names.
func TestParseStrictRejectsInformalLicenses(t *testing.T) {
	// These should fail with ParseStrict
	informal := []string{
		"Apache 2",
		"MIT License",
		"GPL v3",
		"Apache 2 OR MIT",
	}

	for _, input := range informal {
		t.Run(input, func(t *testing.T) {
			_, err := ParseStrict(input)
			if err == nil {
				t.Errorf("ParseStrict(%q) should have failed but succeeded", input)
			}
		})
	}

	// These should succeed with ParseStrict
	strict := []string{
		"MIT",
		"Apache-2.0",
		"GPL-3.0-only",
		"MIT OR Apache-2.0",
	}

	for _, input := range strict {
		t.Run(input, func(t *testing.T) {
			_, err := ParseStrict(input)
			if err != nil {
				t.Errorf("ParseStrict(%q) failed: %v", input, err)
			}
		})
	}
}
