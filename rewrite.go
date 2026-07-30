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
	return rewriteExpressionIdentifiers(expression, rewrite).String()
}

func rewriteExpressionIdentifiers(
	expression Expression,
	rewrite func(string) string,
) Expression {
	switch node := expression.(type) {
	case *License:
		license := &License{
			ID:   rewrite(node.ID),
			Plus: node.Plus,
		}
		if node.Exception != "" {
			license.Exception = rewrite(node.Exception)
		}
		return license
	case *LicenseRef:
		return &License{ID: rewrite(node.String())}
	case *AndExpression:
		return &AndExpression{
			Left:  rewriteExpressionIdentifiers(node.Left, rewrite),
			Right: rewriteExpressionIdentifiers(node.Right, rewrite),
		}
	case *OrExpression:
		return &OrExpression{
			Left:  rewriteExpressionIdentifiers(node.Left, rewrite),
			Right: rewriteExpressionIdentifiers(node.Right, rewrite),
		}
	case *SpecialValue:
		return &SpecialValue{Value: node.Value}
	default:
		panic("spdx: unsupported expression type")
	}
}
