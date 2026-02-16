package featherqlv2

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType classifies lexer tokens.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenKeyword        // SELECT, FROM, WHERE, GROUP, BY, AS, OVER, PARTITION, WINDOW, ORDER, LIMIT, JOIN, ON, AND, OR, NOT, IN, HAVING
	TokenIdent          // column names, table names
	TokenNumber         // integer or float literals
	TokenString         // 'quoted string'
	TokenComma          // ,
	TokenDot            // .
	TokenStar           // *
	TokenLParen         // (
	TokenRParen         // )
	TokenOp             // =, <, >, <=, >=, !=, <>
	TokenPlus           // +
	TokenMinus          // -
)

// Token represents a single lexer token.
type Token struct {
	Type    TokenType
	Value   string
	Pos     int
}

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "GROUP": true,
	"BY": true, "AS": true, "OVER": true, "PARTITION": true,
	"WINDOW": true, "ORDER": true, "LIMIT": true, "JOIN": true,
	"ON": true, "AND": true, "OR": true, "NOT": true, "IN": true,
	"HAVING": true, "LEFT": true, "RIGHT": true, "INNER": true,
	"OUTER": true, "CROSS": true, "BETWEEN": true, "LIKE": true,
	"IS": true, "NULL": true, "ASC": true, "DESC": true,
}

// Tokenize splits a FeatherQL query into tokens.
func Tokenize(input string) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(input) {
		ch := input[i]

		// Skip whitespace
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}

		// Single-character tokens
		switch ch {
		case ',':
			tokens = append(tokens, Token{TokenComma, ",", i})
			i++
			continue
		case '.':
			tokens = append(tokens, Token{TokenDot, ".", i})
			i++
			continue
		case '*':
			tokens = append(tokens, Token{TokenStar, "*", i})
			i++
			continue
		case '(':
			tokens = append(tokens, Token{TokenLParen, "(", i})
			i++
			continue
		case ')':
			tokens = append(tokens, Token{TokenRParen, ")", i})
			i++
			continue
		case '+':
			tokens = append(tokens, Token{TokenPlus, "+", i})
			i++
			continue
		case '-':
			// Could be minus or negative number
			if i+1 < len(input) && unicode.IsDigit(rune(input[i+1])) {
				num, end := scanNumber(input, i)
				tokens = append(tokens, Token{TokenNumber, num, i})
				i = end
				continue
			}
			tokens = append(tokens, Token{TokenMinus, "-", i})
			i++
			continue
		}

		// Operators
		if ch == '=' || ch == '<' || ch == '>' || ch == '!' {
			op, end := scanOperator(input, i)
			tokens = append(tokens, Token{TokenOp, op, i})
			i = end
			continue
		}

		// String literals
		if ch == '\'' {
			str, end, err := scanString(input, i)
			if err != nil {
				return nil, fmt.Errorf("tokenizer error at position %d: %w", i, err)
			}
			tokens = append(tokens, Token{TokenString, str, i})
			i = end
			continue
		}

		// Numbers
		if unicode.IsDigit(rune(ch)) {
			num, end := scanNumber(input, i)
			tokens = append(tokens, Token{TokenNumber, num, i})
			i = end
			continue
		}

		// Identifiers and keywords
		if unicode.IsLetter(rune(ch)) || ch == '_' {
			ident, end := scanIdent(input, i)
			upper := strings.ToUpper(ident)
			if keywords[upper] {
				tokens = append(tokens, Token{TokenKeyword, upper, i})
			} else {
				tokens = append(tokens, Token{TokenIdent, ident, i})
			}
			i = end
			continue
		}

		return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, i)
	}

	tokens = append(tokens, Token{TokenEOF, "", len(input)})
	return tokens, nil
}

func scanNumber(input string, start int) (string, int) {
	i := start
	if i < len(input) && input[i] == '-' {
		i++
	}
	for i < len(input) && (unicode.IsDigit(rune(input[i])) || input[i] == '.') {
		i++
	}
	return input[start:i], i
}

func scanString(input string, start int) (string, int, error) {
	i := start + 1 // skip opening quote
	for i < len(input) {
		if input[i] == '\'' {
			if i+1 < len(input) && input[i+1] == '\'' {
				i += 2 // escaped quote
				continue
			}
			return input[start+1 : i], i + 1, nil
		}
		i++
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

func scanIdent(input string, start int) (string, int) {
	i := start
	for i < len(input) && (unicode.IsLetter(rune(input[i])) || unicode.IsDigit(rune(input[i])) || input[i] == '_') {
		i++
	}
	return input[start:i], i
}

func scanOperator(input string, start int) (string, int) {
	if start+1 < len(input) {
		two := input[start : start+2]
		if two == "<=" || two == ">=" || two == "!=" || two == "<>" {
			return two, start + 2
		}
	}
	return string(input[start]), start + 1
}

// SelectStatement represents a parsed SELECT query.
type SelectStatement struct {
	Columns   []SelectColumn   `json:"columns"`
	From      string           `json:"from"`
	Joins     []JoinClause     `json:"joins,omitempty"`
	Where     *WhereClause     `json:"where,omitempty"`
	GroupBy   []string         `json:"group_by,omitempty"`
	Having    *WhereClause     `json:"having,omitempty"`
	OrderBy   []OrderByColumn  `json:"order_by,omitempty"`
	Limit     int              `json:"limit,omitempty"`
	HasWindow bool             `json:"has_window"`
}

// SelectColumn represents a column in the SELECT clause.
type SelectColumn struct {
	Expression string `json:"expression"`
	Alias      string `json:"alias,omitempty"`
	IsAgg      bool   `json:"is_aggregate"`
	IsWindow   bool   `json:"is_window"`
	AggFunc    string `json:"agg_func,omitempty"`
}

// JoinClause represents a JOIN.
type JoinClause struct {
	Type      string `json:"type"` // INNER, LEFT, RIGHT, CROSS
	Table     string `json:"table"`
	Condition string `json:"condition,omitempty"`
}

// WhereClause represents a WHERE condition (simplified).
type WhereClause struct {
	Raw string `json:"raw"`
}

// OrderByColumn represents an ORDER BY column.
type OrderByColumn struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

// Parser implements a recursive-descent parser for FeatherQL v2.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a parser from tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.peek()
	if t.Type != TokenEOF {
		p.pos++
	}
	return t
}

func (p *Parser) expect(typ TokenType, value string) (Token, error) {
	t := p.advance()
	if t.Type != typ || (value != "" && strings.ToUpper(t.Value) != value) {
		return t, fmt.Errorf("expected %q at position %d, got %q", value, t.Pos, t.Value)
	}
	return t, nil
}

func (p *Parser) isKeyword(val string) bool {
	t := p.peek()
	return t.Type == TokenKeyword && t.Value == val
}

// ParseSelect parses a full SELECT statement.
func (p *Parser) ParseSelect() (*SelectStatement, error) {
	if _, err := p.expect(TokenKeyword, "SELECT"); err != nil {
		return nil, fmt.Errorf("expected SELECT: %w", err)
	}

	stmt := &SelectStatement{}

	// Parse columns
	cols, err := p.parseSelectColumns()
	if err != nil {
		return nil, err
	}
	stmt.Columns = cols

	// FROM
	if _, err := p.expect(TokenKeyword, "FROM"); err != nil {
		return nil, fmt.Errorf("expected FROM: %w", err)
	}
	fromTok := p.advance()
	if fromTok.Type != TokenIdent {
		return nil, fmt.Errorf("expected table name at position %d, got %q", fromTok.Pos, fromTok.Value)
	}
	stmt.From = fromTok.Value

	// Optional JOIN clauses
	for p.isKeyword("JOIN") || p.isKeyword("LEFT") || p.isKeyword("RIGHT") || p.isKeyword("INNER") || p.isKeyword("CROSS") {
		join, err := p.parseJoin()
		if err != nil {
			return nil, err
		}
		stmt.Joins = append(stmt.Joins, join)
	}

	// Optional WHERE
	if p.isKeyword("WHERE") {
		p.advance()
		raw := p.consumeUntilKeyword("GROUP", "ORDER", "LIMIT", "HAVING")
		stmt.Where = &WhereClause{Raw: strings.TrimSpace(raw)}
	}

	// Optional GROUP BY
	if p.isKeyword("GROUP") {
		p.advance()
		if _, err := p.expect(TokenKeyword, "BY"); err != nil {
			return nil, err
		}
		for {
			col := p.advance()
			stmt.GroupBy = append(stmt.GroupBy, col.Value)
			if p.peek().Type != TokenComma {
				break
			}
			p.advance() // consume comma
		}
	}

	// Optional HAVING
	if p.isKeyword("HAVING") {
		p.advance()
		raw := p.consumeUntilKeyword("ORDER", "LIMIT")
		stmt.Having = &WhereClause{Raw: strings.TrimSpace(raw)}
	}

	// Optional ORDER BY
	if p.isKeyword("ORDER") {
		p.advance()
		if _, err := p.expect(TokenKeyword, "BY"); err != nil {
			return nil, err
		}
		for {
			col := p.advance()
			ob := OrderByColumn{Column: col.Value}
			if p.isKeyword("DESC") {
				ob.Desc = true
				p.advance()
			} else if p.isKeyword("ASC") {
				p.advance()
			}
			stmt.OrderBy = append(stmt.OrderBy, ob)
			if p.peek().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	// Optional LIMIT
	if p.isKeyword("LIMIT") {
		p.advance()
		limTok := p.advance()
		var lim int
		fmt.Sscanf(limTok.Value, "%d", &lim)
		stmt.Limit = lim
	}

	// Detect window functions
	for i := range stmt.Columns {
		if stmt.Columns[i].IsWindow {
			stmt.HasWindow = true
			break
		}
	}

	return stmt, nil
}

func (p *Parser) parseSelectColumns() ([]SelectColumn, error) {
	var cols []SelectColumn

	for {
		col, err := p.parseSelectColumn()
		if err != nil {
			return nil, err
		}
		cols = append(cols, col)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance() // consume comma
	}
	return cols, nil
}

func (p *Parser) parseSelectColumn() (SelectColumn, error) {
	col := SelectColumn{}
	var parts []string

	aggFuncs := map[string]bool{"AVG": true, "SUM": true, "COUNT": true, "MIN": true, "MAX": true, "FIRST": true, "LAST": true}

	// Check for aggregate function
	t := p.peek()
	if t.Type == TokenStar {
		p.advance()
		col.Expression = "*"
		goto checkAlias
	}

	if t.Type == TokenIdent || t.Type == TokenKeyword {
		upper := strings.ToUpper(t.Value)
		if aggFuncs[upper] {
			col.IsAgg = true
			col.AggFunc = upper
		}
	}

	// Collect expression tokens until comma, FROM keyword, or AS
	for {
		t := p.peek()
		if t.Type == TokenEOF || t.Type == TokenComma {
			break
		}
		if t.Type == TokenKeyword && (t.Value == "FROM" || t.Value == "AS") {
			break
		}

		// Detect OVER for window functions
		if t.Type == TokenKeyword && t.Value == "OVER" {
			col.IsWindow = true
		}

		parts = append(parts, t.Value)
		p.advance()
	}
	col.Expression = strings.Join(parts, " ")

checkAlias:
	// Optional AS alias
	if p.isKeyword("AS") {
		p.advance()
		alias := p.advance()
		col.Alias = alias.Value
	}

	return col, nil
}

func (p *Parser) parseJoin() (JoinClause, error) {
	join := JoinClause{Type: "INNER"}

	t := p.peek()
	if t.Value == "LEFT" || t.Value == "RIGHT" || t.Value == "CROSS" || t.Value == "INNER" {
		join.Type = t.Value
		p.advance()
		if p.isKeyword("OUTER") {
			p.advance()
		}
	}

	if _, err := p.expect(TokenKeyword, "JOIN"); err != nil {
		return join, err
	}

	table := p.advance()
	join.Table = table.Value

	if p.isKeyword("ON") {
		p.advance()
		raw := p.consumeUntilKeyword("WHERE", "GROUP", "ORDER", "LIMIT", "JOIN", "LEFT", "RIGHT", "INNER", "CROSS", "HAVING")
		join.Condition = strings.TrimSpace(raw)
	}

	return join, nil
}

func (p *Parser) consumeUntilKeyword(stops ...string) string {
	stopSet := make(map[string]bool)
	for _, s := range stops {
		stopSet[s] = true
	}

	var parts []string
	for {
		t := p.peek()
		if t.Type == TokenEOF {
			break
		}
		if t.Type == TokenKeyword && stopSet[t.Value] {
			break
		}
		parts = append(parts, t.Value)
		p.advance()
	}
	return strings.Join(parts, " ")
}

// ParseQuery is a convenience function that tokenizes and parses a query.
func ParseQuery(query string) (*SelectStatement, error) {
	tokens, err := Tokenize(query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailed, err)
	}
	parser := NewParser(tokens)
	stmt, err := parser.ParseSelect()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailed, err)
	}
	return stmt, nil
}
