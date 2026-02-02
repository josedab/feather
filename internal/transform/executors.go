package transform

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/feather-store/feather/internal/storage"
)

// ArithmeticExecutor handles arithmetic transformations.
type ArithmeticExecutor struct{}

// Execute runs an arithmetic transform.
func (e *ArithmeticExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	return evaluateArithmetic(t.Expression, inputs)
}

// Validate ensures the arithmetic transform is configured correctly.
func (e *ArithmeticExecutor) Validate(t *Transform) error {
	if t.Expression == "" {
		return fmt.Errorf("expression is required")
	}
	return nil
}

func evaluateArithmetic(expr string, inputs map[string]interface{}) (interface{}, error) {
	// Replace variable names with values
	for name, value := range inputs {
		numVal, err := toFloat64(value)
		if err != nil {
			return nil, fmt.Errorf("input %s: %w", name, err)
		}
		expr = strings.ReplaceAll(expr, name, fmt.Sprintf("%f", numVal))
	}

	// Parse and evaluate simple arithmetic expressions
	return parseAndEvaluate(expr)
}

func parseAndEvaluate(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)

	// Handle parentheses
	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		end := strings.Index(expr[start:], ")") + start
		if end <= start {
			return 0, ErrInvalidExpression
		}
		inner, err := parseAndEvaluate(expr[start+1 : end])
		if err != nil {
			return 0, err
		}
		expr = expr[:start] + fmt.Sprintf("%f", inner) + expr[end+1:]
	}

	// Handle addition and subtraction (lowest precedence)
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '+' && i > 0 {
			left, err := parseAndEvaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := parseAndEvaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left + right, nil
		}
		if expr[i] == '-' && i > 0 && expr[i-1] != '*' && expr[i-1] != '/' && expr[i-1] != '+' && expr[i-1] != '-' {
			left, err := parseAndEvaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := parseAndEvaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left - right, nil
		}
	}

	// Handle multiplication and division
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '*' {
			left, err := parseAndEvaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := parseAndEvaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return left * right, nil
		}
		if expr[i] == '/' {
			left, err := parseAndEvaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := parseAndEvaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		}
	}

	// Parse as number
	return strconv.ParseFloat(strings.TrimSpace(expr), 64)
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, ErrTypeMismatch
	}
}

func toFloat64Safe(v interface{}) (float64, bool) {
	f, err := toFloat64(v)
	return f, err == nil
}

// AggregationExecutor handles aggregation transformations.
type AggregationExecutor struct {
	store *storage.Store
}

// Execute runs an aggregation transform.
func (e *AggregationExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	aggType, ok := t.Config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("aggregation type must be a string")
	}

	values := make([]float64, 0)
	for _, v := range inputs {
		if f, err := toFloat64(v); err == nil {
			values = append(values, f)
		}
	}

	switch aggType {
	case "sum":
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum, nil
	case "avg", "mean":
		if len(values) == 0 {
			return 0.0, nil
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values)), nil
	case "min":
		if len(values) == 0 {
			return nil, nil
		}
		minValue := values[0]
		for _, v := range values[1:] {
			if v < minValue {
				minValue = v
			}
		}
		return minValue, nil
	case "max":
		if len(values) == 0 {
			return nil, nil
		}
		maxValue := values[0]
		for _, v := range values[1:] {
			if v > maxValue {
				maxValue = v
			}
		}
		return maxValue, nil
	case "count":
		return float64(len(values)), nil
	default:
		return nil, fmt.Errorf("unknown aggregation type: %s", aggType)
	}
}

// Validate ensures the aggregation transform is configured correctly.
func (e *AggregationExecutor) Validate(t *Transform) error {
	if t.Config == nil || t.Config["type"] == nil {
		return fmt.Errorf("aggregation type is required in config")
	}
	return nil
}

// WindowExecutor handles windowed aggregations.
type WindowExecutor struct {
	store *storage.Store
}

// Execute runs a windowed aggregation transform.
func (e *WindowExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	windowStr, ok := t.Config["window"].(string)
	if !ok {
		return nil, fmt.Errorf("window must be a string")
	}
	aggType, ok := t.Config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("aggregation type must be a string")
	}

	window, err := time.ParseDuration(windowStr)
	if err != nil {
		return nil, fmt.Errorf("invalid window: %w", err)
	}

	// Get historical values from store
	entityID, ok := t.Config["entity_id"].(string)
	if !ok {
		return nil, fmt.Errorf("entity_id must be a string")
	}
	if len(t.Inputs) == 0 {
		return nil, fmt.Errorf("input feature is required")
	}
	featureName := t.Inputs[0]

	asOf := time.Now()
	startTime := asOf.Add(-window)

	values, err := e.store.GetAsOf(entityID, []string{featureName}, startTime)
	if err != nil {
		return nil, err
	}

	floatValues := make([]float64, 0)
	for _, v := range values {
		if f, err := toFloat64(v.Value); err == nil {
			floatValues = append(floatValues, f)
		}
	}

	switch aggType {
	case "sum":
		var sum float64
		for _, v := range floatValues {
			sum += v
		}
		return sum, nil
	case "avg":
		if len(floatValues) == 0 {
			return 0.0, nil
		}
		var sum float64
		for _, v := range floatValues {
			sum += v
		}
		return sum / float64(len(floatValues)), nil
	case "count":
		return float64(len(floatValues)), nil
	default:
		return nil, fmt.Errorf("unknown aggregation: %s", aggType)
	}
}

// Validate ensures the window transform is configured correctly.
func (e *WindowExecutor) Validate(t *Transform) error {
	if t.Config == nil {
		return fmt.Errorf("config is required for window transform")
	}
	if t.Config["window"] == nil {
		return fmt.Errorf("window duration is required")
	}
	if t.Config["type"] == nil {
		return fmt.Errorf("aggregation type is required")
	}
	return nil
}

// ConditionalExecutor handles conditional transformations.
type ConditionalExecutor struct{}

// Execute runs a conditional transform.
func (e *ConditionalExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	// Expression format: "condition ? true_value : false_value"
	expr := t.Expression

	// Parse condition
	parts := strings.SplitN(expr, "?", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidExpression
	}

	condition := strings.TrimSpace(parts[0])
	values := strings.SplitN(parts[1], ":", 2)
	if len(values) != 2 {
		return nil, ErrInvalidExpression
	}

	trueValue := strings.TrimSpace(values[0])
	falseValue := strings.TrimSpace(values[1])

	// Evaluate condition
	condResult, err := evaluateCondition(condition, inputs)
	if err != nil {
		return nil, err
	}

	if condResult {
		return resolveValue(trueValue, inputs)
	}
	return resolveValue(falseValue, inputs)
}

// Validate ensures the conditional transform is configured correctly.
func (e *ConditionalExecutor) Validate(t *Transform) error {
	if t.Expression == "" {
		return fmt.Errorf("expression is required")
	}
	if !strings.Contains(t.Expression, "?") || !strings.Contains(t.Expression, ":") {
		return fmt.Errorf("expression must be in format: condition ? true_value : false_value")
	}
	return nil
}

func evaluateCondition(condition string, inputs map[string]interface{}) (bool, error) {
	// Support: ==, !=, >, <, >=, <=
	operators := []string{">=", "<=", "!=", "==", ">", "<"}

	for _, op := range operators {
		if strings.Contains(condition, op) {
			parts := strings.SplitN(condition, op, 2)
			if len(parts) != 2 {
				continue
			}

			left, err := resolveValue(strings.TrimSpace(parts[0]), inputs)
			if err != nil {
				return false, err
			}
			right, err := resolveValue(strings.TrimSpace(parts[1]), inputs)
			if err != nil {
				return false, err
			}

			leftNum, leftIsNum := toFloat64Safe(left)
			rightNum, rightIsNum := toFloat64Safe(right)

			if leftIsNum && rightIsNum {
				switch op {
				case "==":
					return leftNum == rightNum, nil
				case "!=":
					return leftNum != rightNum, nil
				case ">":
					return leftNum > rightNum, nil
				case "<":
					return leftNum < rightNum, nil
				case ">=":
					return leftNum >= rightNum, nil
				case "<=":
					return leftNum <= rightNum, nil
				}
			} else {
				// String comparison
				leftStr := fmt.Sprintf("%v", left)
				rightStr := fmt.Sprintf("%v", right)
				switch op {
				case "==":
					return leftStr == rightStr, nil
				case "!=":
					return leftStr != rightStr, nil
				}
			}
		}
	}

	return false, ErrInvalidExpression
}

func resolveValue(s string, inputs map[string]interface{}) (interface{}, error) {
	s = strings.TrimSpace(s)

	// Check if it's a variable
	if v, ok := inputs[s]; ok {
		return v, nil
	}

	// Try to parse as number
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	// Try to parse as quoted string
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1], nil
	}
	if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
		return s[1 : len(s)-1], nil
	}

	return s, nil
}

// StringExecutor handles string transformations.
type StringExecutor struct{}

// Execute runs a string transform.
func (e *StringExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	op, ok := t.Config["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation must be a string")
	}

	// Get the input string
	var inputStr string
	for _, v := range inputs {
		inputStr = fmt.Sprintf("%v", v)
		break
	}

	switch op {
	case "lower":
		return strings.ToLower(inputStr), nil
	case "upper":
		return strings.ToUpper(inputStr), nil
	case "trim":
		return strings.TrimSpace(inputStr), nil
	case "length":
		return float64(len(inputStr)), nil
	case "concat":
		var parts []string
		for _, input := range t.Inputs {
			if v, ok := inputs[input]; ok {
				parts = append(parts, fmt.Sprintf("%v", v))
			}
		}
		separator := ""
		if sep, ok := t.Config["separator"].(string); ok {
			separator = sep
		}
		return strings.Join(parts, separator), nil
	case "substring":
		startValue, ok := t.Config["start"].(float64)
		if !ok {
			return nil, fmt.Errorf("start must be a number")
		}
		start := int(startValue)
		end := len(inputStr)
		if e, ok := t.Config["end"].(float64); ok {
			end = int(e)
		}
		if start < 0 || start > len(inputStr) || end > len(inputStr) {
			return "", nil
		}
		return inputStr[start:end], nil
	case "replace":
		old, ok := t.Config["old"].(string)
		if !ok {
			return nil, fmt.Errorf("old must be a string")
		}
		newValue, ok := t.Config["new"].(string)
		if !ok {
			return nil, fmt.Errorf("new must be a string")
		}
		return strings.ReplaceAll(inputStr, old, newValue), nil
	case "regex_extract":
		pattern, ok := t.Config["pattern"].(string)
		if !ok {
			return nil, fmt.Errorf("pattern must be a string")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		matches := re.FindStringSubmatch(inputStr)
		if len(matches) > 1 {
			return matches[1], nil
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unknown string operation: %s", op)
	}
}

// Validate checks required config for the string executor.
func (e *StringExecutor) Validate(t *Transform) error {
	if t.Config == nil || t.Config["operation"] == nil {
		return fmt.Errorf("operation is required in config")
	}
	return nil
}

// TimestampExecutor handles timestamp transformations.
type TimestampExecutor struct{}

// Execute runs the timestamp transform.
func (e *TimestampExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	op, ok := t.Config["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation must be a string")
	}

	var ts time.Time
	for _, v := range inputs {
		switch val := v.(type) {
		case time.Time:
			ts = val
		case int64:
			ts = time.Unix(0, val)
		case float64:
			ts = time.Unix(int64(val), 0)
		case string:
			var err error
			ts, err = time.Parse(time.RFC3339, val)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp: %w", err)
			}
		}
		break
	}

	switch op {
	case "year":
		return float64(ts.Year()), nil
	case "month":
		return float64(ts.Month()), nil
	case "day":
		return float64(ts.Day()), nil
	case "hour":
		return float64(ts.Hour()), nil
	case "minute":
		return float64(ts.Minute()), nil
	case "weekday":
		return float64(ts.Weekday()), nil
	case "unix":
		return float64(ts.Unix()), nil
	case "unix_nano":
		return float64(ts.UnixNano()), nil
	case "age_seconds":
		return time.Since(ts).Seconds(), nil
	case "age_hours":
		return time.Since(ts).Hours(), nil
	case "age_days":
		return time.Since(ts).Hours() / 24, nil
	case "format":
		format, ok := t.Config["format"].(string)
		if !ok {
			return nil, fmt.Errorf("format must be a string")
		}
		return ts.Format(format), nil
	default:
		return nil, fmt.Errorf("unknown timestamp operation: %s", op)
	}
}

// Validate checks required config for the timestamp executor.
func (e *TimestampExecutor) Validate(t *Transform) error {
	if t.Config == nil || t.Config["operation"] == nil {
		return fmt.Errorf("operation is required in config")
	}
	return nil
}

// LookupExecutor handles lookup transformations.
type LookupExecutor struct {
	store *storage.Store
}

// Execute runs a lookup transform against the store.
func (e *LookupExecutor) Execute(ctx context.Context, t *Transform, inputs map[string]interface{}) (interface{}, error) {
	lookupEntity, ok := t.Config["lookup_entity"].(string)
	if !ok {
		return nil, fmt.Errorf("lookup_entity must be a string")
	}
	lookupFeature, ok := t.Config["lookup_feature"].(string)
	if !ok {
		return nil, fmt.Errorf("lookup_feature must be a string")
	}

	// Build the lookup key
	var keyValue string
	for _, v := range inputs {
		keyValue = fmt.Sprintf("%v", v)
		break
	}

	lookupKey := fmt.Sprintf("%s:%s", lookupEntity, keyValue)

	values, err := e.store.Get(lookupKey, []string{lookupFeature})
	if err != nil {
		// Return default if configured
		if defaultVal, ok := t.Config["default"]; ok {
			return defaultVal, nil
		}
		return nil, err
	}

	if v, ok := values[lookupFeature]; ok {
		return v.Value, nil
	}

	if defaultVal, ok := t.Config["default"]; ok {
		return defaultVal, nil
	}

	return nil, nil
}

// Validate checks required config for lookup transforms.
func (e *LookupExecutor) Validate(t *Transform) error {
	if t.Config == nil {
		return fmt.Errorf("config is required")
	}
	if t.Config["lookup_entity"] == nil {
		return fmt.Errorf("lookup_entity is required")
	}
	if t.Config["lookup_feature"] == nil {
		return fmt.Errorf("lookup_feature is required")
	}
	return nil
}
