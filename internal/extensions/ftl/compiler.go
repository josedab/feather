// Package ftl provides a Feature Transformation Language (FTL) compiler
// for in-memory feature transformations using a SQL-like DSL.
package ftl

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TokenType classifies lexer tokens.
type TokenType int

const (
	TokenSelect     TokenType = iota
	TokenFrom
	TokenWhere
	TokenJoin
	TokenGroupBy
	TokenOrderBy
	TokenLimit
	TokenAs
	TokenAnd
	TokenOr
	TokenIdentifier
	TokenNumber
	TokenString
	TokenOperator
	TokenLParen
	TokenRParen
	TokenComma
	TokenStar
	TokenEOF
)

// Token is a single lexer token.
type Token struct {
	Type     TokenType `json:"type"`
	Value    string    `json:"value"`
	Position int       `json:"position"`
}

// keywordMap maps keyword strings to their token types.
var keywordMap = map[string]TokenType{
	"SELECT":   TokenSelect,
	"FROM":     TokenFrom,
	"WHERE":    TokenWhere,
	"JOIN":     TokenJoin,
	"GROUP BY": TokenGroupBy,
	"GROUP":    TokenGroupBy,
	"ORDER BY": TokenOrderBy,
	"ORDER":    TokenOrderBy,
	"LIMIT":    TokenLimit,
	"AS":       TokenAs,
	"AND":      TokenAnd,
	"OR":       TokenOr,
}

// NodeType classifies AST nodes.
type NodeType string

const (
	NodeTypeSelect       NodeType = "select"
	NodeTypeFrom         NodeType = "from"
	NodeTypeWhere        NodeType = "where"
	NodeTypeJoin         NodeType = "join"
	NodeTypeGroupBy      NodeType = "groupby"
	NodeTypeFunctionCall NodeType = "function_call"
	NodeTypeBinaryOp     NodeType = "binary_op"
	NodeTypeColumnRef    NodeType = "column_ref"
	NodeTypeLiteral      NodeType = "literal"
	NodeTypeAlias        NodeType = "alias"
)

// ASTNode represents a node in the abstract syntax tree.
type ASTNode struct {
	Type     NodeType   `json:"type"`
	Value    string     `json:"value"`
	Children []*ASTNode `json:"children,omitempty"`
	Alias    string     `json:"alias,omitempty"`
}

// Pipeline represents a compiled feature transformation pipeline.
type Pipeline struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	AST            *ASTNode  `json:"ast"`
	Compiled       bool      `json:"compiled"`
	CreatedAt      time.Time `json:"created_at"`
	ExecutionCount int64     `json:"execution_count"`
	AvgLatencyUs   float64   `json:"avg_latency_us"`
}

// TransformResult represents the result of executing a pipeline.
type TransformResult struct {
	Columns         []string                 `json:"columns"`
	Rows            []map[string]interface{} `json:"rows"`
	RowCount        int                      `json:"row_count"`
	ExecutionTimeUs int64                    `json:"execution_time_us"`
}

// CompileError represents a compilation error.
type CompileError struct {
	Message  string `json:"message"`
	Position int    `json:"position"`
	Line     int    `json:"line"`
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("compile error at position %d (line %d): %s", e.Position, e.Line, e.Message)
}

// FTLStats holds runtime statistics for the FTL compiler.
type FTLStats struct {
	PipelinesCompiled    int     `json:"pipelines_compiled"`
	PipelinesExecuted    int     `json:"pipelines_executed"`
	TotalTransformations int     `json:"total_transformations"`
	AvgLatencyUs         float64 `json:"avg_latency_us"`
	ErrorCount           int     `json:"error_count"`
}

// CompilerConfig configures the FTL compiler.
type CompilerConfig struct {
	MaxPipelineDepth    int  `json:"max_pipeline_depth"`
	MaxRowsPerTransform int  `json:"max_rows_per_transform"`
	EnableOptimizer     bool `json:"enable_optimizer"`
	CacheCompiledAST    bool `json:"cache_compiled_ast"`
}

// DefaultCompilerConfig returns sensible defaults for the compiler.
func DefaultCompilerConfig() CompilerConfig {
	return CompilerConfig{
		MaxPipelineDepth:    64,
		MaxRowsPerTransform: 100000,
		EnableOptimizer:     true,
		CacheCompiledAST:    true,
	}
}

// Compiler is the FTL compiler and executor.
type Compiler struct {
	mu        sync.RWMutex
	config    CompilerConfig
	pipelines map[string]*Pipeline
	stats     FTLStats
}

// NewCompiler creates a new FTL compiler with the given configuration.
func NewCompiler(cfg CompilerConfig) *Compiler {
	return &Compiler{
		config:    cfg,
		pipelines: make(map[string]*Pipeline),
	}
}

// Tokenize lexes the input string into a slice of tokens.
func (c *Compiler) Tokenize(input string) ([]Token, error) {
	var tokens []Token
	i := 0

	for i < len(input) {
		// Skip whitespace
		if input[i] == ' ' || input[i] == '\t' || input[i] == '\n' || input[i] == '\r' {
			i++
			continue
		}

		// String literal
		if input[i] == '\'' {
			start := i
			i++ // skip opening quote
			for i < len(input) && input[i] != '\'' {
				i++
			}
			if i >= len(input) {
				return nil, &CompileError{Message: "unterminated string literal", Position: start, Line: 1}
			}
			tokens = append(tokens, Token{Type: TokenString, Value: input[start+1 : i], Position: start})
			i++ // skip closing quote
			continue
		}

		// Number
		if isDigit(input[i]) || (input[i] == '-' && i+1 < len(input) && isDigit(input[i+1])) {
			start := i
			if input[i] == '-' {
				i++
			}
			for i < len(input) && (isDigit(input[i]) || input[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: input[start:i], Position: start})
			continue
		}

		// Operators
		if input[i] == '>' || input[i] == '<' || input[i] == '!' || input[i] == '=' {
			start := i
			i++
			if i < len(input) && input[i] == '=' {
				i++
			}
			tokens = append(tokens, Token{Type: TokenOperator, Value: input[start:i], Position: start})
			continue
		}
		if input[i] == '+' || input[i] == '-' || input[i] == '/' {
			tokens = append(tokens, Token{Type: TokenOperator, Value: string(input[i]), Position: i})
			i++
			continue
		}

		// Parentheses
		if input[i] == '(' {
			tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Position: i})
			i++
			continue
		}
		if input[i] == ')' {
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Position: i})
			i++
			continue
		}

		// Comma
		if input[i] == ',' {
			tokens = append(tokens, Token{Type: TokenComma, Value: ",", Position: i})
			i++
			continue
		}

		// Star
		if input[i] == '*' {
			tokens = append(tokens, Token{Type: TokenStar, Value: "*", Position: i})
			i++
			continue
		}

		// Identifiers and keywords
		if isLetter(input[i]) || input[i] == '_' {
			start := i
			for i < len(input) && (isLetter(input[i]) || isDigit(input[i]) || input[i] == '_' || input[i] == '.') {
				i++
			}
			word := input[start:i]
			upper := strings.ToUpper(word)

			if tt, ok := keywordMap[upper]; ok {
				// Handle two-word keywords: GROUP BY, ORDER BY
				if (upper == "GROUP" || upper == "ORDER") && i < len(input) {
					j := i
					for j < len(input) && (input[j] == ' ' || input[j] == '\t') {
						j++
					}
					if j+2 <= len(input) && strings.ToUpper(input[j:j+2]) == "BY" {
						next := j + 2
						if next >= len(input) || !isLetter(input[next]) {
							i = next
						}
					}
				}
				tokens = append(tokens, Token{Type: tt, Value: upper, Position: start})
			} else {
				tokens = append(tokens, Token{Type: TokenIdentifier, Value: word, Position: start})
			}
			continue
		}

		return nil, &CompileError{
			Message:  fmt.Sprintf("unexpected character %q", input[i]),
			Position: i,
			Line:     1,
		}
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: "", Position: i})
	return tokens, nil
}

// parser is an internal parser that builds an AST from tokens.
type parser struct {
	tokens []Token
	pos    int
}

// Parse parses an FTL query string into an AST.
func (c *Compiler) Parse(input string) (*ASTNode, error) {
	tokens, err := c.Tokenize(input)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	return p.parseQuery()
}

func (p *parser) peek() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

func (p *parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) expect(tt TokenType) (Token, error) {
	tok := p.peek()
	if tok.Type != tt {
		return tok, &CompileError{
			Message:  fmt.Sprintf("expected token type %d, got %d (%q)", tt, tok.Type, tok.Value),
			Position: tok.Position,
			Line:     1,
		}
	}
	return p.advance(), nil
}

func (p *parser) parseQuery() (*ASTNode, error) {
	tok := p.peek()
	if tok.Type == TokenEOF {
		return nil, &CompileError{Message: "empty query", Position: 0, Line: 1}
	}
	if tok.Type != TokenSelect {
		return nil, &CompileError{
			Message:  fmt.Sprintf("expected SELECT, got %q", tok.Value),
			Position: tok.Position,
			Line:     1,
		}
	}
	return p.parseSelect()
}

func (p *parser) parseSelect() (*ASTNode, error) {
	p.advance() // consume SELECT
	selectNode := &ASTNode{Type: NodeTypeSelect, Value: "SELECT"}

	// Parse column list
	columns, err := p.parseColumnList()
	if err != nil {
		return nil, err
	}
	selectNode.Children = append(selectNode.Children, columns...)

	// Parse FROM
	if p.peek().Type == TokenFrom {
		p.advance()
		fromNode, err := p.parseFrom()
		if err != nil {
			return nil, err
		}
		selectNode.Children = append(selectNode.Children, fromNode)
	}

	// Parse WHERE
	if p.peek().Type == TokenWhere {
		p.advance()
		whereNode, err := p.parseWhere()
		if err != nil {
			return nil, err
		}
		selectNode.Children = append(selectNode.Children, whereNode)
	}

	// Parse GROUP BY
	if p.peek().Type == TokenGroupBy {
		p.advance()
		groupNode, err := p.parseGroupBy()
		if err != nil {
			return nil, err
		}
		selectNode.Children = append(selectNode.Children, groupNode)
	}

	// Parse ORDER BY
	if p.peek().Type == TokenOrderBy {
		p.advance()
		// consume column references for ORDER BY (simplified)
		for p.peek().Type == TokenIdentifier {
			p.advance()
			if p.peek().Type == TokenComma {
				p.advance()
			}
		}
	}

	// Parse LIMIT
	if p.peek().Type == TokenLimit {
		p.advance()
		if p.peek().Type == TokenNumber {
			p.advance()
		}
	}

	return selectNode, nil
}

func (p *parser) parseColumnList() ([]*ASTNode, error) {
	var columns []*ASTNode

	if p.peek().Type == TokenStar {
		p.advance()
		columns = append(columns, &ASTNode{Type: NodeTypeColumnRef, Value: "*"})
		return columns, nil
	}

	for {
		col, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		// Check for alias
		if p.peek().Type == TokenAs {
			p.advance()
			aliasTok, err := p.expect(TokenIdentifier)
			if err != nil {
				return nil, &CompileError{Message: "expected alias name after AS", Position: p.peek().Position, Line: 1}
			}
			col = &ASTNode{Type: NodeTypeAlias, Value: aliasTok.Value, Children: []*ASTNode{col}}
		}

		columns = append(columns, col)

		if p.peek().Type != TokenComma {
			break
		}
		p.advance() // consume comma
	}

	return columns, nil
}

func (p *parser) parseExpression() (*ASTNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	// Check for binary operator
	if p.peek().Type == TokenOperator {
		op := p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &ASTNode{
			Type:     NodeTypeBinaryOp,
			Value:    op.Value,
			Children: []*ASTNode{left, right},
		}, nil
	}

	return left, nil
}

func (p *parser) parsePrimary() (*ASTNode, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenIdentifier:
		p.advance()
		// Check for function call
		if p.peek().Type == TokenLParen {
			return p.parseFunctionCall(tok.Value)
		}
		return &ASTNode{Type: NodeTypeColumnRef, Value: tok.Value}, nil

	case TokenNumber:
		p.advance()
		return &ASTNode{Type: NodeTypeLiteral, Value: tok.Value}, nil

	case TokenString:
		p.advance()
		return &ASTNode{Type: NodeTypeLiteral, Value: tok.Value}, nil

	case TokenStar:
		p.advance()
		return &ASTNode{Type: NodeTypeColumnRef, Value: "*"}, nil

	case TokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen); err != nil {
			return nil, &CompileError{Message: "expected closing parenthesis", Position: p.peek().Position, Line: 1}
		}
		return expr, nil

	default:
		return nil, &CompileError{
			Message:  fmt.Sprintf("unexpected token %q", tok.Value),
			Position: tok.Position,
			Line:     1,
		}
	}
}

func (p *parser) parseFunctionCall(name string) (*ASTNode, error) {
	p.advance() // consume (
	node := &ASTNode{Type: NodeTypeFunctionCall, Value: strings.ToUpper(name)}

	if p.peek().Type != TokenRParen {
		for {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, arg)
			if p.peek().Type != TokenComma {
				break
			}
			p.advance()
		}
	}

	if _, err := p.expect(TokenRParen); err != nil {
		return nil, &CompileError{Message: "expected ) after function arguments", Position: p.peek().Position, Line: 1}
	}
	return node, nil
}

func (p *parser) parseFrom() (*ASTNode, error) {
	tok, err := p.expect(TokenIdentifier)
	if err != nil {
		return nil, &CompileError{Message: "expected table name after FROM", Position: p.peek().Position, Line: 1}
	}
	return &ASTNode{Type: NodeTypeFrom, Value: tok.Value}, nil
}

func (p *parser) parseWhere() (*ASTNode, error) {
	whereNode := &ASTNode{Type: NodeTypeWhere, Value: "WHERE"}

	cond, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	whereNode.Children = append(whereNode.Children, cond)

	// Handle AND/OR chains
	for p.peek().Type == TokenAnd || p.peek().Type == TokenOr {
		op := p.advance()
		right, err := p.parseCondition()
		if err != nil {
			return nil, err
		}
		cond = &ASTNode{
			Type:     NodeTypeBinaryOp,
			Value:    op.Value,
			Children: []*ASTNode{cond, right},
		}
		whereNode.Children = []*ASTNode{cond}
	}

	return whereNode, nil
}

func (p *parser) parseCondition() (*ASTNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	if p.peek().Type != TokenOperator {
		return left, nil
	}
	op := p.advance()
	right, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	return &ASTNode{
		Type:     NodeTypeBinaryOp,
		Value:    op.Value,
		Children: []*ASTNode{left, right},
	}, nil
}

func (p *parser) parseGroupBy() (*ASTNode, error) {
	groupNode := &ASTNode{Type: NodeTypeGroupBy, Value: "GROUP BY"}
	for {
		tok, err := p.expect(TokenIdentifier)
		if err != nil {
			return nil, &CompileError{Message: "expected column name in GROUP BY", Position: p.peek().Position, Line: 1}
		}
		groupNode.Children = append(groupNode.Children, &ASTNode{Type: NodeTypeColumnRef, Value: tok.Value})
		if p.peek().Type != TokenComma {
			break
		}
		p.advance()
	}
	return groupNode, nil
}

// Compile compiles an FTL query into a named pipeline.
func (c *Compiler) Compile(name, source string) (*Pipeline, error) {
	ast, err := c.Parse(source)
	if err != nil {
		c.mu.Lock()
		c.stats.ErrorCount++
		c.mu.Unlock()
		return nil, err
	}

	id := fmt.Sprintf("ftl-%s-%d", sanitizeID(name), time.Now().UnixNano())
	pipeline := &Pipeline{
		ID:        id,
		Name:      name,
		Source:    source,
		AST:       ast,
		Compiled:  true,
		CreatedAt: time.Now(),
	}

	c.mu.Lock()
	c.pipelines[id] = pipeline
	c.stats.PipelinesCompiled++
	c.mu.Unlock()

	return pipeline, nil
}

// Execute executes a compiled pipeline against the provided data.
func (c *Compiler) Execute(pipelineID string, data []map[string]interface{}) (*TransformResult, error) {
	c.mu.RLock()
	pipeline, ok := c.pipelines[pipelineID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", pipelineID)
	}

	start := time.Now()
	result, err := c.executeAST(pipeline.AST, data)
	elapsed := time.Since(start).Microseconds()

	if err != nil {
		c.mu.Lock()
		c.stats.ErrorCount++
		c.mu.Unlock()
		return nil, err
	}

	result.ExecutionTimeUs = elapsed

	c.mu.Lock()
	pipeline.ExecutionCount++
	totalLatency := pipeline.AvgLatencyUs*float64(pipeline.ExecutionCount-1) + float64(elapsed)
	pipeline.AvgLatencyUs = totalLatency / float64(pipeline.ExecutionCount)
	c.stats.PipelinesExecuted++
	c.stats.TotalTransformations++
	c.stats.AvgLatencyUs = (c.stats.AvgLatencyUs*float64(c.stats.TotalTransformations-1) + float64(elapsed)) / float64(c.stats.TotalTransformations)
	c.mu.Unlock()

	return result, nil
}

// ExecuteQuery parses and executes a query in one shot.
func (c *Compiler) ExecuteQuery(query string, data []map[string]interface{}) (*TransformResult, error) {
	ast, err := c.Parse(query)
	if err != nil {
		c.mu.Lock()
		c.stats.ErrorCount++
		c.mu.Unlock()
		return nil, err
	}

	start := time.Now()
	result, err := c.executeAST(ast, data)
	elapsed := time.Since(start).Microseconds()

	if err != nil {
		c.mu.Lock()
		c.stats.ErrorCount++
		c.mu.Unlock()
		return nil, err
	}

	result.ExecutionTimeUs = elapsed

	c.mu.Lock()
	c.stats.PipelinesExecuted++
	c.stats.TotalTransformations++
	c.stats.AvgLatencyUs = (c.stats.AvgLatencyUs*float64(c.stats.TotalTransformations-1) + float64(elapsed)) / float64(c.stats.TotalTransformations)
	c.mu.Unlock()

	return result, nil
}

// executeAST executes an AST node against in-memory data.
func (c *Compiler) executeAST(ast *ASTNode, data []map[string]interface{}) (*TransformResult, error) {
	if ast == nil {
		return nil, fmt.Errorf("nil AST")
	}
	if ast.Type != NodeTypeSelect {
		return nil, fmt.Errorf("expected SELECT node, got %s", ast.Type)
	}

	// Extract SELECT columns, FROM source, and WHERE filter from children
	var selectCols []*ASTNode
	var whereNode *ASTNode

	for _, child := range ast.Children {
		switch child.Type {
		case NodeTypeFrom:
			// Source table — just metadata, data is passed in
		case NodeTypeWhere:
			whereNode = child
		case NodeTypeGroupBy:
			// Simplified: ignore GROUP BY for now
		default:
			selectCols = append(selectCols, child)
		}
	}

	// Apply WHERE filter
	filtered := data
	if whereNode != nil && len(whereNode.Children) > 0 {
		filtered = c.applyFilter(data, whereNode.Children[0])
	}

	// Enforce row limit
	if c.config.MaxRowsPerTransform > 0 && len(filtered) > c.config.MaxRowsPerTransform {
		filtered = filtered[:c.config.MaxRowsPerTransform]
	}

	// Determine columns
	isSelectStar := len(selectCols) == 0 || (len(selectCols) == 1 && selectCols[0].Type == NodeTypeColumnRef && selectCols[0].Value == "*")

	var columns []string
	var rows []map[string]interface{}

	if isSelectStar {
		// SELECT * — return all columns
		if len(filtered) > 0 {
			for k := range filtered[0] {
				columns = append(columns, k)
			}
		}
		rows = filtered
	} else {
		// Project selected columns
		columns = make([]string, 0, len(selectCols))
		for _, col := range selectCols {
			columns = append(columns, resolveColumnName(col))
		}

		rows = make([]map[string]interface{}, 0, len(filtered))
		for _, row := range filtered {
			projected := make(map[string]interface{})
			for _, col := range selectCols {
				name := resolveColumnName(col)
				value := evaluateExpression(col, row)
				projected[name] = value
			}
			rows = append(rows, projected)
		}
	}

	return &TransformResult{
		Columns:  columns,
		Rows:     rows,
		RowCount: len(rows),
	}, nil
}

// applyFilter filters rows based on a WHERE condition AST node.
func (c *Compiler) applyFilter(data []map[string]interface{}, cond *ASTNode) []map[string]interface{} {
	var result []map[string]interface{}
	for _, row := range data {
		if evaluateCondition(cond, row) {
			result = append(result, row)
		}
	}
	return result
}

// evaluateCondition evaluates a condition node against a data row.
func evaluateCondition(node *ASTNode, row map[string]interface{}) bool {
	if node == nil {
		return true
	}

	if node.Type == NodeTypeBinaryOp && (node.Value == "AND" || node.Value == "OR") {
		if len(node.Children) != 2 {
			return false
		}
		left := evaluateCondition(node.Children[0], row)
		right := evaluateCondition(node.Children[1], row)
		if node.Value == "AND" {
			return left && right
		}
		return left || right
	}

	if node.Type == NodeTypeBinaryOp && len(node.Children) == 2 {
		leftVal := evaluateExpression(node.Children[0], row)
		rightVal := evaluateExpression(node.Children[1], row)
		return compareValues(leftVal, rightVal, node.Value)
	}

	return true
}

// compareValues compares two values using the given operator.
func compareValues(left, right interface{}, op string) bool {
	leftF, leftOK := toFloat64(left)
	rightF, rightOK := toFloat64(right)

	if leftOK && rightOK {
		switch op {
		case "=", "==":
			return leftF == rightF
		case "!=", "<>":
			return leftF != rightF
		case ">":
			return leftF > rightF
		case ">=":
			return leftF >= rightF
		case "<":
			return leftF < rightF
		case "<=":
			return leftF <= rightF
		}
	}

	// Fallback to string comparison
	leftS := fmt.Sprintf("%v", left)
	rightS := fmt.Sprintf("%v", right)
	switch op {
	case "=", "==":
		return leftS == rightS
	case "!=", "<>":
		return leftS != rightS
	}

	return false
}

// evaluateExpression evaluates an expression node against a data row.
func evaluateExpression(node *ASTNode, row map[string]interface{}) interface{} {
	if node == nil {
		return nil
	}

	switch node.Type {
	case NodeTypeColumnRef:
		return row[node.Value]
	case NodeTypeLiteral:
		if f, err := strconv.ParseFloat(node.Value, 64); err == nil {
			return f
		}
		return node.Value
	case NodeTypeAlias:
		if len(node.Children) > 0 {
			return evaluateExpression(node.Children[0], row)
		}
		return nil
	case NodeTypeBinaryOp:
		if len(node.Children) == 2 {
			left := evaluateExpression(node.Children[0], row)
			right := evaluateExpression(node.Children[1], row)
			return evaluateBinaryOp(left, right, node.Value)
		}
		return nil
	case NodeTypeFunctionCall:
		// Aggregate functions are not applied per-row in simple mode
		if len(node.Children) > 0 {
			return evaluateExpression(node.Children[0], row)
		}
		return nil
	default:
		return nil
	}
}

func evaluateBinaryOp(left, right interface{}, op string) interface{} {
	leftF, leftOK := toFloat64(left)
	rightF, rightOK := toFloat64(right)
	if leftOK && rightOK {
		switch op {
		case "+":
			return leftF + rightF
		case "-":
			return leftF - rightF
		case "*":
			return leftF * rightF
		case "/":
			if rightF == 0 {
				return nil
			}
			return leftF / rightF
		}
	}
	return nil
}

// resolveColumnName returns the output column name for an AST node.
func resolveColumnName(node *ASTNode) string {
	switch node.Type {
	case NodeTypeAlias:
		return node.Value
	case NodeTypeColumnRef:
		return node.Value
	case NodeTypeFunctionCall:
		if len(node.Children) > 0 {
			return strings.ToLower(node.Value) + "_" + resolveColumnName(node.Children[0])
		}
		return strings.ToLower(node.Value)
	case NodeTypeBinaryOp:
		if len(node.Children) == 2 {
			return resolveColumnName(node.Children[0]) + node.Value + resolveColumnName(node.Children[1])
		}
		return "expr"
	default:
		return node.Value
	}
}

// GetPipeline retrieves a compiled pipeline by ID.
func (c *Compiler) GetPipeline(id string) (*Pipeline, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	p, ok := c.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q not found", id)
	}
	return p, nil
}

// ListPipelines returns all compiled pipelines.
func (c *Compiler) ListPipelines() []*Pipeline {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Pipeline, 0, len(c.pipelines))
	for _, p := range c.pipelines {
		result = append(result, p)
	}
	return result
}

// DeletePipeline removes a compiled pipeline.
func (c *Compiler) DeletePipeline(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.pipelines[id]; !ok {
		return fmt.Errorf("pipeline %q not found", id)
	}
	delete(c.pipelines, id)
	return nil
}

// Stats returns current compiler statistics.
func (c *Compiler) Stats() *FTLStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.stats
	return &s
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	case nil:
		return 0, false
	default:
		return 0, false
	}
}

func isDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func isLetter(ch byte) bool { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }

func sanitizeID(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
