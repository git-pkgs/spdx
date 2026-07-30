package spdx_test

import (
	"strings"
	"testing"

	"github.com/git-pkgs/spdx"
)

type expressionWrapper struct {
	spdx.Expression
}

func TestRewriteIdentifiersEmbeddedExpression(t *testing.T) {
	t.Parallel()

	expression, err := spdx.ParseStrict("MIT OR Apache-2.0")
	if err != nil {
		t.Fatal(err)
	}
	wrapped := expressionWrapper{Expression: expression}
	if got := spdx.RewriteIdentifiers(wrapped, strings.ToLower); got != "mit OR apache-2.0" {
		t.Errorf("RewriteIdentifiers = %q", got)
	}
}
