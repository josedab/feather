package expr

import (
	"testing"
)

func TestLexer_BasicTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []TokenType
	}{
		{"1 + 2", []TokenType{TokenNumber, TokenPlus, TokenNumber, TokenEOF}},
		{"x * y", []TokenType{TokenIdent, TokenStar, TokenIdent, TokenEOF}},
		{"a >= b", []TokenType{TokenIdent, TokenGE, TokenIdent, TokenEOF}},
		{"!flag", []TokenType{TokenNot, TokenIdent, TokenEOF}},
		{"a && b", []TokenType{TokenIdent, TokenAnd, TokenIdent, TokenEOF}},
		{"a || b", []TokenType{TokenIdent, TokenOr, TokenIdent, TokenEOF}},
		{"x == y", []TokenType{TokenIdent, TokenEQ, TokenIdent, TokenEOF}},
		{"x != y", []TokenType{TokenIdent, TokenNE, TokenIdent, TokenEOF}},
		{"\"hello\"", []TokenType{TokenString, TokenEOF}},
		{"'world'", []TokenType{TokenString, TokenEOF}},
		{"func(a, b)", []TokenType{TokenIdent, TokenLParen, TokenIdent, TokenComma, TokenIdent, TokenRParen, TokenEOF}},
		{"arr[0]", []TokenType{TokenIdent, TokenLBracket, TokenNumber, TokenRBracket, TokenEOF}},
		{"obj.prop", []TokenType{TokenIdent, TokenDot, TokenIdent, TokenEOF}},
		{"a ? b : c", []TokenType{TokenIdent, TokenQuestion, TokenIdent, TokenColon, TokenIdent, TokenEOF}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			var tokens []TokenType
			for {
				tok := lexer.NextToken()
				tokens = append(tokens, tok.Type)
				if tok.Type == TokenEOF {
					break
				}
			}
			if len(tokens) != len(tt.expected) {
				t.Errorf("token count mismatch: got %d, want %d", len(tokens), len(tt.expected))
				return
			}
			for i, tok := range tokens {
				if tok != tt.expected[i] {
					t.Errorf("token[%d] = %v, want %v", i, tok, tt.expected[i])
				}
			}
		})
	}
}

func TestLexer_Numbers(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"42", 42},
		{"3.14", 3.14},
		{"0.5", 0.5},
		{"100.0", 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tok := lexer.NextToken()
			if tok.Type != TokenNumber {
				t.Errorf("expected TokenNumber, got %v", tok.Type)
				return
			}
			if tok.Literal.(float64) != tt.expected {
				t.Errorf("got %v, want %v", tok.Literal, tt.expected)
			}
		})
	}
}

func TestParser_NumberLiteral(t *testing.T) {
	parser := NewParser("42")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	num, ok := node.(*NumberNode)
	if !ok {
		t.Fatalf("expected NumberNode, got %T", node)
	}
	if num.Value != 42 {
		t.Errorf("got %v, want 42", num.Value)
	}
}

func TestParser_StringLiteral(t *testing.T) {
	parser := NewParser("\"hello world\"")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	str, ok := node.(*StringNode)
	if !ok {
		t.Fatalf("expected StringNode, got %T", node)
	}
	if str.Value != "hello world" {
		t.Errorf("got %q, want %q", str.Value, "hello world")
	}
}

func TestParser_Identifier(t *testing.T) {
	parser := NewParser("myVar")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ident, ok := node.(*IdentNode)
	if !ok {
		t.Fatalf("expected IdentNode, got %T", node)
	}
	if ident.Name != "myVar" {
		t.Errorf("got %q, want %q", ident.Name, "myVar")
	}
}

func TestParser_BinaryExpression(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{"1 + 2", "+"},
		{"3 - 4", "-"},
		{"5 * 6", "*"},
		{"7 / 8", "/"},
		{"9 % 10", "%"},
		{"a < b", "<"},
		{"a <= b", "<="},
		{"a > b", ">"},
		{"a >= b", ">="},
		{"a == b", "=="},
		{"a != b", "!="},
		{"a && b", "&&"},
		{"a || b", "||"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parser := NewParser(tt.input)
			node, err := parser.Parse()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			binary, ok := node.(*BinaryNode)
			if !ok {
				t.Fatalf("expected BinaryNode, got %T", node)
			}
			if binary.Op != tt.op {
				t.Errorf("got op %q, want %q", binary.Op, tt.op)
			}
		})
	}
}

func TestParser_UnaryExpression(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{"-5", "-"},
		{"!flag", "!"},
		{"-x", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parser := NewParser(tt.input)
			node, err := parser.Parse()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			unary, ok := node.(*UnaryNode)
			if !ok {
				t.Fatalf("expected UnaryNode, got %T", node)
			}
			if unary.Op != tt.op {
				t.Errorf("got op %q, want %q", unary.Op, tt.op)
			}
		})
	}
}

func TestParser_FunctionCall(t *testing.T) {
	tests := []struct {
		input    string
		name     string
		argCount int
	}{
		{"foo()", "foo", 0},
		{"bar(1)", "bar", 1},
		{"baz(1, 2)", "baz", 2},
		{"sum(a, b, c)", "sum", 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parser := NewParser(tt.input)
			node, err := parser.Parse()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			call, ok := node.(*CallNode)
			if !ok {
				t.Fatalf("expected CallNode, got %T", node)
			}
			if call.Name != tt.name {
				t.Errorf("got name %q, want %q", call.Name, tt.name)
			}
			if len(call.Args) != tt.argCount {
				t.Errorf("got %d args, want %d", len(call.Args), tt.argCount)
			}
		})
	}
}

func TestParser_IndexExpression(t *testing.T) {
	parser := NewParser("arr[0]")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	index, ok := node.(*IndexNode)
	if !ok {
		t.Fatalf("expected IndexNode, got %T", node)
	}
	obj, ok := index.Object.(*IdentNode)
	if !ok {
		t.Fatalf("expected IdentNode object, got %T", index.Object)
	}
	if obj.Name != "arr" {
		t.Errorf("got object %q, want %q", obj.Name, "arr")
	}
}

func TestParser_PropertyExpression(t *testing.T) {
	parser := NewParser("obj.prop")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	prop, ok := node.(*PropertyNode)
	if !ok {
		t.Fatalf("expected PropertyNode, got %T", node)
	}
	if prop.Property != "prop" {
		t.Errorf("got property %q, want %q", prop.Property, "prop")
	}
}

func TestParser_TernaryExpression(t *testing.T) {
	parser := NewParser("x > 0 ? x : -x")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	ternary, ok := node.(*TernaryNode)
	if !ok {
		t.Fatalf("expected TernaryNode, got %T", node)
	}
	if ternary.Condition == nil || ternary.Then == nil || ternary.Else == nil {
		t.Error("ternary has nil parts")
	}
}

func TestParser_ParenthesizedExpression(t *testing.T) {
	parser := NewParser("(1 + 2) * 3")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	binary, ok := node.(*BinaryNode)
	if !ok {
		t.Fatalf("expected BinaryNode, got %T", node)
	}
	if binary.Op != "*" {
		t.Errorf("got op %q, want %q", binary.Op, "*")
	}
}

func TestParser_NestedExpressions(t *testing.T) {
	tests := []string{
		"max(min(a, b), c)",
		"arr[0].prop",
		"obj.items[0]",
		"a + b * c",
		"(a + b) * (c + d)",
		"x > 0 && y > 0",
		"a || b && c",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			parser := NewParser(tt)
			_, err := parser.Parse()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
		})
	}
}

func TestParser_Precedence(t *testing.T) {
	// Test that * has higher precedence than +
	parser := NewParser("1 + 2 * 3")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	binary, ok := node.(*BinaryNode)
	if !ok {
		t.Fatalf("expected BinaryNode, got %T", node)
	}
	// Should be (1 + (2 * 3)), so outer op is +
	if binary.Op != "+" {
		t.Errorf("expected + at top level, got %q", binary.Op)
	}
	right, ok := binary.Right.(*BinaryNode)
	if !ok {
		t.Fatalf("expected BinaryNode on right, got %T", binary.Right)
	}
	if right.Op != "*" {
		t.Errorf("expected * on right, got %q", right.Op)
	}
}

func TestParser_BooleanKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"true", 1},
		{"false", 0},
		{"TRUE", 1},
		{"FALSE", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parser := NewParser(tt.input)
			node, err := parser.Parse()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			num, ok := node.(*NumberNode)
			if !ok {
				t.Fatalf("expected NumberNode, got %T", node)
			}
			if num.Value != tt.expected {
				t.Errorf("got %v, want %v", num.Value, tt.expected)
			}
		})
	}
}

func TestParser_ChainedPropertyAccess(t *testing.T) {
	parser := NewParser("a.b.c")
	node, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	prop, ok := node.(*PropertyNode)
	if !ok {
		t.Fatalf("expected PropertyNode, got %T", node)
	}
	if prop.Property != "c" {
		t.Errorf("got property %q, want %q", prop.Property, "c")
	}
	inner, ok := prop.Object.(*PropertyNode)
	if !ok {
		t.Fatalf("expected PropertyNode object, got %T", prop.Object)
	}
	if inner.Property != "b" {
		t.Errorf("got inner property %q, want %q", inner.Property, "b")
	}
}

func TestParser_ErrorCases(t *testing.T) {
	tests := []string{
		"(",     // unclosed paren
		"[",     // unclosed bracket
		"a[",    // unclosed index
		"1 +",   // incomplete binary
		"func(", // unclosed call
		"a ? b", // incomplete ternary
		"a.123", // invalid property
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			parser := NewParser(tt)
			_, err := parser.Parse()
			if err == nil {
				t.Error("expected parse error, got nil")
			}
		})
	}
}

func TestNode_String(t *testing.T) {
	tests := []struct {
		node     Node
		expected string
	}{
		{&NumberNode{Value: 42}, "42"},
		{&StringNode{Value: "hello"}, `"hello"`},
		{&IdentNode{Name: "x"}, "x"},
		{&BinaryNode{Op: "+", Left: &NumberNode{Value: 1}, Right: &NumberNode{Value: 2}}, "(1 + 2)"},
		{&UnaryNode{Op: "-", Operand: &NumberNode{Value: 5}}, "(-5)"},
		{&CallNode{Name: "sum", Args: []Node{&NumberNode{Value: 1}, &NumberNode{Value: 2}}}, "sum(1, 2)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.node.String()
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}
