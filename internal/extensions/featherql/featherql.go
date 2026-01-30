// Package featherql provides a SQL-like DSL for declarative feature
// pipeline definitions with parsing, compilation, and execution.
package featherql

import (
	"fmt"
	"strings"
	"time"
)

// TokenType classifies lexer tokens.
type TokenType int

const (
	// TokenEOF marks end of input.
	TokenEOF TokenType = iota
	// TokenIdent is an identifier (column name, table name, etc.).
	TokenIdent
	// TokenNumber is a numeric literal.
	TokenNumber
	// TokenString is a string literal.
	TokenString
	// TokenDuration is a duration literal (e.g., "1h", "24h").
	TokenDuration
	// TokenKeyword is a reserved keyword.
	TokenKeyword
	// TokenOperator is an operator (+, -, *, /, =, <, >, etc.).
	TokenOperator
	// TokenPunctuation is punctuation (, ; ( ) etc.).
	TokenPunctuation
)

// Token is a single lexer token.
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "GROUP": true, "BY": true,
	"WINDOW": true, "SLIDE": true, "AS": true, "AND": true, "OR": true,
	"NOT": true, "HAVING": true, "ORDER": true, "LIMIT": true, "OFFSET": true,
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"FILTER": true, "JOIN": true, "ON": true, "CREATE": true, "FEATURE": true,
	"MATERIALIZED": true, "VIEW": true, "ENTITY": true, "EVERY": true,
	"INTO": true, "EMIT": true, "TUMBLING": true, "SLIDING": true, "OVER": true,
	"TUMBLE": true, "HOP": true, "SESSION": true, "STREAMING": true, "WATERMARK": true,
}

// Lexer tokenizes FeatherQL input.
type Lexer struct {
	input  string
	pos    int
	line   int
	col    int
	tokens []Token
}

// NewLexer creates a lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: input, line: 1, col: 1}
}

// Tokenize produces all tokens from the input.
func (l *Lexer) Tokenize() ([]Token, error) {
	for l.pos < len(l.input) {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}

		ch := l.input[l.pos]

		switch {
		case ch == '-' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '-':
			l.skipComment()
		case ch == '\'' || ch == '"':
			tok, err := l.readString(ch)
			if err != nil {
				return nil, err
			}
			l.tokens = append(l.tokens, tok)
		case isDigit(ch):
			l.tokens = append(l.tokens, l.readNumber())
		case isLetter(ch) || ch == '_':
			l.tokens = append(l.tokens, l.readIdentOrKeyword())
		case isOperator(ch):
			l.tokens = append(l.tokens, l.readOperator())
		case isPunctuation(ch):
			l.tokens = append(l.tokens, Token{Type: TokenPunctuation, Value: string(ch), Line: l.line, Column: l.col})
			l.advance()
		default:
			return nil, fmt.Errorf("unexpected character %q at line %d, column %d", ch, l.line, l.col)
		}
	}

	l.tokens = append(l.tokens, Token{Type: TokenEOF, Line: l.line, Column: l.col})
	return l.tokens, nil
}

func (l *Lexer) advance() {
	if l.pos < len(l.input) {
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t' || l.input[l.pos] == '\n' || l.input[l.pos] == '\r') {
		l.advance()
	}
}

func (l *Lexer) skipComment() {
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.advance()
	}
}

func (l *Lexer) readString(quote byte) (Token, error) {
	startLine, startCol := l.line, l.col
	l.advance() // skip opening quote
	start := l.pos

	for l.pos < len(l.input) && l.input[l.pos] != quote {
		l.advance()
	}
	if l.pos >= len(l.input) {
		return Token{}, fmt.Errorf("unterminated string at line %d, column %d", startLine, startCol)
	}
	value := l.input[start:l.pos]
	l.advance() // skip closing quote
	return Token{Type: TokenString, Value: value, Line: startLine, Column: startCol}, nil
}

func (l *Lexer) readNumber() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		l.advance()
	}

	// Check for duration suffix
	if l.pos < len(l.input) && (l.input[l.pos] == 'h' || l.input[l.pos] == 'm' || l.input[l.pos] == 's' || l.input[l.pos] == 'd') {
		l.advance()
		return Token{Type: TokenDuration, Value: l.input[start:l.pos], Line: startLine, Column: startCol}
	}
	return Token{Type: TokenNumber, Value: l.input[start:l.pos], Line: startLine, Column: startCol}
}

func (l *Lexer) readIdentOrKeyword() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.advance()
	}
	value := l.input[start:l.pos]

	if keywords[strings.ToUpper(value)] {
		return Token{Type: TokenKeyword, Value: strings.ToUpper(value), Line: startLine, Column: startCol}
	}
	return Token{Type: TokenIdent, Value: value, Line: startLine, Column: startCol}
}

func (l *Lexer) readOperator() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	l.advance()
	// Handle two-char operators
	if l.pos < len(l.input) {
		twoChar := l.input[start : l.pos+1]
		if twoChar == "<=" || twoChar == ">=" || twoChar == "!=" || twoChar == "<>" {
			l.advance()
			return Token{Type: TokenOperator, Value: twoChar, Line: startLine, Column: startCol}
		}
	}
	return Token{Type: TokenOperator, Value: l.input[start:l.pos], Line: startLine, Column: startCol}
}

func isDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func isLetter(ch byte) bool { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }
func isOperator(ch byte) bool {
	return ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '=' || ch == '<' || ch == '>' || ch == '!'
}
func isPunctuation(ch byte) bool {
	return ch == '(' || ch == ')' || ch == ',' || ch == ';' || ch == '.'
}

// NodeType classifies AST nodes.
type NodeType int

const (
	// NodeSelect represents a SELECT statement.
	NodeSelect NodeType = iota
	// NodeCreateFeature represents a CREATE FEATURE statement.
	NodeCreateFeature
	// NodeColumn represents a column reference.
	NodeColumn
	// NodeAggregation represents an aggregation function call.
	NodeAggregation
	// NodeWindow represents a window specification.
	NodeWindow
	// NodeFilter represents a WHERE clause filter.
	NodeFilter
	// NodeLiteral represents a literal value.
	NodeLiteral
	// NodeBinaryOp represents a binary operator expression.
	NodeBinaryOp
	// NodeAlias represents an aliased expression.
	NodeAlias
)

// ASTNode represents a node in the abstract syntax tree.
type ASTNode struct {
	Type     NodeType
	Value    string
	Children []*ASTNode
}

// Pipeline represents a compiled feature pipeline.
type Pipeline struct {
	Name       string        `json:"name"`
	EntityType string        `json:"entity_type"`
	Columns    []PipelineCol `json:"columns"`
	Filter     *FilterExpr   `json:"filter,omitempty"`
	Window     *WindowSpec   `json:"window,omitempty"`
	Schedule   time.Duration `json:"schedule,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

// PipelineCol defines a column in the pipeline output.
type PipelineCol struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
	AggFunc    string `json:"agg_func,omitempty"`
	DataType   string `json:"data_type,omitempty"`
}

// FilterExpr represents a filter condition.
type FilterExpr struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// WindowSpec defines a time window for aggregation.
type WindowSpec struct {
	Type      string        `json:"type"` // tumbling, sliding, session, tumble, hop
	Duration  time.Duration `json:"duration"`
	SlideBy   time.Duration `json:"slide_by,omitempty"`
	Gap       time.Duration `json:"gap,omitempty"`       // session window gap
	MaxLate   time.Duration `json:"max_late,omitempty"`  // watermark lateness
}

// Parser parses FeatherQL tokens into an AST.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a parser from tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse parses a complete FeatherQL statement.
func (p *Parser) Parse() (*ASTNode, error) {
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type == TokenEOF {
		return nil, fmt.Errorf("empty query")
	}

	tok := p.tokens[p.pos]
	switch {
	case tok.Type == TokenKeyword && tok.Value == "SELECT":
		return p.parseSelect()
	case tok.Type == TokenKeyword && tok.Value == "CREATE":
		return p.parseCreateFeature()
	default:
		return nil, fmt.Errorf("unexpected token %q at line %d", tok.Value, tok.Line)
	}
}

func (p *Parser) parseSelect() (*ASTNode, error) {
	p.pos++ // consume SELECT
	node := &ASTNode{Type: NodeSelect}

	// Parse column list
	for {
		col, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("parsing column: %w", err)
		}

		// Check for alias
		if p.peek().Type == TokenKeyword && p.peek().Value == "AS" {
			p.pos++ // consume AS
			alias := p.expect(TokenIdent)
			if alias == nil {
				return nil, fmt.Errorf("expected alias name")
			}
			col = &ASTNode{Type: NodeAlias, Value: alias.Value, Children: []*ASTNode{col}}
		}

		node.Children = append(node.Children, col)

		if p.peek().Type != TokenPunctuation || p.peek().Value != "," {
			break
		}
		p.pos++ // consume comma
	}

	// Parse FROM
	if p.peek().Type == TokenKeyword && p.peek().Value == "FROM" {
		p.pos++
		source := p.expect(TokenIdent)
		if source != nil {
			node.Value = source.Value
		}
	}

	// Parse WHERE
	if p.peek().Type == TokenKeyword && p.peek().Value == "WHERE" {
		p.pos++
		filter, err := p.parseFilterExpr()
		if err != nil {
			return nil, fmt.Errorf("parsing WHERE: %w", err)
		}
		node.Children = append(node.Children, filter)
	}

	// Parse WINDOW
	if p.peek().Type == TokenKeyword && p.peek().Value == "WINDOW" {
		p.pos++
		window, err := p.parseWindowSpec()
		if err != nil {
			return nil, fmt.Errorf("parsing WINDOW: %w", err)
		}
		node.Children = append(node.Children, window)
	}

	return node, nil
}

func (p *Parser) parseCreateFeature() (*ASTNode, error) {
	p.pos++ // consume CREATE
	if p.peek().Type != TokenKeyword || p.peek().Value != "FEATURE" {
		return nil, fmt.Errorf("expected FEATURE after CREATE")
	}
	p.pos++ // consume FEATURE

	name := p.expect(TokenIdent)
	if name == nil {
		return nil, fmt.Errorf("expected feature name")
	}

	node := &ASTNode{Type: NodeCreateFeature, Value: name.Value}

	// Parse AS SELECT ...
	if p.peek().Type == TokenKeyword && p.peek().Value == "AS" {
		p.pos++
		selectNode, err := p.parseSelect()
		if err != nil {
			return nil, fmt.Errorf("parsing feature definition: %w", err)
		}
		node.Children = append(node.Children, selectNode)
	}

	return node, nil
}

func (p *Parser) parseExpression() (*ASTNode, error) {
	tok := p.peek()

	// Aggregation function
	if tok.Type == TokenKeyword && isAggFunction(tok.Value) {
		return p.parseAggregation()
	}

	// Column reference or literal
	if tok.Type == TokenIdent {
		p.pos++
		return &ASTNode{Type: NodeColumn, Value: tok.Value}, nil
	}

	if tok.Type == TokenNumber || tok.Type == TokenString {
		p.pos++
		return &ASTNode{Type: NodeLiteral, Value: tok.Value}, nil
	}

	return nil, fmt.Errorf("unexpected token %q at line %d", tok.Value, tok.Line)
}

func (p *Parser) parseAggregation() (*ASTNode, error) {
	funcTok := p.peek()
	p.pos++ // consume function name

	if p.peek().Type != TokenPunctuation || p.peek().Value != "(" {
		return nil, fmt.Errorf("expected '(' after %s", funcTok.Value)
	}
	p.pos++ // consume (

	inner, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("parsing aggregation argument: %w", err)
	}

	if p.peek().Type != TokenPunctuation || p.peek().Value != ")" {
		return nil, fmt.Errorf("expected ')' after aggregation argument")
	}
	p.pos++ // consume )

	return &ASTNode{
		Type:     NodeAggregation,
		Value:    funcTok.Value,
		Children: []*ASTNode{inner},
	}, nil
}

func (p *Parser) parseFilterExpr() (*ASTNode, error) {
	left, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if p.peek().Type != TokenOperator {
		return nil, fmt.Errorf("expected operator in filter expression")
	}
	op := p.peek().Value
	p.pos++

	right, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	return &ASTNode{
		Type:     NodeFilter,
		Value:    op,
		Children: []*ASTNode{left, right},
	}, nil
}

func (p *Parser) parseWindowSpec() (*ASTNode, error) {
	node := &ASTNode{Type: NodeWindow}

	// Window type: TUMBLING, SLIDING, TUMBLE, HOP, SESSION
	tok := p.peek()
	if tok.Type == TokenKeyword {
		switch tok.Value {
		case "TUMBLING", "TUMBLE":
			node.Value = "TUMBLE"
			p.pos++
		case "SLIDING":
			node.Value = "SLIDING"
			p.pos++
		case "HOP":
			node.Value = "HOP"
			p.pos++
		case "SESSION":
			node.Value = "SESSION"
			p.pos++
		default:
			node.Value = "TUMBLE"
		}
	} else {
		node.Value = "TUMBLE"
	}

	// Check for function-call syntax: TUMBLE(source, duration) or HOP(source, size, slide)
	if p.peek().Type == TokenPunctuation && p.peek().Value == "(" {
		p.pos++ // consume (

		// First arg (source or duration)
		arg1, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("parsing window argument: %w", err)
		}
		node.Children = append(node.Children, arg1)

		// Additional args separated by commas
		for p.peek().Type == TokenPunctuation && p.peek().Value == "," {
			p.pos++ // consume ,
			arg, err := p.parseExpression()
			if err != nil {
				return nil, fmt.Errorf("parsing window argument: %w", err)
			}
			node.Children = append(node.Children, arg)
		}

		if p.peek().Type != TokenPunctuation || p.peek().Value != ")" {
			return nil, fmt.Errorf("expected ')' in window specification")
		}
		p.pos++ // consume )
		return node, nil
	}

	// Legacy syntax: WINDOW TUMBLING 1h SLIDE BY 5m
	if p.peek().Type == TokenDuration {
		node.Children = append(node.Children, &ASTNode{Type: NodeLiteral, Value: p.peek().Value})
		p.pos++
	} else if p.peek().Type == TokenNumber {
		node.Children = append(node.Children, &ASTNode{Type: NodeLiteral, Value: p.peek().Value})
		p.pos++
	}

	// SLIDE BY (for HOP/SLIDING windows)
	if p.peek().Type == TokenKeyword && p.peek().Value == "SLIDE" {
		p.pos++
		if p.peek().Type == TokenKeyword && p.peek().Value == "BY" {
			p.pos++
		}
		if p.peek().Type == TokenDuration || p.peek().Type == TokenNumber {
			node.Children = append(node.Children, &ASTNode{Type: NodeLiteral, Value: p.peek().Value})
			p.pos++
		}
	}

	// WATERMARK clause
	if p.peek().Type == TokenKeyword && p.peek().Value == "WATERMARK" {
		p.pos++
		if p.peek().Type == TokenDuration || p.peek().Type == TokenNumber {
			node.Children = append(node.Children, &ASTNode{Type: NodeLiteral, Value: "watermark:" + p.peek().Value})
			p.pos++
		}
	}

	return node, nil
}

func (p *Parser) peek() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

func (p *Parser) expect(tokenType TokenType) *Token {
	if p.pos < len(p.tokens) && p.tokens[p.pos].Type == tokenType {
		tok := p.tokens[p.pos]
		p.pos++
		return &tok
	}
	return nil
}

func isAggFunction(s string) bool {
	switch s {
	case "COUNT", "SUM", "AVG", "MIN", "MAX":
		return true
	}
	return false
}

// Compiler compiles an AST into an executable pipeline.
type Compiler struct{}

// NewCompiler creates a new compiler.
func NewCompiler() *Compiler {
	return &Compiler{}
}

// Compile transforms an AST into a Pipeline definition.
func (c *Compiler) Compile(ast *ASTNode) (*Pipeline, error) {
	if ast == nil {
		return nil, fmt.Errorf("nil AST")
	}

	switch ast.Type {
	case NodeCreateFeature:
		return c.compileCreateFeature(ast)
	case NodeSelect:
		return c.compileSelect(ast)
	default:
		return nil, fmt.Errorf("unsupported top-level node type: %d", ast.Type)
	}
}

func (c *Compiler) compileCreateFeature(ast *ASTNode) (*Pipeline, error) {
	pipeline := &Pipeline{
		Name:      ast.Value,
		CreatedAt: time.Now(),
	}

	if len(ast.Children) > 0 && ast.Children[0].Type == NodeSelect {
		selectPipeline, err := c.compileSelect(ast.Children[0])
		if err != nil {
			return nil, fmt.Errorf("compiling feature select: %w", err)
		}
		pipeline.EntityType = selectPipeline.EntityType
		pipeline.Columns = selectPipeline.Columns
		pipeline.Filter = selectPipeline.Filter
		pipeline.Window = selectPipeline.Window
	}

	return pipeline, nil
}

func (c *Compiler) compileSelect(ast *ASTNode) (*Pipeline, error) {
	pipeline := &Pipeline{
		EntityType: ast.Value,
		CreatedAt:  time.Now(),
	}

	for _, child := range ast.Children {
		switch child.Type {
		case NodeColumn:
			pipeline.Columns = append(pipeline.Columns, PipelineCol{
				Name:       child.Value,
				Expression: child.Value,
			})
		case NodeAggregation:
			colName := child.Value
			if len(child.Children) > 0 {
				colName = child.Value + "_" + child.Children[0].Value
			}
			pipeline.Columns = append(pipeline.Columns, PipelineCol{
				Name:       colName,
				Expression: child.Value + "(" + child.Children[0].Value + ")",
				AggFunc:    child.Value,
			})
		case NodeAlias:
			col := PipelineCol{Name: child.Value}
			if len(child.Children) > 0 {
				inner := child.Children[0]
				if inner.Type == NodeAggregation && len(inner.Children) > 0 {
					col.Expression = inner.Value + "(" + inner.Children[0].Value + ")"
					col.AggFunc = inner.Value
				} else {
					col.Expression = inner.Value
				}
			}
			pipeline.Columns = append(pipeline.Columns, col)
		case NodeFilter:
			if len(child.Children) >= 2 {
				pipeline.Filter = &FilterExpr{
					Field:    child.Children[0].Value,
					Operator: child.Value,
					Value:    child.Children[1].Value,
				}
			}
		case NodeWindow:
			ws := &WindowSpec{Type: strings.ToLower(child.Value)}
			for _, wChild := range child.Children {
				val := wChild.Value
				if strings.HasPrefix(val, "watermark:") {
					d, _ := parseDuration(strings.TrimPrefix(val, "watermark:"))
					ws.MaxLate = d
				} else if ws.Duration == 0 {
					d, _ := parseDuration(val)
					ws.Duration = d
				} else if ws.Type == "hop" || ws.Type == "sliding" {
					d, _ := parseDuration(val)
					ws.SlideBy = d
				} else if ws.Type == "session" {
					d, _ := parseDuration(val)
					ws.Gap = d
				}
			}
			if ws.Type == "session" && ws.Gap == 0 && ws.Duration > 0 {
				ws.Gap = ws.Duration
			}
			pipeline.Window = ws
		}
	}

	return pipeline, nil
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}
	last := s[len(s)-1]
	switch last {
	case 'd':
		// Go's time.ParseDuration doesn't support 'd', convert to hours
		s = s[:len(s)-1] + "h"
		d, err := time.ParseDuration(s)
		return d * 24, err
	default:
		return time.ParseDuration(s)
	}
}
