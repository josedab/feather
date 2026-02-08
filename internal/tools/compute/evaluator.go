package compute

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Evaluator evaluates feature computation expressions.
type Evaluator struct {
	functions map[string]ComputeFunc
}

// ComputeFunc is a function that can be called within an expression.
type ComputeFunc func(args ...interface{}) (interface{}, error)

// NewEvaluator creates a new expression evaluator with built-in functions.
func NewEvaluator() *Evaluator {
	e := &Evaluator{
		functions: make(map[string]ComputeFunc),
	}
	e.registerBuiltins()
	return e
}

// RegisterFunction registers a custom function.
func (e *Evaluator) RegisterFunction(name string, fn ComputeFunc) {
	e.functions[name] = fn
}

func (e *Evaluator) registerBuiltins() {
	e.functions["abs"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("abs: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("abs: %w", err)
		}
		return math.Abs(v), nil
	}

	e.functions["ceil"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("ceil: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("ceil: %w", err)
		}
		return math.Ceil(v), nil
	}

	e.functions["floor"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("floor: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("floor: %w", err)
		}
		return math.Floor(v), nil
	}

	e.functions["round"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("round: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("round: %w", err)
		}
		return math.Round(v), nil
	}

	e.functions["min"] = func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("min: expected at least 2 arguments, got %d", len(args))
		}
		result, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("min: %w", err)
		}
		for _, a := range args[1:] {
			v, err := toFloat(a)
			if err != nil {
				return nil, fmt.Errorf("min: %w", err)
			}
			if v < result {
				result = v
			}
		}
		return result, nil
	}

	e.functions["max"] = func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("max: expected at least 2 arguments, got %d", len(args))
		}
		result, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("max: %w", err)
		}
		for _, a := range args[1:] {
			v, err := toFloat(a)
			if err != nil {
				return nil, fmt.Errorf("max: %w", err)
			}
			if v > result {
				result = v
			}
		}
		return result, nil
	}

	e.functions["coalesce"] = func(args ...interface{}) (interface{}, error) {
		for _, a := range args {
			if a != nil {
				return a, nil
			}
		}
		return nil, nil
	}

	e.functions["if_then_else"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("if_then_else: expected 3 arguments, got %d", len(args))
		}
		cond, err := toBool(args[0])
		if err != nil {
			return nil, fmt.Errorf("if_then_else: %w", err)
		}
		if cond {
			return args[1], nil
		}
		return args[2], nil
	}

	e.functions["concat"] = func(args ...interface{}) (interface{}, error) {
		var sb strings.Builder
		for _, a := range args {
			sb.WriteString(fmt.Sprintf("%v", a))
		}
		return sb.String(), nil
	}

	e.functions["lower"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("lower: expected 1 argument, got %d", len(args))
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("lower: expected string argument")
		}
		return strings.ToLower(s), nil
	}

	e.functions["upper"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("upper: expected 1 argument, got %d", len(args))
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("upper: expected string argument")
		}
		return strings.ToUpper(s), nil
	}

	e.functions["len"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("len: expected 1 argument, got %d", len(args))
		}
		switch v := args[0].(type) {
		case string:
			return float64(len(v)), nil
		case []interface{}:
			return float64(len(v)), nil
		default:
			return nil, fmt.Errorf("len: unsupported type %T", args[0])
		}
	}

	e.functions["contains"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("contains: expected 2 arguments, got %d", len(args))
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("contains: expected string arguments")
		}
		substr, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("contains: expected string arguments")
		}
		return strings.Contains(s, substr), nil
	}

	e.functions["log"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("log: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("log: %w", err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("log: argument must be positive")
		}
		return math.Log(v), nil
	}

	e.functions["exp"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("exp: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("exp: %w", err)
		}
		return math.Exp(v), nil
	}

	e.functions["sqrt"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sqrt: expected 1 argument, got %d", len(args))
		}
		v, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("sqrt: %w", err)
		}
		if v < 0 {
			return nil, fmt.Errorf("sqrt: argument must be non-negative")
		}
		return math.Sqrt(v), nil
	}

	e.functions["pow"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("pow: expected 2 arguments, got %d", len(args))
		}
		base, err := toFloat(args[0])
		if err != nil {
			return nil, fmt.Errorf("pow: %w", err)
		}
		exp, err := toFloat(args[1])
		if err != nil {
			return nil, fmt.Errorf("pow: %w", err)
		}
		return math.Pow(base, exp), nil
	}
}

// Evaluate evaluates an expression with the given variable bindings.
func (e *Evaluator) Evaluate(expression string, vars map[string]interface{}) (interface{}, error) {
	p := &parser{
		evaluator: e,
		vars:      vars,
		input:     expression,
		pos:       0,
	}
	result, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("evaluating expression %q: %w", expression, err)
	}
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("evaluating expression %q: unexpected character at position %d", expression, p.pos)
	}
	return result, nil
}

// parser is a recursive-descent parser for expressions.
type parser struct {
	evaluator *Evaluator
	vars      map[string]interface{}
	input     string
	pos       int
}

func (p *parser) parseExpression() (interface{}, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (interface{}, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchStr("||") {
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			lb, err := toBool(left)
			if err != nil {
				return nil, fmt.Errorf("|| operator: %w", err)
			}
			rb, err := toBool(right)
			if err != nil {
				return nil, fmt.Errorf("|| operator: %w", err)
			}
			left = lb || rb
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseAnd() (interface{}, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchStr("&&") {
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			lb, err := toBool(left)
			if err != nil {
				return nil, fmt.Errorf("&& operator: %w", err)
			}
			rb, err := toBool(right)
			if err != nil {
				return nil, fmt.Errorf("&& operator: %w", err)
			}
			left = lb && rb
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseComparison() (interface{}, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	// Order matters: check two-char ops before single-char ones.
	if p.matchStr(">=") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		lf, err := toFloat(left)
		if err != nil {
			return nil, err
		}
		rf, err := toFloat(right)
		if err != nil {
			return nil, err
		}
		return lf >= rf, nil
	}
	if p.matchStr("<=") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		lf, err := toFloat(left)
		if err != nil {
			return nil, err
		}
		rf, err := toFloat(right)
		if err != nil {
			return nil, err
		}
		return lf <= rf, nil
	}
	if p.matchStr("==") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right), nil
	}
	if p.matchStr("!=") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right), nil
	}
	if p.matchStr(">") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		lf, err := toFloat(left)
		if err != nil {
			return nil, err
		}
		rf, err := toFloat(right)
		if err != nil {
			return nil, err
		}
		return lf > rf, nil
	}
	if p.matchStr("<") {
		right, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		lf, err := toFloat(left)
		if err != nil {
			return nil, err
		}
		rf, err := toFloat(right)
		if err != nil {
			return nil, err
		}
		return lf < rf, nil
	}

	return left, nil
}

func (p *parser) parseAddSub() (interface{}, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchByte('+') {
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			lf, err := toFloat(left)
			if err != nil {
				return nil, err
			}
			rf, err := toFloat(right)
			if err != nil {
				return nil, err
			}
			left = lf + rf
		} else if p.matchByte('-') {
			right, err := p.parseMulDiv()
			if err != nil {
				return nil, err
			}
			lf, err := toFloat(left)
			if err != nil {
				return nil, err
			}
			rf, err := toFloat(right)
			if err != nil {
				return nil, err
			}
			left = lf - rf
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseMulDiv() (interface{}, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchByte('*') {
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			lf, err := toFloat(left)
			if err != nil {
				return nil, err
			}
			rf, err := toFloat(right)
			if err != nil {
				return nil, err
			}
			left = lf * rf
		} else if p.matchByte('/') {
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			lf, err := toFloat(left)
			if err != nil {
				return nil, err
			}
			rf, err := toFloat(right)
			if err != nil {
				return nil, err
			}
			if rf == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			left = lf / rf
		} else {
			break
		}
	}
	return left, nil
}

func (p *parser) parseUnary() (interface{}, error) {
	p.skipWhitespace()
	if p.matchByte('-') {
		val, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		f, err := toFloat(val)
		if err != nil {
			return nil, err
		}
		return -f, nil
	}
	if p.matchByte('!') {
		val, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		b, err := toBool(val)
		if err != nil {
			return nil, err
		}
		return !b, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (interface{}, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	// Parenthesized expression
	if p.peekByte() == '(' {
		p.pos++
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		p.pos++
		return val, nil
	}

	// String literal
	if p.peekByte() == '"' {
		return p.parseString()
	}

	// Number literal
	if isDigit(p.peekByte()) || (p.peekByte() == '.' && p.pos+1 < len(p.input) && isDigit(p.input[p.pos+1])) {
		return p.parseNumber()
	}

	// Identifier (variable or function call) or boolean literal
	if isIdentStart(p.peekByte()) {
		name := p.parseIdentifier()

		// Boolean literals
		if name == "true" {
			return true, nil
		}
		if name == "false" {
			return false, nil
		}

		p.skipWhitespace()

		// Function call
		if p.pos < len(p.input) && p.peekByte() == '(' {
			return p.parseFunctionCall(name)
		}

		// Variable lookup
		if val, ok := p.vars[name]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("undefined variable: %s", name)
	}

	return nil, fmt.Errorf("unexpected character: %c", p.peekByte())
}

func (p *parser) parseFunctionCall(name string) (interface{}, error) {
	p.pos++ // consume '('
	var args []interface{}

	p.skipWhitespace()
	if p.pos < len(p.input) && p.peekByte() != ')' {
		for {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)

			p.skipWhitespace()
			if p.pos >= len(p.input) {
				return nil, fmt.Errorf("expected closing parenthesis in function call")
			}
			if p.peekByte() == ')' {
				break
			}
			if p.peekByte() != ',' {
				return nil, fmt.Errorf("expected ',' or ')' in function arguments")
			}
			p.pos++ // consume ','
		}
	}

	if p.pos >= len(p.input) || p.peekByte() != ')' {
		return nil, fmt.Errorf("expected closing parenthesis")
	}
	p.pos++ // consume ')'

	fn, ok := p.evaluator.functions[name]
	if !ok {
		return nil, fmt.Errorf("undefined function: %s", name)
	}
	return fn(args...)
}

func (p *parser) parseString() (interface{}, error) {
	p.pos++ // consume opening quote
	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '"' {
			p.pos++
			return sb.String(), nil
		}
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			switch p.input[p.pos] {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			default:
				sb.WriteByte(p.input[p.pos])
			}
		} else {
			sb.WriteByte(ch)
		}
		p.pos++
	}
	return nil, fmt.Errorf("unterminated string literal")
}

func (p *parser) parseNumber() (interface{}, error) {
	start := p.pos
	for p.pos < len(p.input) && (isDigit(p.input[p.pos]) || p.input[p.pos] == '.') {
		p.pos++
	}
	// Handle scientific notation
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		for p.pos < len(p.input) && isDigit(p.input[p.pos]) {
			p.pos++
		}
	}

	val, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number: %s", p.input[start:p.pos])
	}
	return val, nil
}

func (p *parser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) && isIdentPart(p.input[p.pos]) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n' || p.input[p.pos] == '\r') {
		p.pos++
	}
}

func (p *parser) peekByte() byte {
	return p.input[p.pos]
}

func (p *parser) matchByte(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *parser) matchStr(s string) bool {
	if p.pos+len(s) <= len(p.input) && p.input[p.pos:p.pos+len(s)] == s {
		// Make sure we don't match partial tokens (e.g., ">" in ">=")
		p.pos += len(s)
		return true
	}
	return false
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isIdentPart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_'
}

// toFloat converts a value to float64.
func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// toBool converts a value to bool.
func toBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case float64:
		return val != 0, nil
	case int:
		return val != 0, nil
	case int64:
		return val != 0, nil
	case string:
		return val != "", nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}
