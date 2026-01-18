package spdx

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed licenses.json
var licensesJSON []byte

// Category represents a license category from scancode-licensedb.
type Category string

const (
	CategoryPermissive      Category = "Permissive"
	CategoryCopyleft        Category = "Copyleft"
	CategoryCopyleftLimited Category = "Copyleft Limited"
	CategoryCommercial      Category = "Commercial"
	CategoryProprietaryFree Category = "Proprietary Free"
	CategoryPublicDomain    Category = "Public Domain"
	CategoryPatentLicense   Category = "Patent License"
	CategorySourceAvailable Category = "Source-available"
	CategoryFreeRestricted  Category = "Free Restricted"
	CategoryCLA             Category = "CLA"
	CategoryUnstated        Category = "Unstated License"
	CategoryUnknown         Category = "Unknown"
)

// licenseEntry represents a license in the scancode database.
type licenseEntry struct {
	LicenseKey          string   `json:"license_key"`
	Category            string   `json:"category"`
	SPDXLicenseKey      string   `json:"spdx_license_key"`
	OtherSPDXKeys       []string `json:"other_spdx_license_keys"`
	IsException         bool     `json:"is_exception"`
	IsDeprecated        bool     `json:"is_deprecated"`
}

var (
	categoryOnce sync.Once
	categoryMap  map[string]Category // lowercase SPDX key -> category
	licenseData  []licenseEntry
)

func initCategoryMap() {
	categoryOnce.Do(func() {
		if err := json.Unmarshal(licensesJSON, &licenseData); err != nil {
			// If JSON is invalid, map will be empty
			categoryMap = make(map[string]Category)
			return
		}

		categoryMap = make(map[string]Category, len(licenseData)*2)
		for _, entry := range licenseData {
			cat := Category(entry.Category)
			if cat == "" {
				cat = CategoryUnknown
			}

			// Map primary SPDX key
			if entry.SPDXLicenseKey != "" {
				categoryMap[strings.ToLower(entry.SPDXLicenseKey)] = cat
			}

			// Map alternative SPDX keys (skip LicenseRef- ones)
			for _, key := range entry.OtherSPDXKeys {
				if !strings.HasPrefix(key, "LicenseRef-") {
					categoryMap[strings.ToLower(key)] = cat
				}
			}

			// Also map the license_key itself
			categoryMap[strings.ToLower(entry.LicenseKey)] = cat
		}
	})
}

// LicenseCategory returns the category for a given license identifier.
// It accepts SPDX identifiers (like "MIT", "Apache-2.0") or scancode keys.
// Returns CategoryUnknown if the license is not found.
//
// Example:
//
//	LicenseCategory("MIT")           // CategoryPermissive
//	LicenseCategory("GPL-3.0-only")  // CategoryCopyleft
//	LicenseCategory("MPL-2.0")       // CategoryCopyleftLimited
func LicenseCategory(license string) Category {
	initCategoryMap()

	// Try exact match first
	if cat, ok := categoryMap[strings.ToLower(license)]; ok {
		return cat
	}

	// Try without -only/-or-later suffixes
	license = strings.TrimSuffix(license, "-only")
	license = strings.TrimSuffix(license, "-or-later")
	if cat, ok := categoryMap[strings.ToLower(license)]; ok {
		return cat
	}

	return CategoryUnknown
}

// ExpressionCategories returns all unique categories for licenses in an expression.
// It parses the expression and returns the category for each license found.
//
// Example:
//
//	ExpressionCategories("MIT OR Apache-2.0")
//	// []Category{CategoryPermissive}  (both are Permissive)
//
//	ExpressionCategories("MIT OR GPL-3.0-only")
//	// []Category{CategoryPermissive, CategoryCopyleft}
func ExpressionCategories(expression string) ([]Category, error) {
	licenses, err := ExtractLicenses(expression)
	if err != nil {
		return nil, err
	}

	seen := make(map[Category]bool)
	var categories []Category

	for _, lic := range licenses {
		cat := LicenseCategory(lic)
		if !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}

	return categories, nil
}

// IsPermissive returns true if the license is in a permissive category.
// This includes Permissive, Public Domain, and similar open categories.
func IsPermissive(license string) bool {
	cat := LicenseCategory(license)
	return cat == CategoryPermissive || cat == CategoryPublicDomain
}

// IsCopyleft returns true if the license has copyleft requirements.
// This includes both full Copyleft and Copyleft Limited (weak copyleft).
func IsCopyleft(license string) bool {
	cat := LicenseCategory(license)
	return cat == CategoryCopyleft || cat == CategoryCopyleftLimited
}

// IsCommercial returns true if the license is commercial/proprietary.
func IsCommercial(license string) bool {
	cat := LicenseCategory(license)
	return cat == CategoryCommercial || cat == CategoryProprietaryFree
}

// HasCopyleft returns true if any license in the expression has copyleft requirements.
// This includes both full Copyleft and Copyleft Limited (weak copyleft like LGPL, MPL).
//
// Example:
//
//	HasCopyleft("MIT OR Apache-2.0")       // false
//	HasCopyleft("MIT OR GPL-3.0-only")     // true
//	HasCopyleft("MIT AND LGPL-2.1-only")   // true
func HasCopyleft(expression string) bool {
	licenses, err := ExtractLicenses(expression)
	if err != nil {
		return false
	}

	for _, lic := range licenses {
		if IsCopyleft(lic) {
			return true
		}
	}
	return false
}

// IsFullyPermissive returns true if all licenses in the expression are permissive.
// This includes Permissive and Public Domain categories.
//
// Example:
//
//	IsFullyPermissive("MIT OR Apache-2.0")     // true
//	IsFullyPermissive("MIT AND BSD-3-Clause") // true
//	IsFullyPermissive("MIT OR GPL-3.0-only")  // false
func IsFullyPermissive(expression string) bool {
	licenses, err := ExtractLicenses(expression)
	if err != nil {
		return false
	}

	for _, lic := range licenses {
		if !IsPermissive(lic) {
			return false
		}
	}
	return len(licenses) > 0
}

// LicenseInfo contains detailed information about a license.
type LicenseInfo struct {
	Key          string   // scancode license key
	SPDXKey      string   // primary SPDX identifier
	Category     Category // license category
	IsException  bool     // true if this is a license exception
	IsDeprecated bool     // true if deprecated
}

// GetLicenseInfo returns detailed information about a license.
// Returns nil if the license is not found.
func GetLicenseInfo(license string) *LicenseInfo {
	initCategoryMap()

	lower := strings.ToLower(license)

	for _, entry := range licenseData {
		// Check SPDX key
		if strings.ToLower(entry.SPDXLicenseKey) == lower {
			return &LicenseInfo{
				Key:          entry.LicenseKey,
				SPDXKey:      entry.SPDXLicenseKey,
				Category:     Category(entry.Category),
				IsException:  entry.IsException,
				IsDeprecated: entry.IsDeprecated,
			}
		}

		// Check license key
		if strings.ToLower(entry.LicenseKey) == lower {
			return &LicenseInfo{
				Key:          entry.LicenseKey,
				SPDXKey:      entry.SPDXLicenseKey,
				Category:     Category(entry.Category),
				IsException:  entry.IsException,
				IsDeprecated: entry.IsDeprecated,
			}
		}
	}

	return nil
}
