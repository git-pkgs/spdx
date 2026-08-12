package spdx

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/github/go-spdx/v2/spdxexp/spdxlicenses"
)

var (
	licenseMap            map[string]string // lowercase -> canonical
	exceptionMap          map[string]string // lowercase -> canonical
	canonicalLicenseMap   map[string]string // canonical -> canonical
	canonicalExceptionMap map[string]string // canonical -> canonical
	licenseFoldMap        map[uint64][]string
	exceptionFoldMap      map[uint64][]string
)

func initMaps() {
	licenses := spdxlicenses.GetLicenses()
	deprecated := spdxlicenses.GetDeprecated()
	exceptions := spdxlicenses.GetExceptions()

	licenseMap = make(map[string]string, len(licenses)+len(deprecated))
	canonicalLicenseMap = make(map[string]string, len(licenses)+len(deprecated))
	for _, id := range licenses {
		licenseMap[strings.ToLower(id)] = id
		canonicalLicenseMap[id] = id
	}
	for _, id := range deprecated {
		lower := strings.ToLower(id)
		if _, exists := licenseMap[lower]; !exists {
			licenseMap[lower] = id
		}
		if _, exists := canonicalLicenseMap[id]; !exists {
			canonicalLicenseMap[id] = id
		}
	}

	exceptionMap = make(map[string]string, len(exceptions))
	canonicalExceptionMap = make(map[string]string, len(exceptions))
	for _, id := range exceptions {
		exceptionMap[strings.ToLower(id)] = id
		canonicalExceptionMap[id] = id
	}

	licenseFoldMap = makeFoldMap(licenseMap)
	exceptionFoldMap = makeFoldMap(exceptionMap)
}

// lookupLicense returns the canonical SPDX license ID for the given string,
// or empty string if not found.
func lookupLicense(s string) string {
	if id := canonicalLicenseMap[s]; id != "" {
		return id
	}
	return lookupFolded(s, licenseMap, licenseFoldMap)
}

// lookupException returns the canonical SPDX exception ID for the given string,
// or empty string if not found.
func lookupException(s string) string {
	if id := canonicalExceptionMap[s]; id != "" {
		return id
	}
	return lookupFolded(s, exceptionMap, exceptionFoldMap)
}

// isValidLicenseOrException checks if the string is a valid license or exception.
func isValidLicenseOrException(s string) bool {
	return lookupLicense(s) != "" || lookupException(s) != ""
}

func makeFoldMap(source map[string]string) map[uint64][]string {
	result := make(map[uint64][]string, len(source))
	for _, canonical := range source {
		hash := foldHash(canonical)
		result[hash] = append(result[hash], canonical)
	}
	return result
}

func lookupFolded(s string, lowerMap map[string]string, foldMap map[uint64][]string) string {
	if !isASCII(s) {
		return lowerMap[strings.ToLower(s)]
	}
	for _, canonical := range foldMap[foldHash(s)] {
		if strings.EqualFold(s, canonical) {
			return canonical
		}
	}
	return ""
}

func lookupLicenseWithSuffix(s, suffix string) string {
	if !isASCII(s) {
		return lookupLicense(s + suffix)
	}
	for _, canonical := range licenseFoldMap[foldHashParts(s, suffix)] {
		if len(canonical) == len(s)+len(suffix) &&
			strings.EqualFold(s, canonical[:len(s)]) &&
			strings.EqualFold(suffix, canonical[len(s):]) {
			return canonical
		}
	}
	return ""
}

func foldHash(s string) uint64 {
	return foldHashParts(s)
}

func foldHashParts(parts ...string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	hash := uint64(offset64)
	for _, part := range parts {
		for i := 0; i < len(part); i++ {
			character := part[i]
			if character >= 'A' && character <= 'Z' {
				character += 'a' - 'A'
			}
			hash ^= uint64(character)
			hash *= prime64
		}
	}
	return hash
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// transposition represents a common misspelling or variation to correct.
type transposition struct {
	from      string
	fromUpper string // pre-computed uppercase
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
	// Preserve Unicode case-conversion behavior without allocating for SPDX's ASCII inputs.
	func(s string) string {
		if isASCII(s) {
			return s
		}
		return strings.ToUpper(s)
	},
	// Remove dots (M.I.T. -> MIT)
	func(s string) string {
		if !strings.Contains(s, ".") {
			return s
		}
		return strings.ReplaceAll(s, ".", "")
	},
	// Remove all whitespace (Apache- 2.0 -> Apache-2.0)
	func(s string) string {
		if !hasWhitespace(s) {
			return s
		}
		return replaceWhitespace(s, "")
	},
	// Replace spaces with dashes (CC BY 4.0 -> CC-BY-4.0)
	func(s string) string {
		if !hasWhitespace(s) {
			return s
		}
		return replaceWhitespace(s, "-")
	},
	// Replace v with dash (LGPLv2.1 -> LGPL-2.1)
	func(s string) string {
		if !strings.Contains(s, "v") {
			return s
		}
		return strings.Replace(s, "v", "-", 1)
	},
	// Apache 2.0 -> Apache-2.0
	func(s string) string {
		if !hasDigit(s) {
			return s
		}
		return replaceSeparatedDigits(s, false)
	},
	// GPL 2 -> GPL-2.0
	func(s string) string {
		if !hasDigit(s) {
			return s
		}
		return replaceSeparatedDigits(s, true)
	},
	// Apache Version 2.0 -> Apache-2.0
	func(s string) string {
		if !hasVersionWord(s) {
			return s
		}
		return replaceVersion(s, false)
	},
	// Apache Version 2 -> Apache-2.0
	func(s string) string {
		if !hasVersionWord(s) {
			return s
		}
		return replaceVersion(s, true)
	},
	// Replace / with - (MPL/2.0 -> MPL-2.0)
	func(s string) string {
		if !strings.Contains(s, "/") {
			return s
		}
		return strings.ReplaceAll(s, "/", "-")
	},
	// GPL-2.0, GPL-3.0 -> add -only or -or-later
	func(s string) string {
		suffix := "-only"
		if strings.Contains(s, "3.0") {
			suffix = "-or-later"
		}
		if canonical := lookupLicenseWithSuffix(s, suffix); canonical != "" {
			return canonical
		}
		return s
	},
	// GPL-2.0- -> GPL-2.0-only
	func(s string) string {
		if strings.HasSuffix(s, "-") {
			return s + "only"
		}
		return s
	},
	// GPL2 -> GPL-2.0
	func(s string) string {
		if len(s) == 0 || s[len(s)-1] < '0' || s[len(s)-1] > '9' {
			return s
		}
		return s[:len(s)-1] + "-" + s[len(s)-1:] + ".0"
	},
	// BSD 3 -> BSD-3-Clause
	func(s string) string {
		if !containsFold(s, "BSD") || len(s) == 0 || s[len(s)-1] < '0' || s[len(s)-1] > '9' {
			return s
		}
		return reBSDNum.ReplaceAllString(s, "-$2-Clause")
	},
	// BSD clause 3 -> BSD-3-Clause
	func(s string) string {
		if !containsFold(s, "BSD") || !containsFold(s, "clause") {
			return s
		}
		return reBSDClause.ReplaceAllString(s, "-$3-Clause")
	},
	// New BSD -> BSD-3-Clause
	func(s string) string {
		if !containsFold(s, "BSD") ||
			(!containsFold(s, "Modified") && !containsFold(s, "New") && !containsFold(s, "Revised")) {
			return s
		}
		return reNewBSD.ReplaceAllString(s, "BSD-3-Clause")
	},
	// Simplified BSD -> BSD-2-Clause
	func(s string) string {
		if !containsFold(s, "BSD") || !containsFold(s, "Simplified") {
			return s
		}
		return reSimplifiedBSD.ReplaceAllString(s, "BSD-2-Clause")
	},
	// Free BSD -> BSD-2-Clause-FreeBSD
	func(s string) string {
		if containsFold(s, "BSD") && (containsFold(s, "Free") || containsFold(s, "Net")) &&
			reFreeNetBSD.MatchString(s) {
			match := reFreeNetBSD.FindStringSubmatch(s)
			if len(match) > 1 {
				variant := strings.ToUpper(match[1][:1]) + strings.ToLower(match[1][1:])
				return "BSD-2-Clause-" + variant + "BSD"
			}
		}
		return s
	},
	// Clear BSD -> BSD-3-Clause-Clear
	func(s string) string {
		if !containsFold(s, "BSD") || !containsFold(s, "Clear") {
			return s
		}
		return reClearBSD.ReplaceAllString(s, "BSD-3-Clause-Clear")
	},
	// Old BSD -> BSD-4-Clause
	func(s string) string {
		if !containsFold(s, "BSD") ||
			(!containsFold(s, "Old") && !containsFold(s, "Original")) {
			return s
		}
		return reOldBSD.ReplaceAllString(s, "BSD-4-Clause")
	},
	// BY-NC-4.0 -> CC-BY-NC-4.0
	func(s string) string {
		if hasPrefixFold(s, "BY-") {
			return "CC-" + s
		}
		return s
	},
	// Attribution-NonCommercial -> CC-BY-NC-4.0
	func(s string) string {
		if !strings.Contains(s, "Attribution") &&
			!strings.Contains(s, "NonCommercial") &&
			!strings.Contains(s, "NoDerivatives") &&
			!strings.Contains(s, "ShareAlike") {
			return s
		}
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
				result += "-4.0"
			}
		}
		return result
	},
}

func hasWhitespace(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			return true
		}
	}
	return false
}

func replaceWhitespace(s, replacement string) string {
	var result strings.Builder
	result.Grow(len(s))
	start := 0
	for i := 0; i < len(s); {
		if !isRegexpSpace(s[i]) {
			i++
			continue
		}
		result.WriteString(s[start:i])
		result.WriteString(replacement)
		for i < len(s) && isRegexpSpace(s[i]) {
			i++
		}
		start = i
	}
	result.WriteString(s[start:])
	return result.String()
}

func isRegexpSpace(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

func replaceSeparatedDigits(s string, atEnd bool) string {
	if atEnd {
		if len(s) == 0 || s[len(s)-1] < '0' || s[len(s)-1] > '9' {
			return s
		}
		start := len(s) - 1
		for start > 0 && isRegexpSpace(s[start-1]) {
			start--
		}
		if start > 0 && s[start-1] == ',' {
			start--
		}
		return s[:start] + "-" + s[len(s)-1:] + ".0"
	}

	var result strings.Builder
	result.Grow(len(s) + 1)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			continue
		}
		matchStart := i
		for matchStart > start && isRegexpSpace(s[matchStart-1]) {
			matchStart--
		}
		if matchStart > start && s[matchStart-1] == ',' {
			matchStart--
		}
		result.WriteString(s[start:matchStart])
		result.WriteByte('-')
		result.WriteByte(s[i])
		start = i + 1
	}
	result.WriteString(s[start:])
	return result.String()
}

func replaceVersion(s string, atEnd bool) string {
	var result strings.Builder
	result.Grow(len(s) + len(".0"))
	written := 0
	searchFrom := 0
	for {
		start, end, digit, ok := findVersion(s, searchFrom, atEnd)
		if !ok {
			if written == 0 {
				return s
			}
			result.WriteString(s[written:])
			return result.String()
		}
		result.WriteString(s[written:start])
		result.WriteByte('-')
		result.WriteByte(digit)
		if atEnd {
			result.WriteString(".0")
		}
		written = end
		searchFrom = end
	}
}

func findVersion(s string, searchFrom int, atEnd bool) (int, int, byte, bool) {
	for i := searchFrom; i < len(s); i++ {
		var tokenEnd int
		switch {
		case hasPrefixFold(s[i:], "Version"):
			tokenEnd = i + len("Version")
		case s[i] == 'v' || s[i] == 'V':
			tokenEnd = i + 1
			if tokenEnd < len(s) && s[tokenEnd] == '.' {
				tokenEnd++
			}
		default:
			continue
		}

		digit := tokenEnd
		for digit < len(s) && isRegexpSpace(s[digit]) {
			digit++
		}
		if digit >= len(s) || s[digit] < '0' || s[digit] > '9' || atEnd && digit != len(s)-1 {
			continue
		}

		start := i
		for start > searchFrom && isRegexpSpace(s[start-1]) {
			start--
		}
		if start > searchFrom && s[start-1] == ',' {
			start--
		}
		return start, digit + 1, s[digit], true
	}
	return 0, 0, 0, false
}

func hasDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			return true
		}
	}
	return false
}

func hasVersionWord(s string) bool {
	return containsFold(s, "v") || containsFold(s, "version")
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
	initMaps()

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
	if hasPlus && hasASCIILower(base) {
		// The former uppercase transform changed mixed-case canonical IDs, which
		// caused the base lookup below to preserve the suffix. Keep that behavior
		// without allocating an uppercase copy.
		if id := lookupLicense(base); id != "" {
			return upgradeGPL(id + "+")
		}
	}

	for _, t := range transforms {
		transformed := strings.TrimSpace(t(s))
		if transformed != s {
			if id := lookupLicense(transformed); id != "" {
				return upgradeGPL(id)
			}
		}

		// Also try transform on base (without +) and add + back
		if hasPlus {
			transformedBase := strings.TrimSpace(t(base))
			if transformedBase != base {
				if id := lookupLicense(transformedBase); id != "" {
					return upgradeGPL(id + "+")
				}
			}
		}
	}
	return ""
}

func hasASCIILower(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			return true
		}
	}
	return false
}

// tryTranspositions applies transpositions and then transforms.
func tryTranspositions(s string) string {
	for _, trans := range transpositions {
		if containsFold(s, trans.fromUpper) {
			corrected := replaceAllFold(s, trans.from, trans.to, trans.re)

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
	for _, lr := range lastResorts {
		if containsFold(s, lr.substring) {
			return upgradeGPL(lr.license)
		}
	}
	return ""
}

// tryTranspositionsWithLastResorts applies transpositions then last resorts.
func tryTranspositionsWithLastResorts(s string) string {
	for _, trans := range transpositions {
		if containsFold(s, trans.fromUpper) {
			corrected := replaceAllFold(s, trans.from, trans.to, trans.re)

			if result := tryLastResorts(corrected); result != "" {
				return result
			}
		}
	}
	return ""
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func containsFold(s, substring string) bool {
	if substring == "" {
		return true
	}
	if !isASCII(s) {
		return strings.Contains(strings.ToUpper(s), strings.ToUpper(substring))
	}
	return indexFoldASCII(s, substring, 0) >= 0
}

func indexFoldASCII(s, substring string, start int) int {
	limit := len(s) - len(substring)
	for i := start; i <= limit; i++ {
		matched := true
		for j := 0; j < len(substring); j++ {
			left := s[i+j]
			right := substring[j]
			if left >= 'a' && left <= 'z' {
				left -= 'a' - 'A'
			}
			if right >= 'a' && right <= 'z' {
				right -= 'a' - 'A'
			}
			if left != right {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func replaceAllFold(s, old, replacement string, fallback *regexp.Regexp) string {
	if !isASCII(s) {
		return fallback.ReplaceAllString(s, replacement)
	}

	searchFrom := 0
	firstChange := -1
	for {
		match := indexFoldASCII(s, old, searchFrom)
		if match < 0 {
			return s
		}
		if len(old) != len(replacement) || s[match:match+len(old)] != replacement {
			firstChange = match
			break
		}
		searchFrom = match + len(old)
	}

	var result strings.Builder
	result.Grow(len(s) + len(replacement) - len(old))
	result.WriteString(s[:firstChange])
	searchFrom = firstChange
	for {
		match := indexFoldASCII(s, old, searchFrom)
		if match < 0 {
			result.WriteString(s[searchFrom:])
			break
		}
		result.WriteString(s[searchFrom:match])
		result.WriteString(replacement)
		searchFrom = match + len(old)
	}
	return result.String()
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
