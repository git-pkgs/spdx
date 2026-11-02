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
	CategoryNonCommercial   Category = "Non-Commercial"
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
	LicenseKey     string   `json:"license_key"`
	Category       string   `json:"category"`
	SPDXLicenseKey string   `json:"spdx_license_key"`
	OtherSPDXKeys  []string `json:"other_spdx_license_keys"`
	IsException    bool     `json:"is_exception"`
	IsDeprecated   bool     `json:"is_deprecated"`
}

var (
	categoryOnce sync.Once
	licenseIndex map[string]*licenseEntry // lowercase key -> entry
	licenseData  []licenseEntry
)

func initCategoryMap() {
	categoryOnce.Do(func() {
		if err := json.Unmarshal(licensesJSON, &licenseData); err != nil {
			// If JSON is invalid, map will be empty
			licenseIndex = make(map[string]*licenseEntry)
			return
		}

		// Keys are added in priority order so that an SPDX identifier is
		// never shadowed by another entry's alternate or ScanCode key, such
		// as BSL-1.0 (Boost) and ScanCode's bsl-1.0 (Business Source).
		estimatedKeys := 3
		licenseIndex = make(map[string]*licenseEntry, len(licenseData)*estimatedKeys)
		add := func(key string, entry *licenseEntry) {
			key = strings.ToLower(key)
			if _, exists := licenseIndex[key]; key != "" && !exists {
				licenseIndex[key] = entry
			}
		}
		for i := range licenseData {
			add(licenseData[i].SPDXLicenseKey, &licenseData[i])
		}
		for i := range licenseData {
			for _, key := range licenseData[i].OtherSPDXKeys {
				add(key, &licenseData[i])
			}
		}
		for i := range licenseData {
			add(licenseData[i].LicenseKey, &licenseData[i])
		}
	})
}

func lookupLicenseEntry(license string) *licenseEntry {
	initCategoryMap()
	return licenseIndex[strings.ToLower(license)]
}

func (e *licenseEntry) category() Category {
	if e.Category == "" {
		return CategoryUnknown
	}
	return Category(e.Category)
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
	if entry := lookupLicenseEntry(license); entry != nil {
		return entry.category()
	}

	// Try without -only/-or-later suffixes
	license = strings.TrimSuffix(license, "-only")
	license = strings.TrimSuffix(license, "-or-later")
	if entry := lookupLicenseEntry(license); entry != nil {
		return entry.category()
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
	licenses, err := licenseIDs(expression)
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
	licenses, err := licenseIDs(expression)
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
	licenses, err := licenseIDs(expression)
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
	entry := lookupLicenseEntry(license)
	if entry == nil {
		return nil
	}
	return &LicenseInfo{
		Key:          entry.LicenseKey,
		SPDXKey:      entry.SPDXLicenseKey,
		Category:     Category(entry.Category),
		IsException:  entry.IsException,
		IsDeprecated: entry.IsDeprecated,
	}
}
