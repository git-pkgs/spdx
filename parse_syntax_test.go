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
			name:  "exception",
			input: "GPL-2.0-only WITH Future-exception-9.9",
			want:  "GPL-2.0-only WITH Future-exception-9.9",
			err:   ErrInvalidException,
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

func TestParseSyntaxMatchesStrictAST(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"mit",
		"MIT OR Apache-2.0 AND BSD-3-Clause",
		"(MIT OR Apache-2.0) AND GPL-2.0-only",
		"GPL-2.0-only WITH Classpath-exception-2.0",
		"GPL-2.0+",
		"LicenseRef-custom AND DocumentRef-vendor:LicenseRef-proprietary",
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
