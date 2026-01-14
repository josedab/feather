package featherql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer_SimpleSelect(t *testing.T) {
	input := "SELECT clicks, revenue FROM user_events"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	// SELECT, clicks, ',', revenue, FROM, user_events, EOF
	assert.Equal(t, TokenKeyword, tokens[0].Type)
	assert.Equal(t, "SELECT", tokens[0].Value)
	assert.Equal(t, TokenIdent, tokens[1].Type)
	assert.Equal(t, "clicks", tokens[1].Value)
	assert.Equal(t, TokenPunctuation, tokens[2].Type)
	assert.Equal(t, TokenIdent, tokens[3].Type)
	assert.Equal(t, TokenKeyword, tokens[4].Type)
	assert.Equal(t, "FROM", tokens[4].Value)
	assert.Equal(t, TokenIdent, tokens[5].Type)
	assert.Equal(t, TokenEOF, tokens[6].Type)
}

func TestLexer_AggregationWithWindow(t *testing.T) {
	input := "SELECT COUNT(clicks) AS click_count FROM users WINDOW SLIDING 1h SLIDE BY 5m"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)
	require.NotEmpty(t, tokens)

	// Find the duration tokens
	var durationTokens []Token
	for _, tok := range tokens {
		if tok.Type == TokenDuration {
			durationTokens = append(durationTokens, tok)
		}
	}
	assert.Len(t, durationTokens, 2)
	assert.Equal(t, "1h", durationTokens[0].Value)
	assert.Equal(t, "5m", durationTokens[1].Value)
}

func TestLexer_StringLiteral(t *testing.T) {
	input := "SELECT name FROM users WHERE status = 'active'"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	var strTokens []Token
	for _, tok := range tokens {
		if tok.Type == TokenString {
			strTokens = append(strTokens, tok)
		}
	}
	require.Len(t, strTokens, 1)
	assert.Equal(t, "active", strTokens[0].Value)
}

func TestLexer_Comment(t *testing.T) {
	input := "-- this is a comment\nSELECT clicks FROM users"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	assert.Equal(t, TokenKeyword, tokens[0].Type)
	assert.Equal(t, "SELECT", tokens[0].Value)
}

func TestLexer_Operators(t *testing.T) {
	input := "clicks >= 10"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	require.NoError(t, err)

	assert.Equal(t, TokenIdent, tokens[0].Type)
	assert.Equal(t, TokenOperator, tokens[1].Type)
	assert.Equal(t, ">=", tokens[1].Value)
	assert.Equal(t, TokenNumber, tokens[2].Type)
}

func TestParser_SimpleSelect(t *testing.T) {
	tokens, _ := NewLexer("SELECT clicks, revenue FROM users").Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	require.NoError(t, err)

	assert.Equal(t, NodeSelect, ast.Type)
	assert.Equal(t, "users", ast.Value)
	assert.Len(t, ast.Children, 2)
	assert.Equal(t, "clicks", ast.Children[0].Value)
	assert.Equal(t, "revenue", ast.Children[1].Value)
}

func TestParser_Aggregation(t *testing.T) {
	tokens, _ := NewLexer("SELECT COUNT(clicks) AS click_count FROM users").Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	require.NoError(t, err)

	assert.Equal(t, NodeSelect, ast.Type)
	require.Len(t, ast.Children, 1)
	assert.Equal(t, NodeAlias, ast.Children[0].Type)
	assert.Equal(t, "click_count", ast.Children[0].Value)
}

func TestParser_CreateFeature(t *testing.T) {
	tokens, _ := NewLexer("CREATE FEATURE click_rate AS SELECT clicks FROM events").Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	require.NoError(t, err)

	assert.Equal(t, NodeCreateFeature, ast.Type)
	assert.Equal(t, "click_rate", ast.Value)
	require.Len(t, ast.Children, 1)
	assert.Equal(t, NodeSelect, ast.Children[0].Type)
}

func TestParser_WithFilter(t *testing.T) {
	tokens, _ := NewLexer("SELECT clicks FROM users WHERE clicks > 10").Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	require.NoError(t, err)

	assert.Equal(t, NodeSelect, ast.Type)
	// Should have column + filter
	require.Len(t, ast.Children, 2)
	assert.Equal(t, NodeFilter, ast.Children[1].Type)
	assert.Equal(t, ">", ast.Children[1].Value)
}

func TestParser_WithWindow(t *testing.T) {
	tokens, _ := NewLexer("SELECT SUM(revenue) FROM orders WINDOW SLIDING 24h SLIDE BY 1h").Tokenize()
	parser := NewParser(tokens)
	ast, err := parser.Parse()
	require.NoError(t, err)

	var windowNode *ASTNode
	for _, child := range ast.Children {
		if child.Type == NodeWindow {
			windowNode = child
			break
		}
	}
	require.NotNil(t, windowNode)
	assert.Equal(t, "SLIDING", windowNode.Value)
	require.Len(t, windowNode.Children, 2)
	assert.Equal(t, "24h", windowNode.Children[0].Value)
	assert.Equal(t, "1h", windowNode.Children[1].Value)
}

func TestCompiler_SelectPipeline(t *testing.T) {
	tokens, _ := NewLexer("SELECT COUNT(clicks) AS click_count FROM users WINDOW SLIDING 1h").Tokenize()
	parser := NewParser(tokens)
	ast, _ := parser.Parse()

	compiler := NewCompiler()
	pipeline, err := compiler.Compile(ast)
	require.NoError(t, err)

	assert.Equal(t, "users", pipeline.EntityType)
	require.Len(t, pipeline.Columns, 1)
	assert.Equal(t, "click_count", pipeline.Columns[0].Name)
	assert.Equal(t, "COUNT", pipeline.Columns[0].AggFunc)
	assert.NotNil(t, pipeline.Window)
	assert.Equal(t, "sliding", pipeline.Window.Type)
}

func TestCompiler_CreateFeaturePipeline(t *testing.T) {
	tokens, _ := NewLexer("CREATE FEATURE user_clicks AS SELECT SUM(clicks) AS total FROM events").Tokenize()
	parser := NewParser(tokens)
	ast, _ := parser.Parse()

	compiler := NewCompiler()
	pipeline, err := compiler.Compile(ast)
	require.NoError(t, err)

	assert.Equal(t, "user_clicks", pipeline.Name)
	require.Len(t, pipeline.Columns, 1)
	assert.Equal(t, "total", pipeline.Columns[0].Name)
	assert.Equal(t, "SUM", pipeline.Columns[0].AggFunc)
}

func TestCompiler_NilAST(t *testing.T) {
	compiler := NewCompiler()
	_, err := compiler.Compile(nil)
	require.Error(t, err)
}

func TestEndToEnd_ParseCompile(t *testing.T) {
	queries := []string{
		"SELECT clicks FROM users",
		"SELECT COUNT(clicks) AS cnt FROM events WINDOW TUMBLING 1h",
		"CREATE FEATURE hourly_clicks AS SELECT SUM(clicks) FROM events WINDOW SLIDING 1h SLIDE BY 5m",
		"SELECT revenue FROM orders WHERE amount > 100",
	}

	compiler := NewCompiler()
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			tokens, err := NewLexer(q).Tokenize()
			require.NoError(t, err)

			ast, err := NewParser(tokens).Parse()
			require.NoError(t, err)

			pipeline, err := compiler.Compile(ast)
			require.NoError(t, err)
			assert.NotEmpty(t, pipeline.Columns)
		})
	}
}
