package transform

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// DSL provides a simple DSL for defining transforms.
type DSL struct {
	pipeline *Pipeline
}

// NewDSL creates a new DSL instance.
func NewDSL(pipeline *Pipeline) *DSL {
	return &DSL{pipeline: pipeline}
}

// Define creates a transform from a DSL expression.
func (d *DSL) Define(name string, expr string) error {
	transform, err := d.parse(name, expr)
	if err != nil {
		return err
	}
	return d.pipeline.RegisterTransform(transform)
}

func (d *DSL) parse(name string, expr string) (*Transform, error) {
	// Format: "output = expression" or "output = func(inputs...)"
	parts := strings.SplitN(expr, "=", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidExpression
	}

	output := strings.TrimSpace(parts[0])
	body := strings.TrimSpace(parts[1])

	// Detect transform type
	t := &Transform{
		Name:   name,
		Output: output,
		Mode:   ModeOnRead,
	}

	// Check for function calls
	if idx := strings.Index(body, "("); idx > 0 {
		funcName := strings.TrimSpace(body[:idx])
		argsStr := body[idx+1 : strings.LastIndex(body, ")")]
		args := strings.Split(argsStr, ",")
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}

		switch funcName {
		case "sum", "avg", "min", "max", "count":
			t.Type = TypeAggregation
			t.Inputs = args
			t.Config = map[string]interface{}{"type": funcName}
		case "lower", "upper", "trim", "concat":
			t.Type = TypeString
			t.Inputs = args
			t.Config = map[string]interface{}{"operation": funcName}
		case "year", "month", "day", "hour", "weekday":
			t.Type = TypeTimestamp
			t.Inputs = args
			t.Config = map[string]interface{}{"operation": funcName}
		case "window":
			if len(args) >= 3 {
				t.Type = TypeWindow
				t.Inputs = []string{args[0]}
				t.Config = map[string]interface{}{
					"type":   args[1],
					"window": args[2],
				}
			}
		case "lookup":
			if len(args) >= 3 {
				t.Type = TypeLookup
				t.Inputs = []string{args[0]}
				t.Config = map[string]interface{}{
					"lookup_entity":  args[1],
					"lookup_feature": args[2],
				}
			}
		default:
			return nil, fmt.Errorf("unknown function: %s", funcName)
		}
	} else if strings.Contains(body, "?") {
		// Conditional
		t.Type = TypeConditional
		t.Expression = body
		t.Inputs = extractVariables(body)
	} else {
		// Arithmetic
		t.Type = TypeArithmetic
		t.Expression = body
		t.Inputs = extractVariables(body)
	}

	return t, nil
}

func extractVariables(expr string) []string {
	// Extract variable names from expression
	re := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)
	matches := re.FindAllString(expr, -1)

	// Deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	return result
}

// MathFunctions provides common math functions for transforms.
var MathFunctions = map[string]func(float64) float64{
	"abs":   math.Abs,
	"sqrt":  math.Sqrt,
	"log":   math.Log,
	"log10": math.Log10,
	"exp":   math.Exp,
	"floor": math.Floor,
	"ceil":  math.Ceil,
	"round": math.Round,
	"sin":   math.Sin,
	"cos":   math.Cos,
	"tan":   math.Tan,
}

// JSON serializes a transform to JSON.
func JSON(t *Transform) ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON deserializes a transform from JSON.
func FromJSON(data []byte) (*Transform, error) {
	var t Transform
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
