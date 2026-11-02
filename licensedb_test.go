package spdx

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/github/go-spdx/v2/spdxexp"
)

func TestExtractLicensesMatchesGoSPDX(t *testing.T) {
	t.Parallel()

	var counts map[string]int
	data, err := os.ReadFile("real_licenses.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &counts); err != nil {
		t.Fatal(err)
	}
	inputs := make([]string, 0, len(counts))
	for input := range counts {
		inputs = append(inputs, input)
	}
	initCategoryMap()
	for _, entry := range licenseData {
		if key := entry.SPDXLicenseKey; key != "" {
			inputs = append(inputs, key+"+ OR MIT", key+"-only", key+"-or-later", key+"-or-later WITH LLVM-exception", key+"-only+")
		}
	}

	for _, input := range inputs {
		want, err := spdxexp.ExtractLicenses(input)
		if err != nil {
			continue
		}
		got, err := ExtractLicenses(input)
		if err != nil {
			t.Errorf("ExtractLicenses(%q): %v", input, err)
			continue
		}
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("ExtractLicenses(%q) = %q, go-spdx = %q", input, got, want)
		}
	}
}

func TestExtractLicensesScanCodeException(t *testing.T) {
	t.Parallel()

	got, err := ExtractLicenses("MIT OR GPL-2.0+ WITH LicenseRef-scancode-bacula-exception OR MIT")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GPL-2.0-or-later WITH LicenseRef-scancode-bacula-exception", "MIT"}
	if !slices.Equal(got, want) {
		t.Errorf("ExtractLicenses() = %q, want %q", got, want)
	}
}

func TestExtractLicensesRedundantSuffixes(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"Apache-2.0-only":                   {"Apache-2.0"},
		"MIT-or-later":                      {"MIT+"},
		"MIT-only OR BSD-3-Clause-or-later": {"BSD-3-Clause+", "MIT"},
		"GPL-2.0-or-later-only":             {"GPL-2.0-or-later"},
	}
	for input, want := range tests {
		got, err := ExtractLicenses(input)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("ExtractLicenses(%q) = %q, %v, want %q", input, got, err, want)
		}
	}
	if !IsFullyPermissive("Apache-2.0-only AND MIT-or-later") {
		t.Error("IsFullyPermissive rejected redundant suffixes")
	}
	if Valid("Apache-2.0-only") {
		t.Error("Valid accepted a redundant suffix")
	}
}

func TestParseScanCodeException(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"GPL-2.0-only WITH LicenseRef-scancode-bacula-exception": "GPL-2.0-only WITH LicenseRef-scancode-bacula-exception",
		// Informal names go through the normalization pass.
		"gpl v2 WITH LicenseRef-scancode-bacula-exception OR mit license": "(GPL-2.0-only WITH LicenseRef-scancode-bacula-exception) OR MIT",
	}
	for input, want := range tests {
		got, err := NormalizeExpression(input)
		if err != nil {
			t.Errorf("NormalizeExpression(%q): %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeExpression(%q) = %q, want %q", input, got, want)
		}
		if got, err := NormalizeExpressionLax(input); err != nil || got != want {
			t.Errorf("NormalizeExpressionLax(%q) = %q, %v, want %q", input, got, err, want)
		}
	}

	for _, input := range []string{
		"GPL-2.0-only WITH LicenseRef-scancode-bacula-exception",
		"GPL-2.0-only WITH LicenseRef-",
	} {
		if _, err := ParseStrict(input); err == nil {
			t.Errorf("ParseStrict(%q) succeeded", input)
		}
		if Valid(input) {
			t.Errorf("Valid(%q) = true", input)
		}
	}
	for _, input := range []string{
		"GPL-2.0-only WITH LicenseRef-bad/value",
		"GPL-2.0-only WITH LicenseRef-",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("Parse(%q) succeeded", input)
		}
	}
}
