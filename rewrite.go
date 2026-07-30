package spdx

// RewriteIdentifiers applies rewrite to each license identifier, exception
// identifier, and complete license reference in expression order. Operators,
// modifiers, and operator precedence are preserved. Replacements are inserted
// verbatim and are not validated as SPDX identifiers.
//
// A nil rewrite function returns expression.String().
func RewriteIdentifiers(expression Expression, rewrite func(string) string) string {
	if rewrite == nil {
		return expression.String()
	}
	return expression.rewriteIdentifiers(rewrite).String()
}

func (license *License) rewriteIdentifiers(rewrite func(string) string) Expression {
	rewritten := &License{
		ID:   rewrite(license.ID),
		Plus: license.Plus,
	}
	if license.Exception != "" {
		rewritten.Exception = rewrite(license.Exception)
	}
	return rewritten
}

func (reference *LicenseRef) rewriteIdentifiers(rewrite func(string) string) Expression {
	return &License{ID: rewrite(reference.String())}
}

func (expression *AndExpression) rewriteIdentifiers(rewrite func(string) string) Expression {
	return &AndExpression{
		Left:  expression.Left.rewriteIdentifiers(rewrite),
		Right: expression.Right.rewriteIdentifiers(rewrite),
	}
}

func (expression *OrExpression) rewriteIdentifiers(rewrite func(string) string) Expression {
	return &OrExpression{
		Left:  expression.Left.rewriteIdentifiers(rewrite),
		Right: expression.Right.rewriteIdentifiers(rewrite),
	}
}

func (special *SpecialValue) rewriteIdentifiers(func(string) string) Expression {
	return &SpecialValue{Value: special.Value}
}
