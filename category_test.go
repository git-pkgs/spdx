package spdx

import "testing"

func TestLicenseCategory(t *testing.T) {
	tests := map[string]Category{
		// Permissive
		"MIT":          CategoryPermissive,
		"mit":          CategoryPermissive,
		"Apache-2.0":   CategoryPermissive,
		"BSD-3-Clause": CategoryPermissive,
		"BSD-2-Clause": CategoryPermissive,
		"ISC":          CategoryPermissive,

		// Copyleft
		"GPL-2.0-only":    CategoryCopyleft,
		"GPL-3.0-only":    CategoryCopyleft,
		"GPL-3.0-or-later": CategoryCopyleft,
		"AGPL-3.0-only":   CategoryCopyleft,

		// Copyleft Limited (weak copyleft)
		"LGPL-2.1-only":   CategoryCopyleftLimited,
		"LGPL-3.0-only":   CategoryCopyleftLimited,
		"MPL-2.0":         CategoryCopyleftLimited,
		"EPL-2.0":         CategoryCopyleftLimited,

		// Public Domain
		"Unlicense": CategoryPublicDomain,
		"CC0-1.0":   CategoryPublicDomain,
	}

	for license, expected := range tests {
		t.Run(license, func(t *testing.T) {
			got := LicenseCategory(license)
			if got != expected {
				t.Errorf("LicenseCategory(%q) = %q, want %q", license, got, expected)
			}
		})
	}
}

func TestIsPermissive(t *testing.T) {
	permissive := []string{"MIT", "Apache-2.0", "BSD-3-Clause", "ISC", "Unlicense", "CC0-1.0"}
	for _, lic := range permissive {
		if !IsPermissive(lic) {
			t.Errorf("IsPermissive(%q) = false, want true", lic)
		}
	}

	notPermissive := []string{"GPL-3.0-only", "LGPL-2.1-only", "AGPL-3.0-only"}
	for _, lic := range notPermissive {
		if IsPermissive(lic) {
			t.Errorf("IsPermissive(%q) = true, want false", lic)
		}
	}
}

func TestIsCopyleft(t *testing.T) {
	copyleft := []string{"GPL-2.0-only", "GPL-3.0-only", "LGPL-2.1-only", "LGPL-3.0-only", "AGPL-3.0-only", "MPL-2.0"}
	for _, lic := range copyleft {
		if !IsCopyleft(lic) {
			t.Errorf("IsCopyleft(%q) = false, want true", lic)
		}
	}

	notCopyleft := []string{"MIT", "Apache-2.0", "BSD-3-Clause"}
	for _, lic := range notCopyleft {
		if IsCopyleft(lic) {
			t.Errorf("IsCopyleft(%q) = true, want false", lic)
		}
	}
}

func TestExpressionCategories(t *testing.T) {
	tests := []struct {
		expr       string
		categories []Category
	}{
		{"MIT", []Category{CategoryPermissive}},
		{"MIT OR Apache-2.0", []Category{CategoryPermissive}},
		{"MIT OR GPL-3.0-only", []Category{CategoryPermissive, CategoryCopyleft}},
		{"GPL-2.0-only OR GPL-3.0-only", []Category{CategoryCopyleft}},
		{"MIT AND Apache-2.0 AND BSD-3-Clause", []Category{CategoryPermissive}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			cats, err := ExpressionCategories(tt.expr)
			if err != nil {
				t.Fatalf("ExpressionCategories(%q) error: %v", tt.expr, err)
			}

			if len(cats) != len(tt.categories) {
				t.Errorf("ExpressionCategories(%q) = %v, want %v", tt.expr, cats, tt.categories)
				return
			}

			// Check all expected categories are present
			for _, expected := range tt.categories {
				found := false
				for _, got := range cats {
					if got == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExpressionCategories(%q) missing category %q", tt.expr, expected)
				}
			}
		})
	}
}

func TestGetLicenseInfo(t *testing.T) {
	info := GetLicenseInfo("MIT")
	if info == nil {
		t.Fatal("GetLicenseInfo(\"MIT\") returned nil")
	}
	if info.SPDXKey != "MIT" {
		t.Errorf("GetLicenseInfo(\"MIT\").SPDXKey = %q, want \"MIT\"", info.SPDXKey)
	}
	if info.Category != CategoryPermissive {
		t.Errorf("GetLicenseInfo(\"MIT\").Category = %q, want %q", info.Category, CategoryPermissive)
	}
	if info.IsException {
		t.Error("GetLicenseInfo(\"MIT\").IsException = true, want false")
	}

	// Test exception
	info = GetLicenseInfo("Classpath-exception-2.0")
	if info == nil {
		t.Fatal("GetLicenseInfo(\"Classpath-exception-2.0\") returned nil")
	}
	if !info.IsException {
		t.Error("GetLicenseInfo(\"Classpath-exception-2.0\").IsException = false, want true")
	}
}

func TestHasCopyleft(t *testing.T) {
	tests := map[string]bool{
		"MIT":                      false,
		"MIT OR Apache-2.0":        false,
		"MIT AND BSD-3-Clause":     false,
		"GPL-3.0-only":             true,
		"MIT OR GPL-3.0-only":      true,
		"MIT AND LGPL-2.1-only":    true,
		"Apache-2.0 OR MPL-2.0":    true,  // MPL is weak copyleft
		"Unlicense OR CC0-1.0":     false, // public domain
	}

	for expr, expected := range tests {
		t.Run(expr, func(t *testing.T) {
			got := HasCopyleft(expr)
			if got != expected {
				t.Errorf("HasCopyleft(%q) = %v, want %v", expr, got, expected)
			}
		})
	}
}

func TestIsFullyPermissive(t *testing.T) {
	tests := map[string]bool{
		"MIT":                      true,
		"MIT OR Apache-2.0":        true,
		"MIT AND BSD-3-Clause":     true,
		"Unlicense OR CC0-1.0":     true,  // public domain counts as permissive
		"MIT OR Unlicense":         true,
		"GPL-3.0-only":             false,
		"MIT OR GPL-3.0-only":      false,
		"MIT AND LGPL-2.1-only":    false,
		"Apache-2.0 OR MPL-2.0":    false, // MPL is copyleft limited
	}

	for expr, expected := range tests {
		t.Run(expr, func(t *testing.T) {
			got := IsFullyPermissive(expr)
			if got != expected {
				t.Errorf("IsFullyPermissive(%q) = %v, want %v", expr, got, expected)
			}
		})
	}
}

func TestUnknownLicense(t *testing.T) {
	cat := LicenseCategory("TOTALLY-FAKE-LICENSE-12345")
	if cat != CategoryUnknown {
		t.Errorf("LicenseCategory(unknown) = %q, want %q", cat, CategoryUnknown)
	}
}

func BenchmarkLicenseCategory(b *testing.B) {
	licenses := []string{"MIT", "Apache-2.0", "GPL-3.0-only", "BSD-3-Clause", "MPL-2.0"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, lic := range licenses {
			LicenseCategory(lic)
		}
	}
}
