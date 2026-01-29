package pushdown

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidExpression = errors.New("invalid expression")
	ErrDivisionByZero    = errors.New("division by zero")
	ErrUnknownFunction   = errors.New("unknown function")
	ErrTypeMismatch      = errors.New("type mismatch in expression")
	ErrFeatureNotFound   = errors.New("feature not found in context")
)

// Operator represents an arithmetic operator.
type Operator string

const (
	OpAdd Operator = "+"
	OpSub Operator = "-"
	OpMul Operator = "*"
	OpDiv Operator = "/"
)

// FuncType identifies built-in functions.
type FuncType string

const (
	FuncAbs      FuncType = "abs"
	FuncLog      FuncType = "log"
	FuncSqrt     FuncType = "sqrt"
	FuncRound    FuncType = "round"
	FuncMin      FuncType = "min"
	FuncMax      FuncType = "max"
	FuncIf       FuncType = "if"
	FuncCoalesce FuncType = "coalesce"
)

// ExprNode represents a node in the expression AST.
type ExprNode struct {
	// Type identifies the node kind.
	Type string // "literal", "feature", "binary", "function"
	// Literal value for constants.
	LiteralValue float64
	// Feature name for feature references.
	FeatureName string
	// Op for binary operations.
	Op Operator
	// Left and Right children for binary ops.
	Left  *ExprNode
	Right *ExprNode
	// Func for function calls.
	Func FuncType
	// Args for function arguments.
	Args []*ExprNode
}

// DerivedFeature defines a computed feature using expressions.
type DerivedFeature struct {
	// Name is the output feature name.
	Name string `json:"name"`
	// Expression is the computation expression string.
	Expression string `json:"expression"`
	// Inputs are the features required for computation.
	Inputs []string `json:"inputs"`
	// Description explains the derived feature.
	Description string `json:"description,omitempty"`
	// CacheTTL is how long to cache the computed result.
	CacheTTL time.Duration `json:"cache_ttl,omitempty"`
	// parsed is the compiled expression tree.
	parsed *ExprNode
}

// CachedResult stores a computed result with expiry.
type CachedResult struct {
	Value      float64
	ComputedAt time.Time
	ExpiresAt  time.Time
}

// Evaluator evaluates pushdown expressions against feature contexts.
type Evaluator struct {
	mu       sync.RWMutex
	features map[string]*DerivedFeature
	cache    map[string]*CachedResult // "entity:feature" -> result
}

// NewEvaluator creates a new pushdown evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		features: make(map[string]*DerivedFeature),
		cache:    make(map[string]*CachedResult),
	}
}

// RegisterDerived registers a derived feature definition.
func (e *Evaluator) RegisterDerived(df *DerivedFeature) error {
	if df.Name == "" || df.Expression == "" {
		return fmt.Errorf("%w: name and expression are required", ErrInvalidExpression)
	}

	parsed, err := parseExpression(df.Expression)
	if err != nil {
		return err
	}
	df.parsed = parsed
	df.Inputs = extractFeatureRefs(parsed)

	e.mu.Lock()
	defer e.mu.Unlock()
	e.features[df.Name] = df
	return nil
}

// UnregisterDerived removes a derived feature.
func (e *Evaluator) UnregisterDerived(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.features, name)
}

// ListDerived returns all registered derived features.
func (e *Evaluator) ListDerived() []*DerivedFeature {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*DerivedFeature, 0, len(e.features))
	for _, df := range e.features {
		result = append(result, df)
	}
	return result
}

// GetDerived retrieves a derived feature by name.
func (e *Evaluator) GetDerived(name string) (*DerivedFeature, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	df, ok := e.features[name]
	if !ok {
		return nil, ErrFeatureNotFound
	}
	return df, nil
}

// Evaluate computes a derived feature given a context of feature values.
func (e *Evaluator) Evaluate(entity, featureName string, ctx map[string]float64) (float64, error) {
	e.mu.RLock()
	df, ok := e.features[featureName]
	e.mu.RUnlock()

	if !ok {
		return 0, ErrFeatureNotFound
	}

	// Check cache
	cacheKey := entity + ":" + featureName
	if df.CacheTTL > 0 {
		e.mu.RLock()
		cached, hasCached := e.cache[cacheKey]
		e.mu.RUnlock()
		if hasCached && time.Now().Before(cached.ExpiresAt) {
			return cached.Value, nil
		}
	}

	result, err := evalNode(df.parsed, ctx)
	if err != nil {
		return 0, err
	}

	// Cache result
	if df.CacheTTL > 0 {
		e.mu.Lock()
		e.cache[cacheKey] = &CachedResult{
			Value:      result,
			ComputedAt: time.Now(),
			ExpiresAt:  time.Now().Add(df.CacheTTL),
		}
		e.mu.Unlock()
	}

	return result, nil
}

// EvaluateExpression evaluates an ad-hoc expression without registration.
func (e *Evaluator) EvaluateExpression(expr string, ctx map[string]float64) (float64, error) {
	parsed, err := parseExpression(expr)
	if err != nil {
		return 0, err
	}
	return evalNode(parsed, ctx)
}

func evalNode(node *ExprNode, ctx map[string]float64) (float64, error) {
	if node == nil {
		return 0, ErrInvalidExpression
	}

	switch node.Type {
	case "literal":
		return node.LiteralValue, nil

	case "feature":
		val, ok := ctx[node.FeatureName]
		if !ok {
			return 0, fmt.Errorf("%w: %s", ErrFeatureNotFound, node.FeatureName)
		}
		return val, nil

	case "binary":
		left, err := evalNode(node.Left, ctx)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(node.Right, ctx)
		if err != nil {
			return 0, err
		}

		switch node.Op {
		case OpAdd:
			return left + right, nil
		case OpSub:
			return left - right, nil
		case OpMul:
			return left * right, nil
		case OpDiv:
			if right == 0 {
				return 0, ErrDivisionByZero
			}
			return left / right, nil
		default:
			return 0, fmt.Errorf("%w: unknown operator %s", ErrInvalidExpression, node.Op)
		}

	case "function":
		return evalFunction(node, ctx)

	default:
		return 0, fmt.Errorf("%w: unknown node type %s", ErrInvalidExpression, node.Type)
	}
}

func evalFunction(node *ExprNode, ctx map[string]float64) (float64, error) {
	args := make([]float64, 0, len(node.Args))
	for _, arg := range node.Args {
		val, err := evalNode(arg, ctx)
		if err != nil {
			return 0, err
		}
		args = append(args, val)
	}

	switch node.Func {
	case FuncAbs:
		if len(args) != 1 {
			return 0, fmt.Errorf("%w: abs requires 1 argument", ErrInvalidExpression)
		}
		return math.Abs(args[0]), nil
	case FuncLog:
		if len(args) != 1 {
			return 0, fmt.Errorf("%w: log requires 1 argument", ErrInvalidExpression)
		}
		if args[0] <= 0 {
			return 0, fmt.Errorf("%w: log of non-positive number", ErrInvalidExpression)
		}
		return math.Log(args[0]), nil
	case FuncSqrt:
		if len(args) != 1 {
			return 0, fmt.Errorf("%w: sqrt requires 1 argument", ErrInvalidExpression)
		}
		return math.Sqrt(args[0]), nil
	case FuncRound:
		if len(args) != 1 {
			return 0, fmt.Errorf("%w: round requires 1 argument", ErrInvalidExpression)
		}
		return math.Round(args[0]), nil
	case FuncMin:
		if len(args) < 2 {
			return 0, fmt.Errorf("%w: min requires at least 2 arguments", ErrInvalidExpression)
		}
		result := args[0]
		for _, a := range args[1:] {
			if a < result {
				result = a
			}
		}
		return result, nil
	case FuncMax:
		if len(args) < 2 {
			return 0, fmt.Errorf("%w: max requires at least 2 arguments", ErrInvalidExpression)
		}
		result := args[0]
		for _, a := range args[1:] {
			if a > result {
				result = a
			}
		}
		return result, nil
	case FuncCoalesce:
		for _, a := range args {
			if a != 0 {
				return a, nil
			}
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrUnknownFunction, node.Func)
	}
}

// parseExpression is a simple recursive-descent parser for expressions.
// Supports: numbers, feature refs ($name), binary ops (+,-,*,/), functions.
func parseExpression(expr string) (*ExprNode, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, ErrInvalidExpression
	}

	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return nil, ErrInvalidExpression
	}

	node, remaining, err := parseAddSub(tokens)
	if err != nil {
		return nil, err
	}
	if len(remaining) > 0 {
		return nil, fmt.Errorf("%w: unexpected tokens: %v", ErrInvalidExpression, remaining)
	}
	return node, nil
}

func parseAddSub(tokens []string) (*ExprNode, []string, error) {
	left, rest, err := parseMulDiv(tokens)
	if err != nil {
		return nil, nil, err
	}

	for len(rest) > 0 && (rest[0] == "+" || rest[0] == "-") {
		op := Operator(rest[0])
		right, newRest, err := parseMulDiv(rest[1:])
		if err != nil {
			return nil, nil, err
		}
		left = &ExprNode{Type: "binary", Op: op, Left: left, Right: right}
		rest = newRest
	}
	return left, rest, nil
}

func parseMulDiv(tokens []string) (*ExprNode, []string, error) {
	left, rest, err := parseAtom(tokens)
	if err != nil {
		return nil, nil, err
	}

	for len(rest) > 0 && (rest[0] == "*" || rest[0] == "/") {
		op := Operator(rest[0])
		right, newRest, err := parseAtom(rest[1:])
		if err != nil {
			return nil, nil, err
		}
		left = &ExprNode{Type: "binary", Op: op, Left: left, Right: right}
		rest = newRest
	}
	return left, rest, nil
}

func parseAtom(tokens []string) (*ExprNode, []string, error) {
	if len(tokens) == 0 {
		return nil, nil, ErrInvalidExpression
	}

	tok := tokens[0]

	// Parenthesized expression
	if tok == "(" {
		node, rest, err := parseAddSub(tokens[1:])
		if err != nil {
			return nil, nil, err
		}
		if len(rest) == 0 || rest[0] != ")" {
			return nil, nil, fmt.Errorf("%w: missing closing paren", ErrInvalidExpression)
		}
		return node, rest[1:], nil
	}

	// Function call: name(args...)
	if len(tokens) > 1 && tokens[1] == "(" {
		funcName := FuncType(tok)
		args, rest, err := parseFuncArgs(tokens[2:])
		if err != nil {
			return nil, nil, err
		}
		return &ExprNode{Type: "function", Func: funcName, Args: args}, rest, nil
	}

	// Feature reference: $name
	if strings.HasPrefix(tok, "$") {
		return &ExprNode{Type: "feature", FeatureName: tok[1:]}, tokens[1:], nil
	}

	// Numeric literal
	val, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		// Treat as feature reference without $
		return &ExprNode{Type: "feature", FeatureName: tok}, tokens[1:], nil
	}
	return &ExprNode{Type: "literal", LiteralValue: val}, tokens[1:], nil
}

func parseFuncArgs(tokens []string) ([]*ExprNode, []string, error) {
	var args []*ExprNode

	if len(tokens) > 0 && tokens[0] == ")" {
		return args, tokens[1:], nil
	}

	for {
		arg, rest, err := parseAddSub(tokens)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, arg)
		tokens = rest

		if len(tokens) == 0 {
			return nil, nil, fmt.Errorf("%w: missing closing paren in function", ErrInvalidExpression)
		}
		if tokens[0] == ")" {
			return args, tokens[1:], nil
		}
		if tokens[0] == "," {
			tokens = tokens[1:]
			continue
		}
		return nil, nil, fmt.Errorf("%w: unexpected token in function args: %s", ErrInvalidExpression, tokens[0])
	}
}

func tokenize(expr string) []string {
	var tokens []string
	var current strings.Builder

	for _, ch := range expr {
		switch {
		case ch == ' ' || ch == '\t':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '(' || ch == ')' || ch == ',':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(ch))
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func extractFeatureRefs(node *ExprNode) []string {
	if node == nil {
		return nil
	}

	var refs []string
	if node.Type == "feature" {
		refs = append(refs, node.FeatureName)
	}
	refs = append(refs, extractFeatureRefs(node.Left)...)
	refs = append(refs, extractFeatureRefs(node.Right)...)
	for _, arg := range node.Args {
		refs = append(refs, extractFeatureRefs(arg)...)
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			unique = append(unique, r)
		}
	}
	return unique
}
