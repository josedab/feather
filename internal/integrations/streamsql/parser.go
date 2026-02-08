package streamsql

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// TokenType represents the type of a SQL token.
type TokenType int

const (
	TokenSelect     TokenType = iota
	TokenFrom
	TokenWhere
	TokenGroupBy
	TokenHaving
	TokenWindow
	TokenTumble
	TokenSlide
	TokenAs
	TokenAnd
	TokenOr
	TokenNot
	TokenIdentifier
	TokenNumber
	TokenString
	TokenStar
	TokenComma
	TokenDot
	TokenLParen
	TokenRParen
	TokenOperator
	TokenEOF
	TokenJoin
	TokenOn
	TokenOrderBy
	TokenLimit
	TokenEmit
)

// StatementType represents the type of a SQL statement.
type StatementType string

const (
	StatementSelect StatementType = "SELECT"
)

// EmitMode controls when query results are emitted.
type EmitMode string

const (
	EmitOnChange EmitMode = "on_change"
	EmitOnWindow EmitMode = "on_window"
)

// WindowType represents the type of a window function.
type WindowType string

const (
	WindowTumbling WindowType = "tumbling"
	WindowSliding  WindowType = "sliding"
	WindowSession  WindowType = "session"
)

// Token represents a lexed SQL token.
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}

// Lexer tokenizes a SQL input string.
type Lexer struct {
	input  string
	pos    int
	tokens []Token
}

// NewLexer creates a new Lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{
		input:  input,
		pos:    0,
		tokens: nil,
	}
}

// Tokenize scans the input and returns a slice of tokens.
func (l *Lexer) Tokenize() ([]Token, error) {
	l.tokens = nil
	for l.pos < len(l.input) {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}

		ch := l.input[l.pos]

		switch {
		case ch == '\'':
			tok, err := l.readString()
			if err != nil {
				return nil, err
			}
			l.tokens = append(l.tokens, tok)
		case ch == '(':
			l.tokens = append(l.tokens, Token{Type: TokenLParen, Value: "(", Pos: l.pos})
			l.pos++
		case ch == ')':
			l.tokens = append(l.tokens, Token{Type: TokenRParen, Value: ")", Pos: l.pos})
			l.pos++
		case ch == '*':
			l.tokens = append(l.tokens, Token{Type: TokenStar, Value: "*", Pos: l.pos})
			l.pos++
		case ch == ',':
			l.tokens = append(l.tokens, Token{Type: TokenComma, Value: ",", Pos: l.pos})
			l.pos++
		case ch == '.':
			l.tokens = append(l.tokens, Token{Type: TokenDot, Value: ".", Pos: l.pos})
			l.pos++
		case isOperatorChar(ch):
			tok := l.readOperator()
			l.tokens = append(l.tokens, tok)
		case isDigit(ch):
			tok := l.readNumber()
			l.tokens = append(l.tokens, tok)
		case isIdentStart(ch):
			tok := l.readIdentifierOrKeyword()
			l.tokens = append(l.tokens, tok)
		default:
			return nil, fmt.Errorf("tokenizing at position %d: unexpected character %q", l.pos, ch)
		}
	}
	l.tokens = append(l.tokens, Token{Type: TokenEOF, Value: "", Pos: l.pos})
	return l.tokens, nil
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *Lexer) readString() (Token, error) {
	start := l.pos
	l.pos++ // skip opening quote
	for l.pos < len(l.input) {
		if l.input[l.pos] == '\'' {
			val := l.input[start+1 : l.pos]
			l.pos++ // skip closing quote
			return Token{Type: TokenString, Value: val, Pos: start}, nil
		}
		l.pos++
	}
	return Token{}, fmt.Errorf("tokenizing string at position %d: unterminated string literal", start)
}

func (l *Lexer) readNumber() Token {
	start := l.pos
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		l.pos++
	}
	return Token{Type: TokenNumber, Value: l.input[start:l.pos], Pos: start}
}

func (l *Lexer) readOperator() Token {
	start := l.pos
	// Handle two-character operators
	if l.pos+1 < len(l.input) {
		two := l.input[l.pos : l.pos+2]
		if two == "<=" || two == ">=" || two == "!=" || two == "<>" {
			l.pos += 2
			return Token{Type: TokenOperator, Value: two, Pos: start}
		}
	}
	l.pos++
	return Token{Type: TokenOperator, Value: l.input[start:l.pos], Pos: start}
}

func (l *Lexer) readIdentifierOrKeyword() Token {
	start := l.pos
	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.pos++
	}
	word := l.input[start:l.pos]
	upper := strings.ToUpper(word)

	// Check for two-word keywords: GROUP BY, ORDER BY
	if upper == "GROUP" || upper == "ORDER" {
		saved := l.pos
		l.skipWhitespace()
		if l.pos < len(l.input) {
			nextStart := l.pos
			for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
				l.pos++
			}
			nextWord := strings.ToUpper(l.input[nextStart:l.pos])
			if nextWord == "BY" {
				if upper == "GROUP" {
					return Token{Type: TokenGroupBy, Value: "GROUP BY", Pos: start}
				}
				return Token{Type: TokenOrderBy, Value: "ORDER BY", Pos: start}
			}
			l.pos = saved
		} else {
			l.pos = saved
		}
	}

	tokType := keywordType(upper)
	return Token{Type: tokType, Value: word, Pos: start}
}

func keywordType(upper string) TokenType {
	switch upper {
	case "SELECT":
		return TokenSelect
	case "FROM":
		return TokenFrom
	case "WHERE":
		return TokenWhere
	case "HAVING":
		return TokenHaving
	case "WINDOW":
		return TokenWindow
	case "TUMBLE":
		return TokenTumble
	case "SLIDE":
		return TokenSlide
	case "AS":
		return TokenAs
	case "AND":
		return TokenAnd
	case "OR":
		return TokenOr
	case "NOT":
		return TokenNot
	case "JOIN":
		return TokenJoin
	case "ON":
		return TokenOn
	case "LIMIT":
		return TokenLimit
	case "EMIT":
		return TokenEmit
	default:
		return TokenIdentifier
	}
}

func isDigit(ch byte) bool       { return ch >= '0' && ch <= '9' }
func isIdentStart(ch byte) bool  { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' }
func isIdentChar(ch byte) bool   { return isIdentStart(ch) || isDigit(ch) }
func isOperatorChar(ch byte) bool { return ch == '=' || ch == '<' || ch == '>' || ch == '!' }

// Statement represents a parsed SQL statement.
type Statement struct {
	Type     StatementType  `json:"type"`
	Select   []*SelectExpr  `json:"select"`
	From     *FromClause    `json:"from"`
	Where    *WhereClause   `json:"where,omitempty"`
	GroupBy  []string       `json:"group_by,omitempty"`
	Having   *WhereClause   `json:"having,omitempty"`
	Window   *WindowClause  `json:"window,omitempty"`
	OrderBy  []*OrderByExpr `json:"order_by,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	EmitMode EmitMode       `json:"emit_mode,omitempty"`
}

// SelectExpr represents a single expression in a SELECT clause.
type SelectExpr struct {
	Expression string   `json:"expression"`
	Alias      string   `json:"alias,omitempty"`
	Function   string   `json:"function,omitempty"`
	Args       []string `json:"args,omitempty"`
}

// FromClause represents the FROM clause.
type FromClause struct {
	Stream string      `json:"stream"`
	Alias  string      `json:"alias,omitempty"`
	Join   *JoinClause `json:"join,omitempty"`
}

// JoinClause represents a JOIN between two streams.
type JoinClause struct {
	Stream    string `json:"stream"`
	Alias     string `json:"alias,omitempty"`
	Condition string `json:"condition"`
}

// WhereClause represents a WHERE or HAVING clause.
type WhereClause struct {
	Conditions []*Condition `json:"conditions"`
	Logic      string       `json:"logic,omitempty"` // "AND" or "OR"
}

// Condition represents a single filter condition.
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// WindowClause represents a WINDOW clause for streaming aggregations.
type WindowClause struct {
	Type  WindowType    `json:"type"`
	Size  time.Duration `json:"size"`
	Slide time.Duration `json:"slide,omitempty"`
	Field string        `json:"field"`
}

// OrderByExpr represents an ORDER BY expression.
type OrderByExpr struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// Parser parses a slice of tokens into a Statement.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a new Parser for the given tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// Parse parses tokens into a Statement.
func (p *Parser) Parse() (*Statement, error) {
	stmt := &Statement{Type: StatementSelect}

	if err := p.expect(TokenSelect); err != nil {
		return nil, fmt.Errorf("parsing statement: %w", err)
	}

	selectExprs, err := p.parseSelectList()
	if err != nil {
		return nil, fmt.Errorf("parsing select list: %w", err)
	}
	stmt.Select = selectExprs

	if err := p.expect(TokenFrom); err != nil {
		return nil, fmt.Errorf("parsing FROM clause: %w", err)
	}

	from, err := p.parseFrom()
	if err != nil {
		return nil, fmt.Errorf("parsing FROM clause: %w", err)
	}
	stmt.From = from

	// Parse optional clauses
	for p.current().Type != TokenEOF {
		switch p.current().Type {
		case TokenWhere:
			p.advance()
			where, err := p.parseConditions()
			if err != nil {
				return nil, fmt.Errorf("parsing WHERE clause: %w", err)
			}
			stmt.Where = where
		case TokenGroupBy:
			p.advance()
			groupBy, err := p.parseGroupBy()
			if err != nil {
				return nil, fmt.Errorf("parsing GROUP BY clause: %w", err)
			}
			stmt.GroupBy = groupBy
		case TokenHaving:
			p.advance()
			having, err := p.parseConditions()
			if err != nil {
				return nil, fmt.Errorf("parsing HAVING clause: %w", err)
			}
			stmt.Having = having
		case TokenWindow:
			p.advance()
			window, err := p.parseWindow()
			if err != nil {
				return nil, fmt.Errorf("parsing WINDOW clause: %w", err)
			}
			stmt.Window = window
		case TokenOrderBy:
			p.advance()
			orderBy, err := p.parseOrderBy()
			if err != nil {
				return nil, fmt.Errorf("parsing ORDER BY clause: %w", err)
			}
			stmt.OrderBy = orderBy
		case TokenLimit:
			p.advance()
			limit, err := p.parseLimit()
			if err != nil {
				return nil, fmt.Errorf("parsing LIMIT clause: %w", err)
			}
			stmt.Limit = limit
		case TokenEmit:
			p.advance()
			mode, err := p.parseEmit()
			if err != nil {
				return nil, fmt.Errorf("parsing EMIT clause: %w", err)
			}
			stmt.EmitMode = mode
		default:
			return nil, fmt.Errorf("parsing statement: unexpected token %q at position %d", p.current().Value, p.current().Pos)
		}
	}

	return stmt, nil
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(tt TokenType) error {
	if p.current().Type != tt {
		return fmt.Errorf("expected token type %d, got %q at position %d", tt, p.current().Value, p.current().Pos)
	}
	p.advance()
	return nil
}

func (p *Parser) parseSelectList() ([]*SelectExpr, error) {
	var exprs []*SelectExpr
	for {
		expr, err := p.parseSelectExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		if p.current().Type != TokenComma {
			break
		}
		p.advance() // skip comma
	}
	return exprs, nil
}

func (p *Parser) parseSelectExpr() (*SelectExpr, error) {
	if p.current().Type == TokenStar {
		p.advance()
		return &SelectExpr{Expression: "*"}, nil
	}

	tok := p.current()
	if tok.Type != TokenIdentifier {
		return nil, fmt.Errorf("parsing select expression: expected identifier, got %q", tok.Value)
	}

	name := strings.ToUpper(tok.Value)
	// Check if this is an aggregate function
	if isAggFunc(name) && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == TokenLParen {
		return p.parseAggregateExpr()
	}

	p.advance()
	expr := &SelectExpr{Expression: tok.Value}

	// Check for alias
	if p.current().Type == TokenAs {
		p.advance()
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("parsing alias: expected identifier, got %q", p.current().Value)
		}
		expr.Alias = p.current().Value
		p.advance()
	}

	return expr, nil
}

func (p *Parser) parseAggregateExpr() (*SelectExpr, error) {
	funcName := strings.ToUpper(p.current().Value)
	p.advance() // skip function name
	if err := p.expect(TokenLParen); err != nil {
		return nil, fmt.Errorf("parsing aggregate function: %w", err)
	}

	var args []string
	if p.current().Type == TokenStar {
		args = append(args, "*")
		p.advance()
	} else {
		for {
			if p.current().Type != TokenIdentifier && p.current().Type != TokenNumber {
				return nil, fmt.Errorf("parsing aggregate args: expected identifier or number, got %q", p.current().Value)
			}
			args = append(args, p.current().Value)
			p.advance()
			if p.current().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("parsing aggregate function closing paren: %w", err)
	}

	expr := &SelectExpr{
		Expression: fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", ")),
		Function:   funcName,
		Args:       args,
	}

	if p.current().Type == TokenAs {
		p.advance()
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("parsing alias: expected identifier, got %q", p.current().Value)
		}
		expr.Alias = p.current().Value
		p.advance()
	}

	return expr, nil
}

func (p *Parser) parseFrom() (*FromClause, error) {
	if p.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("expected stream name, got %q", p.current().Value)
	}
	from := &FromClause{Stream: p.current().Value}
	p.advance()

	// Optional alias
	if p.current().Type == TokenAs {
		p.advance()
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected alias, got %q", p.current().Value)
		}
		from.Alias = p.current().Value
		p.advance()
	} else if p.current().Type == TokenIdentifier && !isClauseStart(p.current()) {
		from.Alias = p.current().Value
		p.advance()
	}

	// Optional JOIN
	if p.current().Type == TokenJoin {
		p.advance()
		join, err := p.parseJoin()
		if err != nil {
			return nil, fmt.Errorf("parsing JOIN: %w", err)
		}
		from.Join = join
	}

	return from, nil
}

func (p *Parser) parseJoin() (*JoinClause, error) {
	if p.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("expected stream name for JOIN, got %q", p.current().Value)
	}
	join := &JoinClause{Stream: p.current().Value}
	p.advance()

	// Optional alias
	if p.current().Type == TokenAs {
		p.advance()
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("expected alias, got %q", p.current().Value)
		}
		join.Alias = p.current().Value
		p.advance()
	}

	if err := p.expect(TokenOn); err != nil {
		return nil, fmt.Errorf("parsing JOIN ON: %w", err)
	}

	// Read join condition as raw text until a clause keyword
	var parts []string
	for p.current().Type != TokenEOF && !isClauseStart(p.current()) {
		parts = append(parts, p.current().Value)
		p.advance()
	}
	join.Condition = strings.Join(parts, " ")

	return join, nil
}

func (p *Parser) parseConditions() (*WhereClause, error) {
	where := &WhereClause{Logic: "AND"}
	cond, err := p.parseSingleCondition()
	if err != nil {
		return nil, err
	}
	where.Conditions = append(where.Conditions, cond)

	for p.current().Type == TokenAnd || p.current().Type == TokenOr {
		if p.current().Type == TokenOr {
			where.Logic = "OR"
		}
		p.advance()
		cond, err := p.parseSingleCondition()
		if err != nil {
			return nil, err
		}
		where.Conditions = append(where.Conditions, cond)
	}

	return where, nil
}

func (p *Parser) parseSingleCondition() (*Condition, error) {
	if p.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("parsing condition: expected field name, got %q", p.current().Value)
	}
	field := p.current().Value
	p.advance()

	if p.current().Type != TokenOperator {
		return nil, fmt.Errorf("parsing condition: expected operator, got %q", p.current().Value)
	}
	op := p.current().Value
	p.advance()

	var value interface{}
	switch p.current().Type {
	case TokenNumber:
		value = parseNumber(p.current().Value)
	case TokenString:
		value = p.current().Value
	case TokenIdentifier:
		value = p.current().Value
	default:
		return nil, fmt.Errorf("parsing condition value: unexpected token %q", p.current().Value)
	}
	p.advance()

	return &Condition{Field: field, Operator: op, Value: value}, nil
}

func (p *Parser) parseGroupBy() ([]string, error) {
	var fields []string
	for {
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("parsing GROUP BY: expected identifier, got %q", p.current().Value)
		}
		fields = append(fields, p.current().Value)
		p.advance()
		if p.current().Type != TokenComma {
			break
		}
		p.advance()
	}
	return fields, nil
}

func (p *Parser) parseWindow() (*WindowClause, error) {
	wc := &WindowClause{}

	switch p.current().Type {
	case TokenTumble:
		wc.Type = WindowTumbling
		p.advance()
	case TokenSlide:
		wc.Type = WindowSliding
		p.advance()
	default:
		return nil, fmt.Errorf("parsing WINDOW: expected TUMBLE or SLIDE, got %q", p.current().Value)
	}

	if err := p.expect(TokenLParen); err != nil {
		return nil, fmt.Errorf("parsing WINDOW function: %w", err)
	}

	// Parse field name
	if p.current().Type != TokenIdentifier {
		return nil, fmt.Errorf("parsing WINDOW field: expected identifier, got %q", p.current().Value)
	}
	wc.Field = p.current().Value
	p.advance()

	if p.current().Type != TokenComma {
		return nil, fmt.Errorf("parsing WINDOW: expected comma after field, got %q", p.current().Value)
	}
	p.advance()

	// Parse window size
	if p.current().Type != TokenString {
		return nil, fmt.Errorf("parsing WINDOW size: expected duration string, got %q", p.current().Value)
	}
	dur, err := time.ParseDuration(p.current().Value)
	if err != nil {
		return nil, fmt.Errorf("parsing WINDOW size duration: %w", err)
	}
	wc.Size = dur
	p.advance()

	// Optional slide interval
	if p.current().Type == TokenComma {
		p.advance()
		if p.current().Type != TokenString {
			return nil, fmt.Errorf("parsing WINDOW slide: expected duration string, got %q", p.current().Value)
		}
		slide, err := time.ParseDuration(p.current().Value)
		if err != nil {
			return nil, fmt.Errorf("parsing WINDOW slide duration: %w", err)
		}
		wc.Slide = slide
		p.advance()
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, fmt.Errorf("parsing WINDOW closing paren: %w", err)
	}

	return wc, nil
}

func (p *Parser) parseOrderBy() ([]*OrderByExpr, error) {
	var exprs []*OrderByExpr
	for {
		if p.current().Type != TokenIdentifier {
			return nil, fmt.Errorf("parsing ORDER BY: expected identifier, got %q", p.current().Value)
		}
		expr := &OrderByExpr{Field: p.current().Value}
		p.advance()

		if p.current().Type == TokenIdentifier {
			upper := strings.ToUpper(p.current().Value)
			if upper == "DESC" {
				expr.Desc = true
				p.advance()
			} else if upper == "ASC" {
				p.advance()
			}
		}
		exprs = append(exprs, expr)
		if p.current().Type != TokenComma {
			break
		}
		p.advance()
	}
	return exprs, nil
}

func (p *Parser) parseLimit() (int, error) {
	if p.current().Type != TokenNumber {
		return 0, fmt.Errorf("parsing LIMIT: expected number, got %q", p.current().Value)
	}
	n := parseNumber(p.current().Value)
	p.advance()
	intVal, ok := n.(int)
	if !ok {
		return 0, fmt.Errorf("parsing LIMIT: expected integer value")
	}
	return intVal, nil
}

func (p *Parser) parseEmit() (EmitMode, error) {
	if p.current().Type != TokenIdentifier {
		return "", fmt.Errorf("parsing EMIT: expected mode, got %q", p.current().Value)
	}
	mode := strings.ToLower(p.current().Value)
	p.advance()
	switch mode {
	case "changes":
		return EmitOnChange, nil
	case "window":
		return EmitOnWindow, nil
	default:
		return "", fmt.Errorf("parsing EMIT: unknown mode %q", mode)
	}
}

func isAggFunc(name string) bool {
	switch name {
	case "COUNT", "SUM", "AVG", "MIN", "MAX", "STDDEV":
		return true
	}
	return false
}

func isClauseStart(tok Token) bool {
	switch tok.Type {
	case TokenWhere, TokenGroupBy, TokenHaving, TokenWindow, TokenOrderBy, TokenLimit, TokenEmit, TokenJoin, TokenOn:
		return true
	}
	return false
}

func parseNumber(s string) interface{} {
	// Try integer first
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		if fmt.Sprintf("%d", n) == s {
			return n
		}
	}
	// Fall back to float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return s
}
