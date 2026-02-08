// Package expr provides expression parsing and evaluation for feature composition.
package expr

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// TokenType represents the type of a token.
type TokenType int

const (
	// TokenEOF marks the end of input.
	TokenEOF TokenType = iota
	// TokenNumber represents a numeric literal.
	TokenNumber
	// TokenString represents a string literal.
	TokenString
	// TokenIdent represents an identifier.
	TokenIdent
	// TokenPlus represents the "+" operator.
	TokenPlus
	// TokenMinus represents the "-" operator.
	TokenMinus
	// TokenStar represents the "*" operator.
	TokenStar
	// TokenSlash represents the "/" operator.
	TokenSlash
	// TokenPercent represents the "%" operator.
	TokenPercent
	// TokenLParen represents the "(" token.
	TokenLParen
	// TokenRParen represents the ")" token.
	TokenRParen
	// TokenComma represents the "," token.
	TokenComma
	// TokenDot represents the "." token.
	TokenDot
	// TokenLT represents the "<" operator.
	TokenLT
	// TokenLE represents the "<=" operator.
	TokenLE
	// TokenGT represents the ">" operator.
	TokenGT
	// TokenGE represents the ">=" operator.
	TokenGE
	// TokenEQ represents the "==" operator.
	TokenEQ
	// TokenNE represents the "!=" operator.
	TokenNE
	// TokenAnd represents the "&&" operator.
	TokenAnd
	// TokenOr represents the "||" operator.
	TokenOr
	// TokenNot represents the "!" operator.
	TokenNot
	// TokenQuestion represents the "?" token.
	TokenQuestion
	// TokenColon represents the ":" token.
	TokenColon
	// TokenLBracket represents the "[" token.
	TokenLBracket
	// TokenRBracket represents the "]" token.
	TokenRBracket
)

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Value   string
	Pos     int
	Literal interface{}
}

// Lexer tokenizes an expression string.
type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	pos := l.pos
	var tok Token

	switch l.ch {
	case 0:
		tok = Token{Type: TokenEOF, Value: "", Pos: pos}
	case '+':
		tok = Token{Type: TokenPlus, Value: "+", Pos: pos}
	case '-':
		tok = Token{Type: TokenMinus, Value: "-", Pos: pos}
	case '*':
		tok = Token{Type: TokenStar, Value: "*", Pos: pos}
	case '/':
		tok = Token{Type: TokenSlash, Value: "/", Pos: pos}
	case '%':
		tok = Token{Type: TokenPercent, Value: "%", Pos: pos}
	case '(':
		tok = Token{Type: TokenLParen, Value: "(", Pos: pos}
	case ')':
		tok = Token{Type: TokenRParen, Value: ")", Pos: pos}
	case '[':
		tok = Token{Type: TokenLBracket, Value: "[", Pos: pos}
	case ']':
		tok = Token{Type: TokenRBracket, Value: "]", Pos: pos}
	case ',':
		tok = Token{Type: TokenComma, Value: ",", Pos: pos}
	case '.':
		tok = Token{Type: TokenDot, Value: ".", Pos: pos}
	case '?':
		tok = Token{Type: TokenQuestion, Value: "?", Pos: pos}
	case ':':
		tok = Token{Type: TokenColon, Value: ":", Pos: pos}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenLE, Value: "<=", Pos: pos}
		} else {
			tok = Token{Type: TokenLT, Value: "<", Pos: pos}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenGE, Value: ">=", Pos: pos}
		} else {
			tok = Token{Type: TokenGT, Value: ">", Pos: pos}
		}
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenEQ, Value: "==", Pos: pos}
		} else {
			tok = Token{Type: TokenEQ, Value: "=", Pos: pos}
		}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok = Token{Type: TokenNE, Value: "!=", Pos: pos}
		} else {
			tok = Token{Type: TokenNot, Value: "!", Pos: pos}
		}
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			tok = Token{Type: TokenAnd, Value: "&&", Pos: pos}
		} else {
			tok = Token{Type: TokenAnd, Value: "&", Pos: pos}
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			tok = Token{Type: TokenOr, Value: "||", Pos: pos}
		} else {
			tok = Token{Type: TokenOr, Value: "|", Pos: pos}
		}
	case '"', '\'':
		quote := l.ch
		l.readChar()
		start := l.pos
		for l.ch != quote && l.ch != 0 {
			l.readChar()
		}
		value := l.input[start:l.pos]
		tok = Token{Type: TokenString, Value: value, Pos: pos, Literal: value}
	default:
		if isDigit(l.ch) {
			return l.readNumber(pos)
		} else if isLetter(l.ch) {
			return l.readIdentifier(pos)
		}
		tok = Token{Type: TokenEOF, Value: string(l.ch), Pos: pos}
	}

	l.readChar()
	return tok
}

func (l *Lexer) readNumber(pos int) Token {
	start := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	value := l.input[start:l.pos]
	num, _ := strconv.ParseFloat(value, 64)
	return Token{Type: TokenNumber, Value: value, Pos: pos, Literal: num}
}

func (l *Lexer) readIdentifier(pos int) Token {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	value := l.input[start:l.pos]
	return Token{Type: TokenIdent, Value: value, Pos: pos}
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

// Node represents an AST node.
type Node interface {
	String() string
}

// NumberNode represents a numeric literal.
type NumberNode struct {
	Value float64
}

func (n *NumberNode) String() string {
	return strconv.FormatFloat(n.Value, 'f', -1, 64)
}

// StringNode represents a string literal.
type StringNode struct {
	Value string
}

func (n *StringNode) String() string {
	return fmt.Sprintf("%q", n.Value)
}

// IdentNode represents an identifier (variable reference).
type IdentNode struct {
	Name string
}

func (n *IdentNode) String() string {
	return n.Name
}

// BinaryNode represents a binary operation.
type BinaryNode struct {
	Op    string
	Left  Node
	Right Node
}

func (n *BinaryNode) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Left.String(), n.Op, n.Right.String())
}

// UnaryNode represents a unary operation.
type UnaryNode struct {
	Op      string
	Operand Node
}

func (n *UnaryNode) String() string {
	return fmt.Sprintf("(%s%s)", n.Op, n.Operand.String())
}

// CallNode represents a function call.
type CallNode struct {
	Name string
	Args []Node
}

func (n *CallNode) String() string {
	args := make([]string, len(n.Args))
	for i, arg := range n.Args {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", n.Name, strings.Join(args, ", "))
}

// IndexNode represents array/map indexing.
type IndexNode struct {
	Object Node
	Index  Node
}

func (n *IndexNode) String() string {
	return fmt.Sprintf("%s[%s]", n.Object.String(), n.Index.String())
}

// TernaryNode represents a ternary conditional.
type TernaryNode struct {
	Condition Node
	Then      Node
	Else      Node
}

func (n *TernaryNode) String() string {
	return fmt.Sprintf("(%s ? %s : %s)", n.Condition.String(), n.Then.String(), n.Else.String())
}

// PropertyNode represents property access.
type PropertyNode struct {
	Object   Node
	Property string
}

func (n *PropertyNode) String() string {
	return fmt.Sprintf("%s.%s", n.Object.String(), n.Property)
}

// Parser parses expressions into an AST.
type Parser struct {
	lexer   *Lexer
	current Token
	peek    Token
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	p := &Parser{lexer: NewLexer(input)}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.current = p.peek
	p.peek = p.lexer.NextToken()
}

// Parse parses the expression and returns the AST root.
func (p *Parser) Parse() (Node, error) {
	return p.parseExpression(0)
}

func (p *Parser) parseExpression(minPrec int) (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		prec := p.precedence(p.current.Type)
		if prec < minPrec {
			break
		}

		op := p.current.Value
		opType := p.current.Type

		if opType == TokenQuestion {
			p.nextToken()
			then, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if p.current.Type != TokenColon {
				return nil, fmt.Errorf("expected ':' in ternary expression at position %d", p.current.Pos)
			}
			p.nextToken()
			elseNode, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			left = &TernaryNode{Condition: left, Then: then, Else: elseNode}
			continue
		}

		p.nextToken()
		right, err := p.parseExpression(prec + 1)
		if err != nil {
			return nil, err
		}

		left = &BinaryNode{Op: op, Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseUnary() (Node, error) {
	if p.current.Type == TokenMinus || p.current.Type == TokenNot {
		op := p.current.Value
		p.nextToken()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryNode{Op: op, Operand: operand}, nil
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (Node, error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch p.current.Type {
		case TokenLParen:
			// Function call
			if ident, ok := node.(*IdentNode); ok {
				args, err := p.parseArguments()
				if err != nil {
					return nil, err
				}
				node = &CallNode{Name: ident.Name, Args: args}
			} else {
				return nil, fmt.Errorf("unexpected '(' at position %d", p.current.Pos)
			}
		case TokenLBracket:
			// Index access
			p.nextToken()
			index, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if p.current.Type != TokenRBracket {
				return nil, fmt.Errorf("expected ']' at position %d", p.current.Pos)
			}
			p.nextToken()
			node = &IndexNode{Object: node, Index: index}
		case TokenDot:
			// Property access
			p.nextToken()
			if p.current.Type != TokenIdent {
				return nil, fmt.Errorf("expected identifier after '.' at position %d", p.current.Pos)
			}
			property := p.current.Value
			p.nextToken()
			node = &PropertyNode{Object: node, Property: property}
		default:
			return node, nil
		}
	}
}

func (p *Parser) parsePrimary() (Node, error) {
	switch p.current.Type {
	case TokenNumber:
		num, ok := p.current.Literal.(float64)
		if !ok {
			return nil, fmt.Errorf("expected number at position %d", p.current.Pos)
		}
		p.nextToken()
		return &NumberNode{Value: num}, nil
	case TokenString:
		str, ok := p.current.Literal.(string)
		if !ok {
			return nil, fmt.Errorf("expected string at position %d", p.current.Pos)
		}
		p.nextToken()
		return &StringNode{Value: str}, nil
	case TokenIdent:
		name := p.current.Value
		p.nextToken()
		// Check for keywords
		switch strings.ToLower(name) {
		case "true":
			return &NumberNode{Value: 1}, nil
		case "false":
			return &NumberNode{Value: 0}, nil
		case "null", "nil":
			return &IdentNode{Name: "null"}, nil
		}
		return &IdentNode{Name: name}, nil
	case TokenLParen:
		p.nextToken()
		node, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRParen {
			return nil, fmt.Errorf("expected ')' at position %d", p.current.Pos)
		}
		p.nextToken()
		return node, nil
	default:
		return nil, fmt.Errorf("unexpected token '%s' at position %d", p.current.Value, p.current.Pos)
	}
}

func (p *Parser) parseArguments() ([]Node, error) {
	if p.current.Type != TokenLParen {
		return nil, fmt.Errorf("expected '(' at position %d", p.current.Pos)
	}
	p.nextToken()

	var args []Node
	for p.current.Type != TokenRParen && p.current.Type != TokenEOF {
		arg, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)

		if p.current.Type == TokenComma {
			p.nextToken()
		} else {
			break
		}
	}

	if p.current.Type != TokenRParen {
		return nil, fmt.Errorf("expected ')' at position %d", p.current.Pos)
	}
	p.nextToken()

	return args, nil
}

func (p *Parser) precedence(t TokenType) int {
	switch t {
	case TokenOr:
		return 1
	case TokenAnd:
		return 2
	case TokenEQ, TokenNE:
		return 3
	case TokenLT, TokenLE, TokenGT, TokenGE:
		return 4
	case TokenPlus, TokenMinus:
		return 5
	case TokenStar, TokenSlash, TokenPercent:
		return 6
	case TokenQuestion:
		return 0 // Lowest precedence for ternary
	default:
		return -1
	}
}
