package expr

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// Func represents a built-in function.
type Func func(args []interface{}) (interface{}, error)

// Evaluator evaluates parsed expressions.
type Evaluator struct {
	variables map[string]interface{}
	functions map[string]Func
}

// NewEvaluator creates a new evaluator with the given variables.
func NewEvaluator(variables map[string]interface{}) *Evaluator {
	e := &Evaluator{
		variables: variables,
		functions: make(map[string]Func),
	}
	e.registerBuiltins()
	return e
}

// RegisterFunction registers a custom function.
func (e *Evaluator) RegisterFunction(name string, fn Func) {
	e.functions[strings.ToLower(name)] = fn
}

// SetVariable sets a variable value.
func (e *Evaluator) SetVariable(name string, value interface{}) {
	e.variables[name] = value
}

func (e *Evaluator) registerBuiltins() {
	// Math functions
	e.functions["abs"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("abs requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("abs requires numeric argument")
		}
		return math.Abs(v), nil
	}

	e.functions["ceil"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("ceil requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("ceil requires numeric argument")
		}
		return math.Ceil(v), nil
	}

	e.functions["floor"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("floor requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("floor requires numeric argument")
		}
		return math.Floor(v), nil
	}

	e.functions["round"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("round requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("round requires numeric argument")
		}
		return math.Round(v), nil
	}

	e.functions["sqrt"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sqrt requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("sqrt requires numeric argument")
		}
		return math.Sqrt(v), nil
	}

	e.functions["pow"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("pow requires 2 arguments")
		}
		base, ok1 := toFloat(args[0])
		exp, ok2 := toFloat(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("pow requires numeric arguments")
		}
		return math.Pow(base, exp), nil
	}

	e.functions["log"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("log requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("log requires numeric argument")
		}
		return math.Log(v), nil
	}

	e.functions["log10"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("log10 requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("log10 requires numeric argument")
		}
		return math.Log10(v), nil
	}

	e.functions["exp"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("exp requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("exp requires numeric argument")
		}
		return math.Exp(v), nil
	}

	e.functions["min"] = func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("min requires at least 1 argument")
		}
		minVal, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("min requires numeric arguments")
		}
		for _, arg := range args[1:] {
			v, ok := toFloat(arg)
			if !ok {
				continue
			}
			if v < minVal {
				minVal = v
			}
		}
		return minVal, nil
	}

	e.functions["max"] = func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("max requires at least 1 argument")
		}
		maxVal, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("max requires numeric arguments")
		}
		for _, arg := range args[1:] {
			v, ok := toFloat(arg)
			if !ok {
				continue
			}
			if v > maxVal {
				maxVal = v
			}
		}
		return maxVal, nil
	}

	// Aggregation functions
	e.functions["sum"] = func(args []interface{}) (interface{}, error) {
		sum := 0.0
		for _, arg := range args {
			if v, ok := toFloat(arg); ok {
				sum += v
			} else if arr := toArray(arg); arr != nil {
				for _, item := range arr {
					if v, ok := toFloat(item); ok {
						sum += v
					}
				}
			}
		}
		return sum, nil
	}

	e.functions["avg"] = func(args []interface{}) (interface{}, error) {
		sum := 0.0
		count := 0
		for _, arg := range args {
			if v, ok := toFloat(arg); ok {
				sum += v
				count++
			} else if arr := toArray(arg); arr != nil {
				for _, item := range arr {
					if v, ok := toFloat(item); ok {
						sum += v
						count++
					}
				}
			}
		}
		if count == 0 {
			return 0.0, nil
		}
		return sum / float64(count), nil
	}

	e.functions["count"] = func(args []interface{}) (interface{}, error) {
		count := 0
		for _, arg := range args {
			if arr := toArray(arg); arr != nil {
				count += len(arr)
			} else {
				count++
			}
		}
		return float64(count), nil
	}

	// String functions
	e.functions["len"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("len requires 1 argument")
		}
		switch v := args[0].(type) {
		case string:
			return float64(len(v)), nil
		case []interface{}:
			return float64(len(v)), nil
		case map[string]interface{}:
			return float64(len(v)), nil
		default:
			return 0.0, nil
		}
	}

	e.functions["lower"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("lower requires 1 argument")
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("lower requires string argument")
		}
		return strings.ToLower(s), nil
	}

	e.functions["upper"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("upper requires 1 argument")
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("upper requires string argument")
		}
		return strings.ToUpper(s), nil
	}

	e.functions["concat"] = func(args []interface{}) (interface{}, error) {
		var result strings.Builder
		for _, arg := range args {
			result.WriteString(fmt.Sprintf("%v", arg))
		}
		return result.String(), nil
	}

	e.functions["substr"] = func(args []interface{}) (interface{}, error) {
		if len(args) < 2 || len(args) > 3 {
			return nil, fmt.Errorf("substr requires 2-3 arguments")
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("substr requires string first argument")
		}
		start, ok := toFloat(args[1])
		if !ok {
			return nil, fmt.Errorf("substr requires numeric start index")
		}
		startIdx := int(start)
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(s) {
			return "", nil
		}
		if len(args) == 3 {
			length, ok := toFloat(args[2])
			if !ok {
				return nil, fmt.Errorf("substr requires numeric length")
			}
			endIdx := startIdx + int(length)
			if endIdx > len(s) {
				endIdx = len(s)
			}
			return s[startIdx:endIdx], nil
		}
		return s[startIdx:], nil
	}

	// Conditional functions
	e.functions["if"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("if requires 3 arguments")
		}
		cond := isTruthy(args[0])
		if cond {
			return args[1], nil
		}
		return args[2], nil
	}

	e.functions["coalesce"] = func(args []interface{}) (interface{}, error) {
		for _, arg := range args {
			if arg != nil {
				return arg, nil
			}
		}
		return nil, nil
	}

	// Type conversion functions
	e.functions["float"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("float requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return 0.0, nil
		}
		return v, nil
	}

	e.functions["int"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("int requires 1 argument")
		}
		v, ok := toFloat(args[0])
		if !ok {
			return 0.0, nil
		}
		return float64(int64(v)), nil
	}

	e.functions["str"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("str requires 1 argument")
		}
		return fmt.Sprintf("%v", args[0]), nil
	}

	e.functions["bool"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("bool requires 1 argument")
		}
		if isTruthy(args[0]) {
			return 1.0, nil
		}
		return 0.0, nil
	}

	// Feature-specific functions
	e.functions["zscore"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("zscore requires 3 arguments (value, mean, stddev)")
		}
		value, ok1 := toFloat(args[0])
		mean, ok2 := toFloat(args[1])
		stddev, ok3 := toFloat(args[2])
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("zscore requires numeric arguments")
		}
		if stddev == 0 {
			return 0.0, nil
		}
		return (value - mean) / stddev, nil
	}

	e.functions["normalize"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("normalize requires 3 arguments (value, min, max)")
		}
		value, ok1 := toFloat(args[0])
		minVal, ok2 := toFloat(args[1])
		maxVal, ok3 := toFloat(args[2])
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("normalize requires numeric arguments")
		}
		if maxVal == minVal {
			return 0.5, nil
		}
		return (value - minVal) / (maxVal - minVal), nil
	}

	e.functions["clip"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 3 {
			return nil, fmt.Errorf("clip requires 3 arguments (value, min, max)")
		}
		value, ok1 := toFloat(args[0])
		minVal, ok2 := toFloat(args[1])
		maxVal, ok3 := toFloat(args[2])
		if !ok1 || !ok2 || !ok3 {
			return nil, fmt.Errorf("clip requires numeric arguments")
		}
		if value < minVal {
			return minVal, nil
		}
		if value > maxVal {
			return maxVal, nil
		}
		return value, nil
	}

	e.functions["sigmoid"] = func(args []interface{}) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("sigmoid requires 1 argument")
		}
		x, ok := toFloat(args[0])
		if !ok {
			return nil, fmt.Errorf("sigmoid requires numeric argument")
		}
		return 1.0 / (1.0 + math.Exp(-x)), nil
	}

	e.functions["softmax"] = func(args []interface{}) (interface{}, error) {
		if len(args) == 0 {
			return []float64{}, nil
		}
		var values []float64
		for _, arg := range args {
			if v, ok := toFloat(arg); ok {
				values = append(values, v)
			} else if arr := toArray(arg); arr != nil {
				for _, item := range arr {
					if v, ok := toFloat(item); ok {
						values = append(values, v)
					}
				}
			}
		}
		if len(values) == 0 {
			return []float64{}, nil
		}
		// Find max for numerical stability
		maxVal := values[0]
		for _, v := range values[1:] {
			if v > maxVal {
				maxVal = v
			}
		}
		// Compute softmax
		var sum float64
		result := make([]float64, len(values))
		for i, v := range values {
			result[i] = math.Exp(v - maxVal)
			sum += result[i]
		}
		for i := range result {
			result[i] /= sum
		}
		return result, nil
	}
}

// Eval evaluates the given AST node.
func (e *Evaluator) Eval(node Node) (interface{}, error) {
	switch n := node.(type) {
	case *NumberNode:
		return n.Value, nil

	case *StringNode:
		return n.Value, nil

	case *IdentNode:
		if n.Name == "null" {
			return nil, nil
		}
		if val, ok := e.variables[n.Name]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("undefined variable: %s", n.Name)

	case *BinaryNode:
		return e.evalBinary(n)

	case *UnaryNode:
		return e.evalUnary(n)

	case *CallNode:
		return e.evalCall(n)

	case *IndexNode:
		return e.evalIndex(n)

	case *TernaryNode:
		cond, err := e.Eval(n.Condition)
		if err != nil {
			return nil, err
		}
		if isTruthy(cond) {
			return e.Eval(n.Then)
		}
		return e.Eval(n.Else)

	case *PropertyNode:
		return e.evalProperty(n)

	default:
		return nil, fmt.Errorf("unknown node type: %T", node)
	}
}

func (e *Evaluator) evalBinary(n *BinaryNode) (interface{}, error) {
	left, err := e.Eval(n.Left)
	if err != nil {
		return nil, err
	}
	right, err := e.Eval(n.Right)
	if err != nil {
		return nil, err
	}

	// String concatenation
	if n.Op == "+" {
		if ls, ok := left.(string); ok {
			if rs, ok := right.(string); ok {
				return ls + rs, nil
			}
		}
	}

	// Numeric operations
	switch n.Op {
	case "+", "-", "*", "/", "%", "<", "<=", ">", ">=":
		lv, lok := toFloat(left)
		rv, rok := toFloat(right)
		if !lok || !rok {
			return nil, fmt.Errorf("numeric operation on non-numeric values")
		}
		switch n.Op {
		case "+":
			return lv + rv, nil
		case "-":
			return lv - rv, nil
		case "*":
			return lv * rv, nil
		case "/":
			if rv == 0 {
				return math.Inf(1), nil
			}
			return lv / rv, nil
		case "%":
			return math.Mod(lv, rv), nil
		case "<":
			return boolToFloat(lv < rv), nil
		case "<=":
			return boolToFloat(lv <= rv), nil
		case ">":
			return boolToFloat(lv > rv), nil
		case ">=":
			return boolToFloat(lv >= rv), nil
		}

	case "==", "=":
		return boolToFloat(reflect.DeepEqual(left, right)), nil
	case "!=":
		return boolToFloat(!reflect.DeepEqual(left, right)), nil

	case "&&", "&":
		return boolToFloat(isTruthy(left) && isTruthy(right)), nil
	case "||", "|":
		return boolToFloat(isTruthy(left) || isTruthy(right)), nil
	}

	return nil, fmt.Errorf("unknown operator: %s", n.Op)
}

func (e *Evaluator) evalUnary(n *UnaryNode) (interface{}, error) {
	operand, err := e.Eval(n.Operand)
	if err != nil {
		return nil, err
	}

	switch n.Op {
	case "-":
		if v, ok := toFloat(operand); ok {
			return -v, nil
		}
		return nil, fmt.Errorf("unary minus on non-numeric value")
	case "!":
		return boolToFloat(!isTruthy(operand)), nil
	}

	return nil, fmt.Errorf("unknown unary operator: %s", n.Op)
}

func (e *Evaluator) evalCall(n *CallNode) (interface{}, error) {
	fn, ok := e.functions[strings.ToLower(n.Name)]
	if !ok {
		return nil, fmt.Errorf("undefined function: %s", n.Name)
	}

	args := make([]interface{}, len(n.Args))
	for i, arg := range n.Args {
		val, err := e.Eval(arg)
		if err != nil {
			return nil, err
		}
		args[i] = val
	}

	return fn(args)
}

func (e *Evaluator) evalIndex(n *IndexNode) (interface{}, error) {
	obj, err := e.Eval(n.Object)
	if err != nil {
		return nil, err
	}
	idx, err := e.Eval(n.Index)
	if err != nil {
		return nil, err
	}

	switch o := obj.(type) {
	case []interface{}:
		if i, ok := toFloat(idx); ok {
			index := int(i)
			if index >= 0 && index < len(o) {
				return o[index], nil
			}
		}
		return nil, nil
	case map[string]interface{}:
		if key, ok := idx.(string); ok {
			return o[key], nil
		}
		return o[fmt.Sprintf("%v", idx)], nil
	case string:
		if i, ok := toFloat(idx); ok {
			index := int(i)
			if index >= 0 && index < len(o) {
				return string(o[index]), nil
			}
		}
		return nil, nil
	}

	return nil, fmt.Errorf("cannot index into %T", obj)
}

func (e *Evaluator) evalProperty(n *PropertyNode) (interface{}, error) {
	obj, err := e.Eval(n.Object)
	if err != nil {
		return nil, err
	}

	if m, ok := obj.(map[string]interface{}); ok {
		return m[n.Property], nil
	}

	// Use reflection for struct fields
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		f := v.FieldByName(n.Property)
		if f.IsValid() {
			return f.Interface(), nil
		}
	}

	return nil, fmt.Errorf("cannot access property %s on %T", n.Property, obj)
}

// Helper functions
func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	default:
		return 0, false
	}
}

func toArray(v interface{}) []interface{} {
	switch arr := v.(type) {
	case []interface{}:
		return arr
	case []float64:
		result := make([]interface{}, len(arr))
		for i, val := range arr {
			result[i] = val
		}
		return result
	case []int:
		result := make([]interface{}, len(arr))
		for i, val := range arr {
			result[i] = val
		}
		return result
	case []string:
		result := make([]interface{}, len(arr))
		for i, val := range arr {
			result[i] = val
		}
		return result
	default:
		return nil
	}
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case float32:
		return val != 0
	case int, int32, int64, uint, uint32, uint64:
		f, _ := toFloat(val)
		return f != 0
	case string:
		return val != ""
	default:
		return true
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// Evaluate parses and evaluates an expression with the given variables.
func Evaluate(expression string, variables map[string]interface{}) (interface{}, error) {
	parser := NewParser(expression)
	ast, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	evaluator := NewEvaluator(variables)
	return evaluator.Eval(ast)
}
