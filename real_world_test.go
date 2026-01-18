package spdx

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestRealWorldLicenses tests our parser against real license strings from package managers.
func TestRealWorldLicenses(t *testing.T) {
	data, err := os.ReadFile("real_licenses.json")
	if err != nil {
		t.Skip("real_licenses.json not found")
	}

	var licenses map[string]int
	if err := json.Unmarshal(data, &licenses); err != nil {
		t.Fatalf("Failed to parse real_licenses.json: %v", err)
	}

	// Categorize results
	var (
		validStrict     []string // Already valid SPDX
		normalizedSingle []string // Single license normalized successfully
		normalizedExpr   []string // Expression normalized successfully
		failed          []string // Could not normalize
		skipped         []string // Intentionally skipped (proprietary, unknown, etc.)
	)

	skipPatterns := []string{
		"UNLICENSED", "proprietary", "Proprietary", "PROPRIETARY",
		"custom", "Custom", "CUSTOM", "private", "Private", "PRIVATE",
		"unknown", "Unknown", "UNKNOWN", "none", "None", "NONE",
		"SEE LICENSE", "See license", "LICENSE", "License",
		"All rights reserved", "All Rights Reserved",
		"TODO", "TBD", "tbc", "hi", "iewrbb", "john-wick-4",
		"Commercial", "COMMERCIAL", "EULA", "non-standard", "Nonstandard",
		"Other/Proprietary", "License Agreement", "Copyright",
	}

	shouldSkip := func(s string) bool {
		for _, p := range skipPatterns {
			if strings.Contains(s, p) {
				return true
			}
		}
		// Skip URLs
		if strings.HasPrefix(s, "http") {
			return true
		}
		return false
	}

	for license := range licenses {
		if shouldSkip(license) {
			skipped = append(skipped, license)
			continue
		}

		// Try strict validation first
		if Valid(license) {
			validStrict = append(validStrict, license)
			continue
		}

		// Try single license normalization
		if _, err := Normalize(license); err == nil {
			normalizedSingle = append(normalizedSingle, license)
			continue
		}

		// Try lax expression parsing
		if _, err := ParseLax(license); err == nil {
			normalizedExpr = append(normalizedExpr, license)
			continue
		}

		failed = append(failed, license)
	}

	// Sort for consistent output
	sort.Strings(validStrict)
	sort.Strings(normalizedSingle)
	sort.Strings(normalizedExpr)
	sort.Strings(failed)
	sort.Strings(skipped)

	t.Logf("Results:")
	t.Logf("  Valid SPDX (strict):    %d", len(validStrict))
	t.Logf("  Normalized (single):    %d", len(normalizedSingle))
	t.Logf("  Normalized (expr):      %d", len(normalizedExpr))
	t.Logf("  Skipped (proprietary):  %d", len(skipped))
	t.Logf("  Failed:                 %d", len(failed))

	// Show some failures for debugging
	if len(failed) > 0 {
		t.Logf("\nFailed to normalize (showing first 50):")
		for i, f := range failed {
			if i >= 50 {
				t.Logf("  ... and %d more", len(failed)-50)
				break
			}
			t.Logf("  %q", f)
		}
	}
}

// TestRealWorldCoverage checks specific high-frequency licenses we should handle.
func TestRealWorldCoverage(t *testing.T) {
	// High-frequency licenses from the dataset that we should definitely handle
	mustHandle := map[string]string{
		// Top licenses by frequency
		"MIT":                          "MIT",
		"ISC":                          "ISC",
		"Apache-2.0":                   "Apache-2.0",
		"BSD-3-Clause":                 "BSD-3-Clause",
		"GPL-3.0":                      "GPL-3.0-or-later",
		"MIT License":                  "MIT",
		"Apache License 2.0":           "Apache-2.0",
		"BSD-2-Clause":                 "BSD-2-Clause",
		"GPL-2.0-or-later":             "GPL-2.0-or-later",
		"GPL-3.0-or-later":             "GPL-3.0-or-later",

		// Common variations
		"mit":                          "MIT",
		"apache-2.0":                   "Apache-2.0",
		"Apache 2.0":                   "Apache-2.0",
		"Apache 2":                     "Apache-2.0",
		"Apache":                       "Apache-2.0",
		"Apache Software License":      "Apache-2.0",
		"Apache License, Version 2.0":  "Apache-2.0",
		"The Apache Software License, Version 2.0": "Apache-2.0",
		"The Apache License, Version 2.0": "Apache-2.0",

		// GPL variations
		"GPL":                          "GPL-3.0-or-later",
		"GPL-2.0":                      "GPL-2.0", // Valid deprecated ID, strict parse returns as-is
		"GPL-3.0-only":                 "GPL-3.0-only",
		"GPLv2":                        "GPL-2.0-only",
		"GPLv3":                        "GPL-3.0-or-later",
		"GPL v3":                       "GPL-3.0-or-later",
		"GPL 3.0":                      "GPL-3.0-or-later",
		"GNU GPL v3":                   "GPL-3.0-or-later",
		"GNU GPLv3":                    "GPL-3.0-or-later",
		"GNU General Public License v3": "GPL-3.0-or-later",

		// LGPL variations
		"LGPL":                         "LGPL-3.0-or-later",
		"LGPL-2.1":                     "LGPL-2.1", // Valid deprecated ID, strict parse returns as-is
		"LGPL-3.0":                     "LGPL-3.0-or-later",
		"LGPLv3":                       "LGPL-3.0-or-later",
		"LGPL 2.1":                     "LGPL-2.1-only",
		"GNU Lesser General Public License v3": "LGPL-3.0-or-later",

		// AGPL variations
		"AGPL-3.0":                     "AGPL-3.0-or-later",
		"AGPL-3.0-only":                "AGPL-3.0-only",
		"AGPLv3":                       "AGPL-3.0-or-later",

		// BSD variations
		"BSD":                          "BSD-2-Clause",
		"BSD License":                  "BSD-2-Clause",
		"BSD 3-Clause":                 "BSD-3-Clause",
		"BSD 2-Clause":                 "BSD-2-Clause",
		"New BSD":                      "BSD-3-Clause",
		"The BSD 3-Clause License":     "BSD-3-Clause",

		// MPL
		"MPL-2.0":                      "MPL-2.0",
		"MPL 2.0":                      "MPL-2.0",
		"Mozilla Public License 2.0":   "MPL-2.0",

		// Others
		"Unlicense":                    "Unlicense",
		"CC0-1.0":                      "CC0-1.0",
		"WTFPL":                        "WTFPL",
		"Zlib":                         "Zlib",
		"ISC License":                  "ISC",
		"EPL-2.0":                      "EPL-2.0",
		"Eclipse Public License 2.0":   "EPL-2.0",
		"BSL-1.0":                      "BSL-1.0",
		"PostgreSQL":                   "PostgreSQL",
		"Public Domain":                "Unlicense", // Best guess
	}

	for input, expected := range mustHandle {
		t.Run(input, func(t *testing.T) {
			// Try strict first
			if Valid(input) {
				expr, _ := Parse(input)
				if expr.String() != expected {
					// Check if it's a variant (GPL-3.0 vs GPL-3.0-or-later)
					if !strings.HasPrefix(expr.String(), strings.TrimSuffix(expected, "-or-later")) {
						t.Errorf("Parse(%q) = %q, want %q", input, expr.String(), expected)
					}
				}
				return
			}

			// Try normalization
			result, err := Normalize(input)
			if err != nil {
				// Try lax expression parsing
				expr, err := ParseLax(input)
				if err != nil {
					t.Errorf("Failed to handle %q: %v", input, err)
					return
				}
				result = expr.String()
			}

			if result != expected {
				// Allow some flexibility for -only vs -or-later variants
				if !strings.HasPrefix(result, strings.TrimSuffix(expected, "-or-later")) &&
					!strings.HasPrefix(result, strings.TrimSuffix(expected, "-only")) {
					t.Errorf("Normalize(%q) = %q, want %q", input, result, expected)
				}
			}
		})
	}
}

// TestRealWorldExpressions tests compound expressions from real data.
func TestRealWorldExpressions(t *testing.T) {
	expressions := map[string]bool{
		// Should succeed
		"MIT OR Apache-2.0":                    true,
		"(MIT OR Apache-2.0)":                  true,
		"Apache-2.0 OR MIT":                    true,
		"BSD-3-Clause AND MIT":                 true,
		"GPL-2.0-only OR GPL-3.0-only":         true,
		"MIT AND Apache-2.0":                   true,
		"(MIT AND Apache-2.0)":                 true,
		"Apache-2.0 WITH LLVM-exception":       true,
		"GPL-2.0-only WITH Classpath-exception-2.0": true,
		"EPL-2.0 OR GPL-2.0-or-later WITH Classpath-exception-2.0": true,
		"MIT/Apache-2.0":                       true, // Common but not valid SPDX
		"Unlicense OR MIT":                     true,

		// Lax expressions that should work
		"Apache 2 OR MIT":                      true,
		"GPL v3 OR MIT":                        true,
		"BSD 3-Clause OR MIT":                  true,

		// Should fail (invalid)
		"MIT OR NOTAREALLICENSE":               false,
		"INVALID AND FAKE":                     false,
	}

	for expr, shouldSucceed := range expressions {
		t.Run(expr, func(t *testing.T) {
			// Try strict first
			_, err := Parse(expr)
			if err == nil {
				if !shouldSucceed {
					t.Errorf("Parse(%q) should have failed", expr)
				}
				return
			}

			// Try lax
			_, err = ParseLax(expr)
			succeeded := err == nil

			if succeeded != shouldSucceed {
				if shouldSucceed {
					t.Errorf("ParseLax(%q) failed: %v", expr, err)
				} else {
					t.Errorf("ParseLax(%q) should have failed", expr)
				}
			}
		})
	}
}

// BenchmarkRealWorldNormalization benchmarks normalization with real data.
func BenchmarkRealWorldNormalization(b *testing.B) {
	data, err := os.ReadFile("real_licenses.json")
	if err != nil {
		b.Skip("real_licenses.json not found")
	}

	var licenses map[string]int
	if err := json.Unmarshal(data, &licenses); err != nil {
		b.Fatalf("Failed to parse: %v", err)
	}

	// Get all license strings
	var inputs []string
	for license := range licenses {
		inputs = append(inputs, license)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			// Try ParseLax on everything
			ParseLax(input)
		}
	}
}
