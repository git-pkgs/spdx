package spdx

import (
	"fmt"
	"strings"
	"testing"
)

func cloneRewritten(expression Expression, rewrite func(string) string) Expression {
	switch e := expression.(type) {
	case *License:
		out := &License{ID: rewrite(e.ID), Plus: e.Plus}
		if e.Exception != "" {
			out.Exception = rewrite(e.Exception)
		}
		return out
	case *LicenseRef:
		return &License{ID: rewrite(e.String())}
	case *AndExpression:
		return &AndExpression{Left: cloneRewritten(e.Left, rewrite), Right: cloneRewritten(e.Right, rewrite)}
	case *OrExpression:
		return &OrExpression{Left: cloneRewritten(e.Left, rewrite), Right: cloneRewritten(e.Right, rewrite)}
	case *SpecialValue:
		return &SpecialValue{Value: e.Value}
	default:
		panic("unexpected expression type")
	}
}

func FuzzRewriteMatchesClone(f *testing.F) {
	for _, input := range []string{"MIT", "MIT OR Apache-2.0 AND ISC", "GPL-2.0+ OR GPL-2.0 WITH Classpath-exception-2.0", "LicenseRef-custom", "DocumentRef-x:LicenseRef-y AND (MIT OR ISC)", "NONE"} {
		f.Add(input, false)
		f.Add(input, true)
	}
	f.Fuzz(func(t *testing.T, input string, erase bool) {
		if len(input) > 4096 {
			t.Skip()
		}
		expression, err := ParseSyntax(input)
		if err != nil {
			return
		}
		original := expression.String()
		rewrite := func(id string) string {
			if erase && strings.Contains(id, "exception") {
				return ""
			}
			return strings.ToLower(id)
		}
		want := cloneRewritten(expression, rewrite).String()
		if got := RewriteIdentifiers(expression, rewrite); got != want {
			t.Fatalf("%q: got %q, want %q", input, got, want)
		}
		if expression.String() != original {
			t.Fatal("input expression mutated")
		}
	})
}

func BenchmarkRewriteSize(b *testing.B) {
	for _, size := range []int{1, 4, 32, 128} {
		expression, err := ParseSyntax(strings.TrimSuffix(strings.Repeat("MIT OR ", size), " OR "))
		if err != nil {
			b.Fatal(err)
		}
		for _, clone := range []bool{false, true} {
			b.Run(fmt.Sprintf("%d/clone=%t", size, clone), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if clone {
						_ = cloneRewritten(expression, strings.ToLower).String()
					} else {
						_ = RewriteIdentifiers(expression, strings.ToLower)
					}
				}
			})
		}
	}
}
