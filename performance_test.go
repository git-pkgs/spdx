package spdx

import (
	"strings"
	"testing"
)

func TestValidMatchesParseStrict(t *testing.T) {
	testCases := []string{
		"MIT",
		"mit OR Apache-2.0",
		"(GPL-2.0-only) WITH Classpath-exception-2.0",
		"GPL-2.0-only WITH Classpath-exception-2.0 OR MIT",
		"LicenseRef-custom",
		"DocumentRef-doc:LicenseRef-custom",
		"NONE OR MIT",
		"",
		"MIT OR",
		"(MIT OR Apache-2.0) WITH LLVM-exception",
		"LicenseRef-custom WITH LLVM-exception",
		"DocumentRef-:LicenseRef-custom",
		"DocumentRef-doc:LicenseRef-",
		strings.Repeat("(", maxParseDepth+1) + "MIT" + strings.Repeat(")", maxParseDepth+1),
		strings.Repeat("X", maxParseLength+1),
	}

	for _, expression := range testCases {
		_, err := ParseStrict(expression)
		if got, want := Valid(expression), err == nil; got != want {
			t.Errorf("Valid(%q) = %t, ParseStrict error = %v", expression, got, err)
		}
	}
}

func TestCanonicalFastPathsDoNotAllocate(t *testing.T) {
	if allocations := testing.AllocsPerRun(100, func() {
		_, _ = Normalize("Apache-2.0")
	}); allocations != 0 {
		t.Errorf("Normalize canonical identifier allocations = %v, want 0", allocations)
	}

	if allocations := testing.AllocsPerRun(100, func() {
		_ = Valid("MIT OR Apache-2.0")
	}); allocations != 0 {
		t.Errorf("Valid canonical expression allocations = %v, want 0", allocations)
	}
}
