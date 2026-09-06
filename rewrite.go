package spdx

import "strings"

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
	switch value := expression.(type) {
	case *License:
		license := value.rewrittenLicense(rewrite)
		return license.String()
	case *LicenseRef:
		return rewrite(value.String())
	case *SpecialValue:
		return value.Value
	}
	var out strings.Builder
	expression.writeRewritten(&out, rewrite, tokenEOF)
	return out.String()
}

func (license *License) writeRewritten(out *strings.Builder, rewrite func(string) string, parent tokenType) {
	rewritten := license.rewrittenLicense(rewrite)
	paren := parent == tokenOr && rewritten.Exception != ""
	if paren {
		out.WriteByte('(')
	}
	out.WriteString(rewritten.ID)
	if rewritten.Plus {
		out.WriteByte('+')
	}
	if rewritten.Exception != "" {
		out.WriteString(" WITH ")
		out.WriteString(rewritten.Exception)
	}
	if paren {
		out.WriteByte(')')
	}
}

func (license *License) rewrittenLicense(rewrite func(string) string) License {
	rewritten := License{ID: rewrite(license.ID), Plus: license.Plus}
	if license.Exception != "" {
		rewritten.Exception = rewrite(license.Exception)
	}
	return rewritten
}

func (reference *LicenseRef) writeRewritten(out *strings.Builder, rewrite func(string) string, _ tokenType) {
	out.WriteString(rewrite(reference.String()))
}

func (expression *AndExpression) writeRewritten(out *strings.Builder, rewrite func(string) string, parent tokenType) {
	if parent == tokenOr {
		out.WriteByte('(')
	}
	expression.Left.writeRewritten(out, rewrite, tokenAnd)
	out.WriteString(" AND ")
	expression.Right.writeRewritten(out, rewrite, tokenAnd)
	if parent == tokenOr {
		out.WriteByte(')')
	}
}

func (expression *OrExpression) writeRewritten(out *strings.Builder, rewrite func(string) string, parent tokenType) {
	if parent == tokenAnd {
		out.WriteByte('(')
	}
	expression.Left.writeRewritten(out, rewrite, tokenOr)
	out.WriteString(" OR ")
	expression.Right.writeRewritten(out, rewrite, tokenOr)
	if parent == tokenAnd {
		out.WriteByte(')')
	}
}

func (special *SpecialValue) writeRewritten(out *strings.Builder, _ func(string) string, _ tokenType) {
	out.WriteString(special.Value)
}
