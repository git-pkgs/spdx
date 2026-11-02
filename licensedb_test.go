package spdx

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
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

var expressionParsers = map[string]func(string) (Expression, error){
	"Parse":       Parse,
	"ParseStrict": ParseStrict,
	"ParseSyntax": ParseSyntax,
}

func TestLicenseRefWithException(t *testing.T) {
	t.Parallel()

	input := "MIT OR LicenseRef-scancode-gpl-2.0-djvu WITH Classpath-exception-2.0"
	want := "MIT OR (LicenseRef-scancode-gpl-2.0-djvu WITH Classpath-exception-2.0)"
	wantAST := &OrExpression{
		Left:  &License{ID: "MIT"},
		Right: &LicenseRef{LicenseRef: "scancode-gpl-2.0-djvu", Exception: "Classpath-exception-2.0"},
	}
	for name, parse := range expressionParsers {
		expression, err := parse(input)
		if err != nil {
			t.Errorf("%s(%q): %v", name, input, err)
			continue
		}
		if !reflect.DeepEqual(expression, wantAST) {
			t.Errorf("%s(%q) = %#v, want %#v", name, input, expression, wantAST)
		}
		if got := expression.String(); got != want {
			t.Errorf("%s(%q).String() = %q, want %q", name, input, got, want)
		}
		if got := expression.Licenses(); !slices.Equal(got, []string{"MIT", "LicenseRef-scancode-gpl-2.0-djvu"}) {
			t.Errorf("%s(%q).Licenses() = %q", name, input, got)
		}
		rewritten := RewriteIdentifiers(expression, strings.ToUpper)
		if rewritten != "MIT OR (LICENSEREF-SCANCODE-GPL-2.0-DJVU WITH CLASSPATH-EXCEPTION-2.0)" {
			t.Errorf("RewriteIdentifiers() = %q", rewritten)
		}
		dropException := func(id string) string {
			if id == "Classpath-exception-2.0" {
				return ""
			}
			return id
		}
		if rewritten := RewriteIdentifiers(expression, dropException); rewritten != "MIT OR LicenseRef-scancode-gpl-2.0-djvu" {
			t.Errorf("RewriteIdentifiers() with empty exception = %q", rewritten)
		}
	}

	if !Valid(input) {
		t.Errorf("Valid(%q) = false", input)
	}
	got, err := NormalizeExpression("mit license OR DocumentRef-a:LicenseRef-b WITH LicenseRef-scancode-bacula-exception")
	if err != nil || got != "MIT OR (DocumentRef-a:LicenseRef-b WITH LicenseRef-scancode-bacula-exception)" {
		t.Errorf("NormalizeExpression() = %q, %v", got, err)
	}
	licenses, err := ExtractLicenses(input)
	if err != nil || !slices.Equal(licenses, []string{"LicenseRef-scancode-gpl-2.0-djvu WITH Classpath-exception-2.0", "MIT"}) {
		t.Errorf("ExtractLicenses(%q) = %q, %v", input, licenses, err)
	}
	if !HasCopyleft("LicenseRef-scancode-gpl-2.0-djvu WITH Classpath-exception-2.0") {
		t.Error("HasCopyleft ignored a copyleft LicenseRef with an exception")
	}
}

func TestLicenseRefWithExceptionInvalid(t *testing.T) {
	t.Parallel()

	for _, invalid := range []string{
		"LicenseRef-a WITH Not-An-Exception",
		"(LicenseRef-a OR MIT) WITH Classpath-exception-2.0",
		"LicenseRef-a WITH Classpath-exception-2.0 WITH Classpath-exception-2.0",
		"(LicenseRef-a WITH Classpath-exception-2.0) WITH LLVM-exception",
		"(MIT WITH Classpath-exception-2.0) WITH LLVM-exception",
	} {
		for name, parse := range expressionParsers {
			if name == "ParseSyntax" && strings.Contains(invalid, "Not-An-Exception") {
				continue // unknown exceptions are allowed
			}
			if _, err := parse(invalid); err == nil {
				t.Errorf("%s(%q) succeeded", name, invalid)
			}
		}
		if Valid(invalid) {
			t.Errorf("Valid(%q) = true", invalid)
		}
	}
}
