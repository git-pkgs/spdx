package spdx

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseSyntaxAllowsUnknownIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		err   error
	}{
		{
			name:  "license",
			input: "Future-License-9.9 OR MIT",
			want:  "Future-License-9.9 OR MIT",
			err:   ErrInvalidLicenseID,
		},
		{
			name:  "license with interior punctuation",
			input: "Future.License-9.9 OR MIT",
			want:  "Future.License-9.9 OR MIT",
			err:   ErrInvalidLicenseID,
		},
		{
			name:  "exception",
			input: "GPL-2.0-only WITH Future-exception-9.9",
			want:  "GPL-2.0-only WITH Future-exception-9.9",
			err:   ErrInvalidException,
		},
		{
			name:  "ScanCode exception reference",
			input: "GPL-2.0-only WITH LicenseRef-scancode-cygwin-exception-2.0",
			want:  "GPL-2.0-only WITH LicenseRef-scancode-cygwin-exception-2.0",
			err:   ErrMissingOperand,
		},
		{
			name:  "custom exception reference",
			input: "gpl-2.0-ONLY WITH licenseref-Custom.1-2",
			want:  "GPL-2.0-only WITH licenseref-Custom.1-2",
			err:   ErrMissingOperand,
		},
		{
			name:  "license with edge punctuation",
			input: "-Future-License. OR - OR .",
			want:  "-Future-License. OR - OR .",
			err:   ErrInvalidLicenseID,
		},
		{
			name:  "exception with edge punctuation",
			input: "GPL-2.0-only WITH .Future-exception-",
			want:  "GPL-2.0-only WITH .Future-exception-",
			err:   ErrInvalidException,
		},
		{
			name:  "exception reference with edge punctuation",
			input: "GPL-2.0-only WITH LicenseRef--custom.",
			want:  "GPL-2.0-only WITH LicenseRef--custom.",
			err:   ErrMissingOperand,
		},
		{
			name:  "references with edge punctuation",
			input: "LicenseRef-scancode-gradle-enterprise-sla-2022-11- OR DocumentRef--vendor.:LicenseRef--",
			want:  "LicenseRef-scancode-gradle-enterprise-sla-2022-11- OR DocumentRef--vendor.:LicenseRef--",
		},
		{
			name:  "exception reference with plus modifier",
			input: "GPL-2.0+ WITH LicenseRef-custom",
			want:  "GPL-2.0+ WITH LicenseRef-custom",
			err:   ErrMissingOperand,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expression, err := ParseSyntax(test.input)
			if err != nil {
				t.Fatalf("ParseSyntax(%q): %v", test.input, err)
			}
			if got := expression.String(); got != test.want {
				t.Errorf("ParseSyntax(%q) = %q, want %q", test.input, got, test.want)
			}

			_, err = ParseStrict(test.input)
			if !errors.Is(err, test.err) {
				t.Errorf("ParseStrict(%q) error = %v, want %v", test.input, err, test.err)
			}
		})
	}
}

func TestParseSyntaxExceptionReferenceRewrite(t *testing.T) {
	t.Parallel()

	exception := "LicenseRef-scancode-cygwin-exception-2.0"
	input := "MIT OR GPL-2.0-only WITH " + exception + " AND Apache-2.0"
	expression, err := ParseSyntax(input)
	if err != nil {
		t.Fatal(err)
	}
	want := "MIT OR (GPL-2.0-only WITH " + exception + " AND Apache-2.0)"
	if got := expression.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	wantAST := &OrExpression{
		Left: &License{ID: "MIT"},
		Right: &AndExpression{
			Left:  &License{ID: "GPL-2.0-only", Exception: exception},
			Right: &License{ID: "Apache-2.0"},
		},
	}
	if !reflect.DeepEqual(expression, wantAST) {
		t.Errorf("AST = %#v, want %#v", expression, wantAST)
	}

	var identifiers []string
	rewritten := RewriteIdentifiers(expression, func(id string) string {
		identifiers = append(identifiers, id)
		if id == exception {
			return "custom-exception"
		}
		return id
	})
	wantIdentifiers := []string{"MIT", "GPL-2.0-only", exception, "Apache-2.0"}
	if !reflect.DeepEqual(identifiers, wantIdentifiers) {
		t.Errorf("identifiers = %q, want %q", identifiers, wantIdentifiers)
	}
	wantRewritten := "MIT OR (GPL-2.0-only WITH custom-exception AND Apache-2.0)"
	if rewritten != wantRewritten {
		t.Errorf("RewriteIdentifiers() = %q, want %q", rewritten, wantRewritten)
	}
	if got := expression.String(); got != want {
		t.Errorf("String() after rewrite = %q, want %q", got, want)
	}
	if _, err := ParseStrict(input); err == nil {
		t.Error("ParseStrict() accepted an unknown exception reference")
	}
	if Valid(input) {
		t.Error("Valid() accepted an unknown exception reference")
	}
}

func TestParseSyntaxMatchesStrictAST(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"mit",
		"MIT OR Apache-2.0 AND BSD-3-Clause",
		"(MIT OR Apache-2.0) AND GPL-2.0-only",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"GPL-2.0+",
		"LicenseRef-custom AND DocumentRef-vendor:LicenseRef-proprietary",
		"LicenseRef-custom.1-2 AND DocumentRef-vendor.1:LicenseRef-proprietary-2",
		"NONE",
		"NOASSERTION",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			syntaxExpression, err := ParseSyntax(input)
			if err != nil {
				t.Fatalf("ParseSyntax(%q): %v", input, err)
			}
			strictExpression, err := ParseStrict(input)
			if err != nil {
				t.Fatalf("ParseStrict(%q): %v", input, err)
			}
			if !reflect.DeepEqual(syntaxExpression, strictExpression) {
				t.Errorf(
					"ASTs differ:\nParseSyntax: %#v\nParseStrict: %#v",
					syntaxExpression,
					strictExpression,
				)
			}
		})
	}
}

func TestParseSyntaxRejectsMalformedExpressions(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"MIT OR",
		"AND MIT",
		"(MIT OR Apache-2.0",
		"MIT OR Apache-2.0)",
		"MIT WITH",
		"MIT/Apache-2.0",
		"Future_License-1.0",
		"MIT WITH Future/exception",
		"MIT WITH LicenseRef-",
		"MIT WITH LicenseRef-invalid/value",
		"MIT WITH LicenseRef-custom+",
		"MIT WITH LicenseRef-custom WITH LicenseRef-other",
		"LicenseRef-",
		"LicenseRef-invalid/value",
		"DocumentRef-vendor",
		"DocumentRef-:LicenseRef-custom",
		"DocumentRef-vendor:LicenseRef-",
		"DocumentRef-vendor:LicenseRef-invalid/value",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseSyntax(input); err == nil {
				t.Errorf("ParseSyntax(%q) succeeded", input)
			}
			if _, err := ParseStrict(input); err == nil {
				t.Errorf("ParseStrict(%q) succeeded", input)
			}
		})
	}
}

func TestParseSyntaxEnforcesLimits(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("X", maxParseLength+1)
	if _, err := ParseSyntax(oversized); !errors.Is(err, ErrExpressionTooLarge) {
		t.Errorf("oversized expression error = %v, want %v", err, ErrExpressionTooLarge)
	}

	deeplyNested := strings.Repeat("(", maxParseDepth+1) +
		"Future-License-9.9" +
		strings.Repeat(")", maxParseDepth+1)
	if _, err := ParseSyntax(deeplyNested); !errors.Is(err, ErrExpressionTooLarge) {
		t.Errorf("deeply nested expression error = %v, want %v", err, ErrExpressionTooLarge)
	}
}

func TestLicenseRefWithTrailingHyphen(t *testing.T) {
	t.Parallel()

	const input = "MIT OR LicenseRef-scancode-gradle-enterprise-sla-2022-11-"
	if !Valid(input) {
		t.Errorf("Valid(%q) = false", input)
	}
	got, err := NormalizeExpression(input)
	if err != nil {
		t.Fatalf("NormalizeExpression(%q): %v", input, err)
	}
	if got != input {
		t.Errorf("NormalizeExpression(%q) = %q", input, got)
	}
}
