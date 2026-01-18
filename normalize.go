package spdx

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/github/go-spdx/v2/spdxexp/spdxlicenses"
)

var (
	initOnce      sync.Once
	licenseMap    map[string]string // lowercase -> canonical
	exceptionMap  map[string]string // lowercase -> canonical
	deprecatedMap map[string]string // lowercase -> canonical
)

func initMaps() {
	initOnce.Do(func() {
		licenses := spdxlicenses.GetLicenses()
		deprecated := spdxlicenses.GetDeprecated()
		exceptions := spdxlicenses.GetExceptions()

		licenseMap = make(map[string]string, len(licenses)+len(deprecated))
		for _, id := range licenses {
			licenseMap[strings.ToLower(id)] = id
		}

		deprecatedMap = make(map[string]string, len(deprecated))
		for _, id := range deprecated {
			lower := strings.ToLower(id)
			deprecatedMap[lower] = id
			if _, exists := licenseMap[lower]; !exists {
				licenseMap[lower] = id
			}
		}

		exceptionMap = make(map[string]string, len(exceptions))
		for _, id := range exceptions {
			exceptionMap[strings.ToLower(id)] = id
		}
	})
}

// lookupLicense returns the canonical SPDX license ID for the given string,
// or empty string if not found.
func lookupLicense(s string) string {
	initMaps()
	return licenseMap[strings.ToLower(s)]
}

// lookupException returns the canonical SPDX exception ID for the given string,
// or empty string if not found.
func lookupException(s string) string {
	initMaps()
	return exceptionMap[strings.ToLower(s)]
}

// isValidLicenseOrException checks if the string is a valid license or exception.
func isValidLicenseOrException(s string) bool {
	initMaps()
	lower := strings.ToLower(s)
	_, isLicense := licenseMap[lower]
	_, isException := exceptionMap[lower]
	return isLicense || isException
}

// transposition represents a common misspelling or variation to correct.
type transposition struct {
	from      string
	fromUpper string         // pre-computed uppercase
	to        string
	re        *regexp.Regexp // pre-compiled case-insensitive regex
}

// transpositionData is used to initialize transpositions before computing derived fields.
var transpositionData = []struct{ from, to string }{
	// Long phrases first - Apache variations
	{"The Apache Software License, Version 2.0", "Apache-2.0"},
	{"The Apache License, Version 2.0", "Apache-2.0"},
	{"Apache Software License, Version 2.0", "Apache-2.0"},
	{"Apache License, Version 2.0", "Apache-2.0"},
	{"The Apache Software License", "Apache"},
	{"Apache Software License", "Apache"},
	// The MIT License -> MIT
	{"The MIT License", "MIT"},
	// GPL family long forms - versioned first (longer matches)
	{"GNU Lesser General Public License v3.0", "LGPL-3.0"},
	{"GNU Lesser General Public License v3", "LGPL-3.0"},
	{"GNU Lesser General Public License v2.1", "LGPL-2.1"},
	{"GNU Lesser General Public License v2.0", "LGPL-2.0"},
	{"GNU Lesser General Public License v2", "LGPL-2.0"},
	// Note: Generic "Lesser General Public License" without version maps to 2.1 per spdx-correct.js
	{"GNU LESSER GENERAL PUBLIC LICENSE", "LGPL-2.1"},
	{"GNU Lesser General Public License", "LGPL-2.1"},
	{"Lesser General Public License", "LGPL-2.1"},
	{"LESSER GENERAL PUBLIC LICENSE", "LGPL-2.1"},
	{"GNU AFFERO GENERAL PUBLIC LICENSE", "AGPL"},
	{"AFFERO GENERAL PUBLIC LICENSE", "AGPL"},
	{"GNU GENERAL PUBLIC LICENSE", "GPL"},
	{"GNU General Public License", "GPL"},
	{"Gnu public license", "GPL"},
	{"GNU Public License", "GPL"},
	{"Mozilla Public License", "MPL"},
	{"Universal Permissive License", "UPL"},
	// Eclipse
	{"Eclipse Public License", "EPL"},
	// Suffixes and modifiers
	{" or later", "+"},
	{"-or-later", "+"},
	{" International", ""},
	{"GNU LGPL", "LGPL"},
	{"GNU GPL", "GPL"},
	{"GNU/GPL", "GPL"},
	{"GNU GLP", "GPL"},
	{"GNU/GPLv", "GPLv"},
	{" License", ""}, // Strip " License" suffix
	{"-License", ""},
	{"WTFGPL", "WTFPL"},
	{"APGL", "AGPL"},
	{"GLP", "GPL"},
	{"APLv", "Apache-"}, // APLv2 -> Apache-2
	{"APL", "Apache"},
	{"ISD", "ISC"},
	{"IST", "ISC"},
	{"MTI", "MIT"},
	{"GNU", "GPL"},
	{"GUN", "GPL"},
	{"Gpl", "GPL"},
	{"WTH", "WTF"},
	{"Claude", "Clause"}, // common typo
	{"+", ""},            // remove trailing + for matching
}

// transpositions is built from transpositionData with pre-computed fields.
var transpositions []transposition

// Pre-compiled regular expressions for performance.
var (
	reWhitespace    = regexp.MustCompile(`\s+`)
	reDigit         = regexp.MustCompile(`,?\s*(\d)`)
	reDigitEnd      = regexp.MustCompile(`,?\s*(\d)$`)
	reVersion       = regexp.MustCompile(`(?i),?\s*(V\.?|Version)\s*(\d)`)
	reVersionEnd    = regexp.MustCompile(`(?i),?\s*(V\.?|Version)\s*(\d)$`)
	reTrailingDigit = regexp.MustCompile(`(\d)$`)
	reBSDNum        = regexp.MustCompile(`(?i)(-|\s)?(\d)$`)
	reBSDClause     = regexp.MustCompile(`(?i)(-|\s)clause(-|\s)(\d)`)
	reNewBSD        = regexp.MustCompile(`(?i)\b(Modified|New|Revised)(-|\s)?BSD((-|\s)License)?`)
	reSimplifiedBSD = regexp.MustCompile(`(?i)\bSimplified(-|\s)?BSD((-|\s)License)?`)
	reFreeNetBSD    = regexp.MustCompile(`(?i)\b(Free|Net)(-|\s)?BSD((-|\s)Licen[sc]e)?`)
	reClearBSD      = regexp.MustCompile(`(?i)\bClear(-|\s)?BSD((-|\s)License)?`)
	reOldBSD        = regexp.MustCompile(`(?i)\b(Old|Original)(-|\s)?BSD((-|\s)License)?`)
	reCCSpaceDigit  = regexp.MustCompile(`\s+(\d)`)
	reCCVersion     = regexp.MustCompile(`\d\.\d`)
)

// Transform functions that modify license strings.
type transform func(string) string

var transforms = []transform{
	// Uppercase
	func(s string) string { return strings.ToUpper(s) },
	// Trim whitespace
	func(s string) string { return strings.TrimSpace(s) },
	// Remove dots (M.I.T. -> MIT)
	func(s string) string { return strings.ReplaceAll(s, ".", "") },
	// Remove all whitespace (Apache- 2.0 -> Apache-2.0)
	func(s string) string { return reWhitespace.ReplaceAllString(s, "") },
	// Replace spaces with dashes (CC BY 4.0 -> CC-BY-4.0)
	func(s string) string { return reWhitespace.ReplaceAllString(s, "-") },
	// Replace v with dash (LGPLv2.1 -> LGPL-2.1)
	func(s string) string { return strings.Replace(s, "v", "-", 1) },
	// Apache 2.0 -> Apache-2.0
	func(s string) string { return reDigit.ReplaceAllString(s, "-$1") },
	// GPL 2 -> GPL-2.0
	func(s string) string { return reDigitEnd.ReplaceAllString(s, "-$1.0") },
	// Apache Version 2.0 -> Apache-2.0
	func(s string) string { return reVersion.ReplaceAllString(s, "-$2") },
	// Apache Version 2 -> Apache-2.0
	func(s string) string { return reVersionEnd.ReplaceAllString(s, "-$2.0") },
	// Capitalize first letter only (zlib -> Zlib)
	func(s string) string {
		if len(s) == 0 {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
	// Replace / with - (MPL/2.0 -> MPL-2.0)
	func(s string) string { return strings.ReplaceAll(s, "/", "-") },
	// GPL-2.0, GPL-3.0 -> add -only or -or-later
	func(s string) string {
		if strings.Contains(s, "3.0") {
			return s + "-or-later"
		}
		return s + "-only"
	},
	// GPL-2.0- -> GPL-2.0-only
	func(s string) string {
		if strings.HasSuffix(s, "-") {
			return s + "only"
		}
		return s
	},
	// GPL2 -> GPL-2.0
	func(s string) string { return reTrailingDigit.ReplaceAllString(s, "-$1.0") },
	// BSD 3 -> BSD-3-Clause
	func(s string) string { return reBSDNum.ReplaceAllString(s, "-$2-Clause") },
	// BSD clause 3 -> BSD-3-Clause
	func(s string) string { return reBSDClause.ReplaceAllString(s, "-$3-Clause") },
	// New BSD -> BSD-3-Clause
	func(s string) string { return reNewBSD.ReplaceAllString(s, "BSD-3-Clause") },
	// Simplified BSD -> BSD-2-Clause
	func(s string) string { return reSimplifiedBSD.ReplaceAllString(s, "BSD-2-Clause") },
	// Free BSD -> BSD-2-Clause-FreeBSD
	func(s string) string {
		if reFreeNetBSD.MatchString(s) {
			match := reFreeNetBSD.FindStringSubmatch(s)
			if len(match) > 1 {
				variant := strings.ToUpper(match[1][:1]) + strings.ToLower(match[1][1:])
				return "BSD-2-Clause-" + variant + "BSD"
			}
		}
		return s
	},
	// Clear BSD -> BSD-3-Clause-Clear
	func(s string) string { return reClearBSD.ReplaceAllString(s, "BSD-3-Clause-Clear") },
	// Old BSD -> BSD-4-Clause
	func(s string) string { return reOldBSD.ReplaceAllString(s, "BSD-4-Clause") },
	// BY-NC-4.0 -> CC-BY-NC-4.0
	func(s string) string {
		if strings.HasPrefix(strings.ToUpper(s), "BY-") {
			return "CC-" + s
		}
		return s
	},
	// Attribution-NonCommercial -> CC-BY-NC-4.0
	func(s string) string {
		result := s
		result = strings.ReplaceAll(result, "Attribution", "BY")
		result = strings.ReplaceAll(result, "NonCommercial", "NC")
		result = strings.ReplaceAll(result, "NoDerivatives", "ND")
		result = strings.ReplaceAll(result, "ShareAlike", "SA")
		result = reCCSpaceDigit.ReplaceAllString(result, "-$1")
		result = strings.ReplaceAll(result, " International", "")
		if result != s && !strings.HasPrefix(result, "CC-") {
			result = "CC-" + result
			if !reCCVersion.MatchString(result) {
				result = result + "-4.0"
			}
		}
		return result
	},
}

// lastResort maps substrings to their canonical license identifiers.
// Sorted by length (longest first) for correct matching.
type lastResort struct {
	substring string
	license   string
}

var lastResorts = []lastResort{
	{"MIT +NO-FALSE-ATTRIBS", "MITNFA"},
	// Public Domain variants
	{"PUBLIC DOMAIN", "Unlicense"},
	{"PUBLIC-DOMAIN", "Unlicense"},
	{"PUBLICDOMAIN", "Unlicense"},
	// Eclipse with version detection (longer matches first)
	{"ECLIPSE PUBLIC LICENSE 2", "EPL-2.0"},
	{"ECLIPSE PUBLIC LICENSE, VERSION 2", "EPL-2.0"},
	{"ECLIPSE PUBLIC LICENSE V2", "EPL-2.0"},
	{"EPL-2", "EPL-2.0"},
	{"EPL 2", "EPL-2.0"},
	{"EPL2", "EPL-2.0"},
	{"ECLIPSE PUBLIC LICENSE 1", "EPL-1.0"},
	{"EPL-1", "EPL-1.0"},
	{"EPL 1", "EPL-1.0"},
	{"EPL1", "EPL-1.0"},
	// ASL variants (Apache Software License)
	{"ASL-2", "Apache-2.0"},
	{"ASL 2", "Apache-2.0"},
	{"ASL2", "Apache-2.0"},
	{"ALV2", "Apache-2.0"},
	{"AL2", "Apache-2.0"},
	{"ASL", "Apache-2.0"},
	// BSD variants
	{"2 CLAUSE", "BSD-2-Clause"},
	{"2-CLAUSE", "BSD-2-Clause"},
	{"3 CLAUSE", "BSD-3-Clause"},
	{"3-CLAUSE", "BSD-3-Clause"},
	// GPL/LGPL/AGPL
	{"AFFERO", "AGPL-3.0-or-later"},
	{"AGPL", "AGPL-3.0-or-later"},
	{"LGPL2.1+", "LGPL-2.1-or-later"},
	{"LGPL2.1", "LGPL-2.1-only"},
	{"LGPLV2.1", "LGPL-2.1-only"},
	{"LGPLV1", "LGPL-1.0-only"},
	{"LGPL-1", "LGPL-1.0-only"},
	{"LGPLV2", "LGPL-2.0-only"},
	{"LGPL-2", "LGPL-2.0-only"},
	{"LGPL", "LGPL-3.0-or-later"},
	{"GPLV1", "GPL-1.0-only"},
	{"GPL-1", "GPL-1.0-only"},
	{"GPLV2", "GPL-2.0-only"},
	{"GPL-2", "GPL-2.0-only"},
	{"GPL", "GPL-3.0-or-later"},
	{"GNU", "GPL-3.0-or-later"},
	// Common licenses
	{"APACHE", "Apache-2.0"},
	{"ARTISTIC_2", "Artistic-2.0"},
	{"ARTISTIC_1", "Artistic-1.0"},
	{"ARTISTIC-2", "Artistic-2.0"},
	{"ARTISTIC-1", "Artistic-1.0"},
	{"ARTISTIC 2", "Artistic-2.0"},
	{"ARTISTIC 1", "Artistic-1.0"},
	{"ARTISTIC", "Artistic-2.0"},
	{"BEER", "Beerware"},
	{"BOOST", "BSL-1.0"},
	{"BSD", "BSD-2-Clause"},
	{"CC0", "CC0-1.0"},
	{"CDDL", "CDDL-1.1"},
	{"ECLIPSE", "EPL-1.0"},
	{"EPL", "EPL-1.0"},
	{"FUCK", "WTFPL"},
	{"MIT", "MIT"},
	{"MPL", "MPL-2.0"},
	{"UNLI", "Unlicense"},
	{"UPL", "UPL-1.0"},
	{"WTF", "WTFPL"},
	{"X11", "X11"},
	{"ZLIB", "Zlib"},
	// ISC variants
	{"ISCL", "ISC"},
	{"ICS", "ISC"},
	{"ISC", "ISC"},
	// OFL (Open Font License)
	{"OPEN FONT", "OFL-1.1"},
	{"OFL", "OFL-1.1"},
	// PHP License
	{"PHP-3", "PHP-3.01"},
	{"PHP", "PHP-3.01"},
	// Python
	{"PYTHON SOFTWARE FOUNDATION", "PSF-2.0"},
	{"PSF-2", "PSF-2.0"},
	{"PSF", "PSF-2.0"},
	{"PYTHON", "Python-2.0"},
	// Perl
	{"PERL_5", "Artistic-1.0-Perl"},
	{"PERL5", "Artistic-1.0-Perl"},
	{"PERL 5", "Artistic-1.0-Perl"},
	// Zope
	{"ZPL", "ZPL-2.1"},
	// EUPL
	{"EUROPEAN UNION PUBLIC", "EUPL-1.2"},
	{"EUPL", "EUPL-1.2"},
	// wxWindows
	{"WXWINDOWS", "wxWindows"},
	{"WXWIDGETS", "wxWindows"},
}

func init() {
	// Build transpositions from data with pre-computed fields
	transpositions = make([]transposition, len(transpositionData))
	for i, d := range transpositionData {
		transpositions[i] = transposition{
			from:      d.from,
			fromUpper: strings.ToUpper(d.from),
			to:        d.to,
			re:        regexp.MustCompile(`(?i)` + regexp.QuoteMeta(d.from)),
		}
	}

	// Sort transpositions by length (longest first)
	sort.Slice(transpositions, func(i, j int) bool {
		li, lj := len(transpositions[i].from), len(transpositions[j].from)
		if li != lj {
			return li > lj
		}
		return transpositions[i].from < transpositions[j].from
	})

	// Sort lastResorts by length (longest first)
	sort.Slice(lastResorts, func(i, j int) bool {
		li, lj := len(lastResorts[i].substring), len(lastResorts[j].substring)
		if li != lj {
			return li > lj
		}
		return lastResorts[i].substring < lastResorts[j].substring
	})
}

// tryTransforms applies transform functions to try to get a valid license.
func tryTransforms(s string) string {
	// Check if input has trailing +
	hasPlus := strings.HasSuffix(s, "+")
	base := strings.TrimSuffix(s, "+")

	for _, t := range transforms {
		transformed := strings.TrimSpace(t(s))
		if transformed != s && lookupLicense(transformed) != "" {
			return upgradeGPL(lookupLicense(transformed))
		}

		// Also try transform on base (without +) and add + back
		if hasPlus {
			transformedBase := strings.TrimSpace(t(base))
			if transformedBase != base && lookupLicense(transformedBase) != "" {
				return upgradeGPL(lookupLicense(transformedBase) + "+")
			}
		}
	}
	return ""
}

// tryTranspositions applies transpositions and then transforms.
func tryTranspositions(s string) string {
	sUpper := strings.ToUpper(s) // compute once
	for _, trans := range transpositions {
		if strings.Contains(s, trans.from) || strings.Contains(sUpper, trans.fromUpper) {
			corrected := strings.ReplaceAll(s, trans.from, trans.to)
			// Also try case-insensitive replacement using pre-compiled regex
			if corrected == s {
				corrected = trans.re.ReplaceAllString(s, trans.to)
			}

			// Check if directly valid
			if id := lookupLicense(corrected); id != "" {
				return upgradeGPL(id)
			}

			// Try transforms on the corrected string
			if result := tryTransforms(corrected); result != "" {
				return result
			}
		}
	}
	return ""
}

// tryLastResorts uses substring matching as a fallback.
func tryLastResorts(s string) string {
	upper := strings.ToUpper(s)
	for _, lr := range lastResorts {
		if strings.Contains(upper, lr.substring) {
			return upgradeGPL(lr.license)
		}
	}
	return ""
}

// tryTranspositionsWithLastResorts applies transpositions then last resorts.
func tryTranspositionsWithLastResorts(s string) string {
	sUpper := strings.ToUpper(s) // compute once
	for _, trans := range transpositions {
		if strings.Contains(s, trans.from) || strings.Contains(sUpper, trans.fromUpper) {
			corrected := strings.ReplaceAll(s, trans.from, trans.to)
			if corrected == s {
				corrected = trans.re.ReplaceAllString(s, trans.to)
			}

			if result := tryLastResorts(corrected); result != "" {
				return result
			}
		}
	}
	return ""
}

// upgradeGPL converts deprecated GPL/LGPL/AGPL identifiers to their modern equivalents.
func upgradeGPL(license string) string {
	switch license {
	case "GPL-1.0", "LGPL-1.0", "AGPL-1.0",
		"GPL-2.0", "LGPL-2.0", "AGPL-2.0",
		"LGPL-2.1":
		return license + "-only"
	case "GPL-1.0+", "GPL-2.0+", "GPL-3.0+",
		"LGPL-2.0+", "LGPL-2.1+", "LGPL-3.0+",
		"AGPL-1.0+", "AGPL-3.0+":
		return strings.TrimSuffix(license, "+") + "-or-later"
	case "GPL-3.0", "LGPL-3.0", "AGPL-3.0":
		return license + "-or-later"
	default:
		return license
	}
}
