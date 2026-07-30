package spdx

import (
	"slices"
	"strings"
	"testing"
)

func TestRewriteIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		rewrite     map[string]string
		want        string
		identifiers []string
	}{
		{
			name:    "license",
			input:   "BSD-3-Clause",
			rewrite: map[string]string{"BSD-3-Clause": "bsd-new"},
			want:    "bsd-new",
			identifiers: []string{
				"BSD-3-Clause",
			},
		},
		{
			name:  "compound expression",
			input: "MIT OR Apache-2.0 AND BSD-3-Clause",
			rewrite: map[string]string{
				"MIT":          "mit",
				"Apache-2.0":   "apache-2.0",
				"BSD-3-Clause": "bsd-new",
			},
			want: "mit OR (apache-2.0 AND bsd-new)",
			identifiers: []string{
				"MIT",
				"Apache-2.0",
				"BSD-3-Clause",
			},
		},
		{
			name:  "plus and exception",
			input: "GPL-2.0+ OR GPL-2.0-only WITH Classpath-exception-2.0",
			rewrite: map[string]string{
				"GPL-2.0":                 "gpl-2.0",
				"GPL-2.0-only":            "gpl-2.0",
				"Classpath-exception-2.0": "classpath-exception-2.0",
			},
			want: "gpl-2.0+ OR (gpl-2.0 WITH classpath-exception-2.0)",
			identifiers: []string{
				"GPL-2.0",
				"GPL-2.0-only",
				"Classpath-exception-2.0",
			},
		},
		{
			name:  "license references",
			input: "LicenseRef-scancode-mit AND DocumentRef-vendor:LicenseRef-custom",
			rewrite: map[string]string{
				"LicenseRef-scancode-mit":              "mit",
				"DocumentRef-vendor:LicenseRef-custom": "unknown-spdx",
			},
			want: "mit AND unknown-spdx",
			identifiers: []string{
				"LicenseRef-scancode-mit",
				"DocumentRef-vendor:LicenseRef-custom",
			},
		},
		{
			name:  "repeated identifier",
			input: "MIT AND MIT",
			rewrite: map[string]string{
				"MIT": "mit",
			},
			want:        "mit AND mit",
			identifiers: []string{"MIT", "MIT"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			expression, err := ParseStrict(test.input)
			if err != nil {
				t.Fatal(err)
			}
			original := expression.String()
			var identifiers []string
			got := RewriteIdentifiers(expression, func(identifier string) string {
				identifiers = append(identifiers, identifier)
				return test.rewrite[identifier]
			})
			if got != test.want {
				t.Errorf("RewriteIdentifiers(%q) = %q, want %q", test.input, got, test.want)
			}
			if !slices.Equal(identifiers, test.identifiers) {
				t.Errorf("identifiers = %q, want %q", identifiers, test.identifiers)
			}
			if expression.String() != original {
				t.Errorf("RewriteIdentifiers mutated expression: got %q, want %q", expression, original)
			}
		})
	}
}

func TestRewriteIdentifiersSpecialValues(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"NONE", "NOASSERTION"} {
		expression, err := ParseStrict(input)
		if err != nil {
			t.Fatal(err)
		}
		got := RewriteIdentifiers(expression, func(identifier string) string {
			t.Fatalf("rewrite called with %q", identifier)
			return ""
		})
		if got != input {
			t.Errorf("RewriteIdentifiers(%q) = %q", input, got)
		}
	}
}

func TestRewriteIdentifiersNilFunction(t *testing.T) {
	t.Parallel()

	expression, err := ParseStrict("MIT OR Apache-2.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := RewriteIdentifiers(expression, nil); got != expression.String() {
		t.Errorf("RewriteIdentifiers with nil function = %q", got)
	}
}

func BenchmarkRewriteIdentifiers(b *testing.B) {
	expression, err := ParseStrict(
		"MIT OR GPL-2.0-only WITH Classpath-exception-2.0",
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		RewriteIdentifiers(expression, strings.ToLower)
	}
}
