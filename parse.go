package spdx

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Expression represents a parsed SPDX expression.
type Expression interface {
	// String returns the normalized string representation.
	String() string
	// Licenses returns all license identifiers in the expression.
	Licenses() []string
	isExpr()
}

// License represents a single SPDX license identifier.
type License struct {
	ID       string // The canonical license ID
	Plus     bool   // True if followed by +
	Exception string // Exception ID if using WITH
}

func (l *License) String() string {
	s := l.ID
	if l.Plus {
		s += "+"
	}
	if l.Exception != "" {
		s += " WITH " + l.Exception
	}
	return s
}

func (l *License) Licenses() []string {
	return []string{l.ID}
}

func (l *License) isExpr() {}

// LicenseRef represents a custom license reference.
type LicenseRef struct {
	DocumentRef string // Optional document reference
	LicenseRef  string // The license reference ID
}

func (l *LicenseRef) String() string {
	if l.DocumentRef != "" {
		return "DocumentRef-" + l.DocumentRef + ":LicenseRef-" + l.LicenseRef
	}
	return "LicenseRef-" + l.LicenseRef
}

func (l *LicenseRef) Licenses() []string {
	return []string{l.String()}
}

func (l *LicenseRef) isExpr() {}

// AndExpression represents an AND combination of expressions.
type AndExpression struct {
	Left  Expression
	Right Expression
}

func (e *AndExpression) String() string {
	left := e.Left.String()
	right := e.Right.String()

	// Wrap OR expressions in parentheses for correct precedence
	if _, ok := e.Left.(*OrExpression); ok {
		left = "(" + left + ")"
	}
	if _, ok := e.Right.(*OrExpression); ok {
		right = "(" + right + ")"
	}

	return left + " AND " + right
}

func (e *AndExpression) Licenses() []string {
	return append(e.Left.Licenses(), e.Right.Licenses()...)
}

func (e *AndExpression) isExpr() {}

// OrExpression represents an OR combination of expressions.
type OrExpression struct {
	Left  Expression
	Right Expression
}

func (e *OrExpression) String() string {
	left := e.Left.String()
	right := e.Right.String()

	// Wrap AND expressions and WITH licenses in parentheses for clarity
	if _, ok := e.Left.(*AndExpression); ok {
		left = "(" + left + ")"
	}
	if _, ok := e.Right.(*AndExpression); ok {
		right = "(" + right + ")"
	}
	// License with exception should also be wrapped
	if lic, ok := e.Right.(*License); ok && lic.Exception != "" {
		right = "(" + right + ")"
	}
	if lic, ok := e.Left.(*License); ok && lic.Exception != "" {
		left = "(" + left + ")"
	}

	return left + " OR " + right
}

func (e *OrExpression) Licenses() []string {
	return append(e.Left.Licenses(), e.Right.Licenses()...)
}

func (e *OrExpression) isExpr() {}

// SpecialValue represents NONE or NOASSERTION.
type SpecialValue struct {
	Value string
}

func (s *SpecialValue) String() string {
	return s.Value
}

func (s *SpecialValue) Licenses() []string {
	return nil
}

func (s *SpecialValue) isExpr() {}

// Parser errors
var (
	ErrEmptyExpression     = errors.New("empty expression")
	ErrUnexpectedToken     = errors.New("unexpected token")
	ErrUnbalancedParens    = errors.New("unbalanced parentheses")
	ErrInvalidLicenseID    = errors.New("invalid license identifier")
	ErrInvalidException    = errors.New("invalid exception identifier")
	ErrMissingOperand      = errors.New("missing operand")
	ErrInvalidSpecialValue = errors.New("NONE and NOASSERTION must be standalone")
)

// tokenType represents the type of a lexer token.
type tokenType int

const (
	tokenLicense tokenType = iota
	tokenLicenseRef
	tokenDocumentRef
	tokenAnd
	tokenOr
	tokenWith
	tokenPlus
	tokenOpenParen
	tokenCloseParen
	tokenEOF
)

type token struct {
	typ   tokenType
	value string
}

// lexer tokenizes an SPDX expression.
type lexer struct {
	input string
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: input}
}

func (l *lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *lexer) next() (token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return token{typ: tokenEOF}, nil
	}

	ch := l.input[l.pos]

	switch ch {
	case '(':
		l.pos++
		return token{typ: tokenOpenParen, value: "("}, nil
	case ')':
		l.pos++
		return token{typ: tokenCloseParen, value: ")"}, nil
	case '+':
		l.pos++
		return token{typ: tokenPlus, value: "+"}, nil
	}

	// Read identifier or keyword
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsSpace(rune(ch)) || ch == '(' || ch == ')' || ch == '+' {
			break
		}
		l.pos++
	}

	if l.pos == start {
		return token{}, fmt.Errorf("unexpected character: %c", ch)
	}

	word := l.input[start:l.pos]
	upper := strings.ToUpper(word)

	switch upper {
	case "AND":
		return token{typ: tokenAnd, value: "AND"}, nil
	case "OR":
		return token{typ: tokenOr, value: "OR"}, nil
	case "WITH":
		return token{typ: tokenWith, value: "WITH"}, nil
	}

	// Check for DocumentRef or LicenseRef
	if strings.HasPrefix(upper, "DOCUMENTREF-") {
		// DocumentRef-xxx:LicenseRef-yyy
		return token{typ: tokenDocumentRef, value: word}, nil
	}
	if strings.HasPrefix(upper, "LICENSEREF-") {
		return token{typ: tokenLicenseRef, value: word}, nil
	}

	return token{typ: tokenLicense, value: word}, nil
}

// parser parses SPDX expressions.
type parser struct {
	lexer   *lexer
	current token
}

func newParser(input string) (*parser, error) {
	p := &parser{lexer: newLexer(input)}
	tok, err := p.lexer.next()
	if err != nil {
		return nil, err
	}
	p.current = tok
	return p, nil
}

func (p *parser) advance() error {
	tok, err := p.lexer.next()
	if err != nil {
		return err
	}
	p.current = tok
	return nil
}

// Parse parses an SPDX expression string into an Expression tree.
// It handles both strict SPDX identifiers and informal license names
// (like "Apache 2" or "MIT License") by normalizing them automatically.
//
// Example:
//
//	Parse("MIT")                     // *License{ID: "MIT"}
//	Parse("MIT OR Apache-2.0")       // *OrExpression{...}
//	Parse("mit OR apache 2")         // normalizes to "MIT OR Apache-2.0"
//	Parse("GPL v3 AND BSD")          // normalizes to "GPL-3.0-or-later AND BSD-2-Clause"
//
// For strict SPDX-only parsing (no fuzzy normalization), use ParseStrict.
func Parse(expression string) (Expression, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, ErrEmptyExpression
	}

	// Pre-process: normalize informal license names while preserving operators
	normalized, err := normalizeExpressionString(expression)
	if err != nil {
		return nil, err
	}

	p, err := newParser(normalized)
	if err != nil {
		return nil, err
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.current.typ != tokenEOF {
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedToken, p.current.value)
	}

	return expr, nil
}

// ParseStrict parses an SPDX expression requiring strict SPDX identifiers.
// Unlike Parse, it does not normalize informal license names.
// Use this when you need to validate that an expression uses only
// exact SPDX license identifiers.
//
// Example:
//
//	ParseStrict("MIT OR Apache-2.0")  // succeeds
//	ParseStrict("mit OR apache 2")    // fails - "apache 2" is not a valid SPDX ID
func ParseStrict(expression string) (Expression, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, ErrEmptyExpression
	}

	p, err := newParser(expression)
	if err != nil {
		return nil, err
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.current.typ != tokenEOF {
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedToken, p.current.value)
	}

	return expr, nil
}

// parseExpression parses a full expression (handles OR, lowest precedence).
func (p *parser) parseExpression() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current.typ == tokenOr {
		if err := p.advance(); err != nil {
			return nil, err
		}

		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}

		left = &OrExpression{Left: left, Right: right}
	}

	return left, nil
}

// parseAnd parses AND expressions (higher precedence than OR).
func (p *parser) parseAnd() (Expression, error) {
	left, err := p.parseWith()
	if err != nil {
		return nil, err
	}

	for p.current.typ == tokenAnd {
		if err := p.advance(); err != nil {
			return nil, err
		}

		right, err := p.parseWith()
		if err != nil {
			return nil, err
		}

		left = &AndExpression{Left: left, Right: right}
	}

	return left, nil
}

// parseWith parses WITH expressions (higher precedence than AND).
func (p *parser) parseWith() (Expression, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	// WITH only applies to licenses, not expressions
	if p.current.typ == tokenWith {
		license, ok := left.(*License)
		if !ok {
			return nil, fmt.Errorf("%w: WITH can only follow a license", ErrUnexpectedToken)
		}

		if err := p.advance(); err != nil {
			return nil, err
		}

		if p.current.typ != tokenLicense {
			return nil, fmt.Errorf("%w: expected exception after WITH", ErrMissingOperand)
		}

		exception := lookupException(p.current.value)
		if exception == "" {
			return nil, fmt.Errorf("%w: %s", ErrInvalidException, p.current.value)
		}

		license.Exception = exception

		if err := p.advance(); err != nil {
			return nil, err
		}
	}

	return left, nil
}

// parseAtom parses atomic expressions (licenses, refs, parenthesized expressions).
func (p *parser) parseAtom() (Expression, error) {
	switch p.current.typ {
	case tokenOpenParen:
		if err := p.advance(); err != nil {
			return nil, err
		}

		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if p.current.typ != tokenCloseParen {
			return nil, ErrUnbalancedParens
		}

		if err := p.advance(); err != nil {
			return nil, err
		}

		return expr, nil

	case tokenLicense:
		value := p.current.value
		upper := strings.ToUpper(value)

		// Handle special values
		if upper == "NONE" || upper == "NOASSERTION" {
			if err := p.advance(); err != nil {
				return nil, err
			}
			return &SpecialValue{Value: upper}, nil
		}

		// Look up the canonical license ID
		id := lookupLicense(value)
		if id == "" {
			return nil, fmt.Errorf("%w: %s", ErrInvalidLicenseID, value)
		}

		license := &License{ID: id}

		if err := p.advance(); err != nil {
			return nil, err
		}

		// Check for +
		if p.current.typ == tokenPlus {
			license.Plus = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		}

		return license, nil

	case tokenLicenseRef:
		ref := parseLicenseRef(p.current.value)
		if err := p.advance(); err != nil {
			return nil, err
		}
		return ref, nil

	case tokenDocumentRef:
		ref := parseDocumentRef(p.current.value)
		if err := p.advance(); err != nil {
			return nil, err
		}
		return ref, nil

	case tokenEOF:
		return nil, ErrMissingOperand

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedToken, p.current.value)
	}
}

// parseLicenseRef parses "LicenseRef-xxx" into a LicenseRef.
func parseLicenseRef(s string) *LicenseRef {
	// Remove "LicenseRef-" prefix (case insensitive)
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "LICENSEREF-") {
		return &LicenseRef{LicenseRef: s[11:]}
	}
	return &LicenseRef{LicenseRef: s}
}

// parseDocumentRef parses "DocumentRef-xxx:LicenseRef-yyy" into a LicenseRef.
func parseDocumentRef(s string) *LicenseRef {
	// Format: DocumentRef-xxx:LicenseRef-yyy
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "DOCUMENTREF-") {
		rest := s[12:] // after "DocumentRef-"
		if idx := strings.Index(strings.ToUpper(rest), ":LICENSEREF-"); idx != -1 {
			docRef := rest[:idx]
			licRef := rest[idx+12:] // after ":LicenseRef-"
			return &LicenseRef{DocumentRef: docRef, LicenseRef: licRef}
		}
	}
	return &LicenseRef{LicenseRef: s}
}
