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
	writeRewritten(*strings.Builder, func(string) string, tokenType)
	isExpr()
}

// License represents a single SPDX license identifier.
type License struct {
	ID        string // The canonical license ID
	Plus      bool   // True if followed by +
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
	ErrExpressionTooLarge  = errors.New("expression too large")
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

	opAND  = "AND"
	opOR   = "OR"
	opWITH = "WITH"
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

	switch {
	case strings.EqualFold(word, opAND):
		return token{typ: tokenAnd, value: opAND}, nil
	case strings.EqualFold(word, opOR):
		return token{typ: tokenOr, value: opOR}, nil
	case strings.EqualFold(word, opWITH):
		return token{typ: tokenWith, value: opWITH}, nil
	}

	// Check for DocumentRef or LicenseRef
	if hasPrefixFold(word, "DocumentRef-") {
		// DocumentRef-xxx:LicenseRef-yyy
		return token{typ: tokenDocumentRef, value: word}, nil
	}
	if hasPrefixFold(word, "LicenseRef-") {
		return token{typ: tokenLicenseRef, value: word}, nil
	}

	return token{typ: tokenLicense, value: word}, nil
}

const (
	maxParseDepth  = 256
	maxParseLength = 1 << 20 // 1 MiB
)

// parser parses SPDX expressions.
type parser struct {
	lexer                   lexer
	current                 token
	depth                   int
	allowUnknownIdentifiers bool
}

func newParser(input string, allowUnknownIdentifiers bool) (parser, error) {
	p := parser{
		lexer:                   lexer{input: input},
		allowUnknownIdentifiers: allowUnknownIdentifiers,
	}
	tok, err := p.lexer.next()
	if err != nil {
		return parser{}, err
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
	if len(expression) > maxParseLength {
		return nil, ErrExpressionTooLarge
	}

	// Canonical expressions do not need the informal-name tokenization pass.
	p, err := newParser(expression, false)
	if err == nil {
		if parsed, strictErr := p.parse(); strictErr == nil {
			upgradeParsedGPL(parsed)
			return parsed, nil
		}
	}

	// Pre-process: normalize informal license names while preserving operators.
	normalized, err := normalizeExpressionString(expression)
	if err != nil {
		return nil, err
	}

	p, err = newParser(normalized, false)
	if err != nil {
		return nil, err
	}

	return p.parse()
}

func upgradeParsedGPL(expression Expression) {
	switch value := expression.(type) {
	case *License:
		license := value.ID
		if value.Plus {
			license += "+"
		}
		upgraded := upgradeGPL(license)
		if upgraded != license {
			value.ID = upgraded
			value.Plus = false
		}
	case *AndExpression:
		upgradeParsedGPL(value.Left)
		upgradeParsedGPL(value.Right)
	case *OrExpression:
		upgradeParsedGPL(value.Left)
		upgradeParsedGPL(value.Right)
	}
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
	return parseWithoutNormalization(expression, false)
}

// ParseSyntax parses an SPDX expression without requiring bare license and
// exception identifiers to exist in the bundled SPDX identifier list. It
// validates identifier syntax, operators, modifiers, and grouping. Known
// identifiers are returned in their canonical form; unknown identifiers are
// preserved as written.
//
// Use ParseStrict when identifiers must also be present in the bundled SPDX
// list.
//
// Example:
//
//	ParseSyntax("Future-License-1.0 OR MIT") // succeeds
//	ParseStrict("Future-License-1.0 OR MIT") // fails
func ParseSyntax(expression string) (Expression, error) {
	return parseWithoutNormalization(expression, true)
}

func parseWithoutNormalization(
	expression string,
	allowUnknownIdentifiers bool,
) (Expression, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, ErrEmptyExpression
	}
	if len(expression) > maxParseLength {
		return nil, ErrExpressionTooLarge
	}

	p, err := newParser(expression, allowUnknownIdentifiers)
	if err != nil {
		return nil, err
	}

	return p.parse()
}

type validatedExpressionKind uint8

const (
	validatedOther validatedExpressionKind = iota
	validatedLicense
)

type validationParser struct {
	lexer   lexer
	current token
	depth   int
}

func validStrictExpression(expression string) bool {
	expression = strings.TrimSpace(expression)
	if expression == "" || len(expression) > maxParseLength {
		return false
	}

	validator := validationParser{lexer: lexer{input: expression}}
	if !validator.advance() {
		return false
	}
	if _, ok := validator.parseExpression(); !ok {
		return false
	}
	return validator.current.typ == tokenEOF
}

func (p *validationParser) advance() bool {
	tok, err := p.lexer.next()
	if err != nil {
		return false
	}
	p.current = tok
	return true
}

func (p *validationParser) parseExpression() (validatedExpressionKind, bool) {
	kind, ok := p.parseAnd()
	if !ok {
		return validatedOther, false
	}
	for p.current.typ == tokenOr {
		if !p.advance() {
			return validatedOther, false
		}
		if _, ok := p.parseAnd(); !ok {
			return validatedOther, false
		}
		kind = validatedOther
	}
	return kind, true
}

func (p *validationParser) parseAnd() (validatedExpressionKind, bool) {
	kind, ok := p.parseWith()
	if !ok {
		return validatedOther, false
	}
	for p.current.typ == tokenAnd {
		if !p.advance() {
			return validatedOther, false
		}
		if _, ok := p.parseWith(); !ok {
			return validatedOther, false
		}
		kind = validatedOther
	}
	return kind, true
}

func (p *validationParser) parseWith() (validatedExpressionKind, bool) {
	kind, ok := p.parseAtom()
	if !ok {
		return validatedOther, false
	}
	if p.current.typ != tokenWith {
		return kind, true
	}
	if kind != validatedLicense || !p.advance() || p.current.typ != tokenLicense ||
		lookupException(p.current.value) == "" || !p.advance() {
		return validatedOther, false
	}
	return validatedLicense, true
}

func (p *validationParser) parseAtom() (validatedExpressionKind, bool) {
	switch p.current.typ {
	case tokenOpenParen:
		p.depth++
		if p.depth > maxParseDepth || !p.advance() {
			return validatedOther, false
		}
		kind, ok := p.parseExpression()
		if !ok || p.current.typ != tokenCloseParen || !p.advance() {
			return validatedOther, false
		}
		p.depth--
		return kind, true

	case tokenLicense:
		value := p.current.value
		if strings.EqualFold(value, "NONE") || strings.EqualFold(value, "NOASSERTION") {
			return validatedOther, p.advance()
		}
		if lookupLicense(value) == "" || !p.advance() {
			return validatedOther, false
		}
		if p.current.typ == tokenPlus && !p.advance() {
			return validatedOther, false
		}
		return validatedLicense, true

	case tokenLicenseRef:
		if !validLicenseRefString(p.current.value) || !p.advance() {
			return validatedOther, false
		}
		return validatedOther, true

	case tokenDocumentRef:
		if !validDocumentRefString(p.current.value) || !p.advance() {
			return validatedOther, false
		}
		return validatedOther, true

	default:
		return validatedOther, false
	}
}

func validLicenseRefString(reference string) bool {
	return len(reference) > len("LicenseRef-") &&
		hasPrefixFold(reference, "LicenseRef-") &&
		validIdentifier(reference[len("LicenseRef-"):])
}

func validDocumentRefString(reference string) bool {
	if len(reference) <= len("DocumentRef-") || !hasPrefixFold(reference, "DocumentRef-") {
		return false
	}
	rest := reference[len("DocumentRef-"):]
	separator := indexFoldASCII(rest, ":LicenseRef-", 0)
	return separator > 0 &&
		validIdentifier(rest[:separator]) &&
		validIdentifier(rest[separator+len(":LicenseRef-"):])
}

func (p *parser) parse() (Expression, error) {
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

		exception, err := p.resolveExceptionIdentifier(p.current.value)
		if err != nil {
			return nil, err
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
		p.depth++
		if p.depth > maxParseDepth {
			return nil, ErrExpressionTooLarge
		}

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

		p.depth--
		return expr, nil

	case tokenLicense:
		return p.parseLicenseAtom()

	case tokenLicenseRef:
		return p.parseLicenseReference(false)

	case tokenDocumentRef:
		return p.parseLicenseReference(true)

	case tokenEOF:
		return nil, ErrMissingOperand

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedToken, p.current.value)
	}
}

func (p *parser) parseLicenseAtom() (Expression, error) {
	value := p.current.value
	if strings.EqualFold(value, "NONE") || strings.EqualFold(value, "NOASSERTION") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if strings.EqualFold(value, "NONE") {
			return &SpecialValue{Value: "NONE"}, nil
		}
		return &SpecialValue{Value: "NOASSERTION"}, nil
	}

	id, err := p.resolveLicenseIdentifier(value)
	if err != nil {
		return nil, err
	}
	license := &License{ID: id}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.current.typ == tokenPlus {
		license.Plus = true
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return license, nil
}

func (p *parser) resolveLicenseIdentifier(identifier string) (string, error) {
	if id := lookupLicense(identifier); id != "" {
		return id, nil
	}
	if p.allowUnknownIdentifiers && validIdentifier(identifier) {
		return identifier, nil
	}
	return "", fmt.Errorf("%w: %s", ErrInvalidLicenseID, identifier)
}

func (p *parser) resolveExceptionIdentifier(identifier string) (string, error) {
	if exception := lookupException(identifier); exception != "" {
		return exception, nil
	}
	if p.allowUnknownIdentifiers && validIdentifier(identifier) {
		return identifier, nil
	}
	return "", fmt.Errorf("%w: %s", ErrInvalidException, identifier)
}

func (p *parser) parseLicenseReference(document bool) (Expression, error) {
	value := p.current.value
	var reference *LicenseRef
	if document {
		reference = parseDocumentRef(value)
	} else {
		reference = parseLicenseRef(value)
	}
	if !validLicenseReference(reference, document) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidLicenseID, value)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return reference, nil
}

func validLicenseReference(reference *LicenseRef, document bool) bool {
	if reference == nil || !validIdentifier(reference.LicenseRef) {
		return false
	}
	if document {
		return validIdentifier(reference.DocumentRef)
	}
	return reference.DocumentRef == ""
}

func validIdentifier(identifier string) bool {
	if identifier == "" || !isIdentifierAlphanumeric(rune(identifier[0])) ||
		!isIdentifierAlphanumeric(rune(identifier[len(identifier)-1])) {
		return false
	}
	for _, character := range identifier {
		switch {
		case isIdentifierAlphanumeric(character):
		case character == '-', character == '.':
		default:
			return false
		}
	}
	return true
}

func isIdentifierAlphanumeric(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9'
}

// parseLicenseRef parses "LicenseRef-xxx" into a LicenseRef.
func parseLicenseRef(s string) *LicenseRef {
	// Remove "LicenseRef-" prefix (case insensitive)
	if hasPrefixFold(s, "LicenseRef-") {
		return &LicenseRef{LicenseRef: s[11:]}
	}
	return &LicenseRef{LicenseRef: s}
}

// parseDocumentRef parses "DocumentRef-xxx:LicenseRef-yyy" into a LicenseRef.
func parseDocumentRef(s string) *LicenseRef {
	// Format: DocumentRef-xxx:LicenseRef-yyy
	if hasPrefixFold(s, "DocumentRef-") {
		rest := s[12:] // after "DocumentRef-"
		if idx := indexFoldASCII(rest, ":LicenseRef-", 0); idx != -1 {
			docRef := rest[:idx]
			licRef := rest[idx+12:] // after ":LicenseRef-"
			return &LicenseRef{DocumentRef: docRef, LicenseRef: licRef}
		}
	}
	return &LicenseRef{LicenseRef: s}
}
