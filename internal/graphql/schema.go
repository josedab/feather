// Package graphql provides a minimal GraphQL schema and execution engine.
package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Schema represents a GraphQL schema.
type Schema struct {
	queryType        *ObjectType
	mutationType     *ObjectType
	subscriptionType *ObjectType
	types            map[string]Type
	directives       map[string]*Directive
}

// Type represents a GraphQL type.
type Type interface {
	Name() string
	Kind() TypeKind
	Description() string
}

// TypeKind represents the kind of a GraphQL type.
type TypeKind string

// TypeKind values for GraphQL type classification.
const (
	TypeKindScalar      TypeKind = "SCALAR"
	TypeKindObject      TypeKind = "OBJECT"
	TypeKindInterface   TypeKind = "INTERFACE"
	TypeKindUnion       TypeKind = "UNION"
	TypeKindEnum        TypeKind = "ENUM"
	TypeKindInputObject TypeKind = "INPUT_OBJECT"
	TypeKindList        TypeKind = "LIST"
	TypeKindNonNull     TypeKind = "NON_NULL"
)

// ScalarType represents a GraphQL scalar type.
type ScalarType struct {
	name        string
	description string
	serialize   func(interface{}) (interface{}, error)
	parseValue  func(interface{}) (interface{}, error)
}

// Name returns the scalar type name.
func (t *ScalarType) Name() string { return t.name }

// Kind returns the GraphQL type kind.
func (t *ScalarType) Kind() TypeKind { return TypeKindScalar }

// Description returns the scalar type description.
func (t *ScalarType) Description() string { return t.description }

// ObjectType represents a GraphQL object type.
type ObjectType struct {
	name        string
	description string
	fields      map[string]*Field
}

// Name returns the object type name.
func (t *ObjectType) Name() string { return t.name }

// Kind returns the GraphQL type kind.
func (t *ObjectType) Kind() TypeKind { return TypeKindObject }

// Description returns the object type description.
func (t *ObjectType) Description() string { return t.description }

// Field represents a GraphQL field.
type Field struct {
	Name        string
	Description string
	Type        Type
	Args        []*Argument
	Resolver    Resolver
	Deprecated  bool
	Deprecation string
}

// Argument represents a GraphQL argument.
type Argument struct {
	Name         string
	Description  string
	Type         Type
	DefaultValue interface{}
}

// InterfaceType represents a GraphQL interface type.
type InterfaceType struct {
	name        string
	description string
}

// Name returns the interface type name.
func (t *InterfaceType) Name() string { return t.name }

// Kind returns the GraphQL type kind.
func (t *InterfaceType) Kind() TypeKind { return TypeKindInterface }

// Description returns the interface type description.
func (t *InterfaceType) Description() string { return t.description }

// EnumType represents a GraphQL enum type.
type EnumType struct {
	name        string
	description string
	values      []*EnumValue
}

// Name returns the enum type name.
func (t *EnumType) Name() string { return t.name }

// Kind returns the GraphQL type kind.
func (t *EnumType) Kind() TypeKind { return TypeKindEnum }

// Description returns the enum type description.
func (t *EnumType) Description() string { return t.description }

// EnumValue represents a GraphQL enum value.
type EnumValue struct {
	Name        string
	Description string
	Value       interface{}
	Deprecated  bool
	Deprecation string
}

// InputObjectType represents a GraphQL input object type.
type InputObjectType struct {
	name        string
	description string
	fields      map[string]*InputField
}

// Name returns the input object type name.
func (t *InputObjectType) Name() string { return t.name }

// Kind returns the GraphQL type kind.
func (t *InputObjectType) Kind() TypeKind { return TypeKindInputObject }

// Description returns the input object type description.
func (t *InputObjectType) Description() string { return t.description }

// InputField represents a GraphQL input field.
type InputField struct {
	Name         string
	Description  string
	Type         Type
	DefaultValue interface{}
}

// ListType represents a GraphQL list type.
type ListType struct {
	OfType Type
}

// Name returns the list type name.
func (t *ListType) Name() string { return fmt.Sprintf("[%s]", t.OfType.Name()) }

// Kind returns the GraphQL type kind.
func (t *ListType) Kind() TypeKind { return TypeKindList }

// Description returns the list type description.
func (t *ListType) Description() string { return "" }

// NonNullType represents a GraphQL non-null type.
type NonNullType struct {
	OfType Type
}

// Name returns the non-null type name.
func (t *NonNullType) Name() string { return fmt.Sprintf("%s!", t.OfType.Name()) }

// Kind returns the GraphQL type kind.
func (t *NonNullType) Kind() TypeKind { return TypeKindNonNull }

// Description returns the non-null type description.
func (t *NonNullType) Description() string { return "" }

// Directive represents a GraphQL directive.
type Directive struct {
	Name        string
	Description string
	Locations   []string
	Args        []*Argument
}

// Resolver is a function that resolves a field.
type Resolver func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error)

// ResolveInfo contains information about the current resolution.
type ResolveInfo struct {
	FieldName      string
	ReturnType     Type
	ParentType     Type
	Path           []string
	Schema         *Schema
	RootValue      interface{}
	Operation      *Operation
	VariableValues map[string]interface{}
}

// Standard scalar types
var (
	StringScalar = &ScalarType{
		name:        "String",
		description: "The `String` scalar type represents textual data.",
		serialize:   func(v interface{}) (interface{}, error) { return fmt.Sprintf("%v", v), nil },
		parseValue:  func(v interface{}) (interface{}, error) { return fmt.Sprintf("%v", v), nil },
	}

	IntScalar = &ScalarType{
		name:        "Int",
		description: "The `Int` scalar type represents non-fractional signed whole numeric values.",
		serialize: func(v interface{}) (interface{}, error) {
			switch val := v.(type) {
			case int:
				return val, nil
			case int32:
				return int(val), nil
			case int64:
				return int(val), nil
			case float64:
				return int(val), nil
			default:
				return 0, fmt.Errorf("cannot coerce %T to Int", v)
			}
		},
		parseValue: func(v interface{}) (interface{}, error) {
			switch val := v.(type) {
			case float64:
				return int(val), nil
			case int:
				return val, nil
			default:
				return 0, fmt.Errorf("cannot parse %T as Int", v)
			}
		},
	}

	FloatScalar = &ScalarType{
		name:        "Float",
		description: "The `Float` scalar type represents signed double-precision fractional values.",
		serialize: func(v interface{}) (interface{}, error) {
			switch val := v.(type) {
			case float64:
				return val, nil
			case float32:
				return float64(val), nil
			case int:
				return float64(val), nil
			default:
				return 0.0, fmt.Errorf("cannot coerce %T to Float", v)
			}
		},
		parseValue: func(v interface{}) (interface{}, error) {
			switch val := v.(type) {
			case float64:
				return val, nil
			default:
				return 0.0, fmt.Errorf("cannot parse %T as Float", v)
			}
		},
	}

	BooleanScalar = &ScalarType{
		name:        "Boolean",
		description: "The `Boolean` scalar type represents `true` or `false`.",
		serialize: func(v interface{}) (interface{}, error) {
			if b, ok := v.(bool); ok {
				return b, nil
			}
			return false, fmt.Errorf("cannot coerce %T to Boolean", v)
		},
		parseValue: func(v interface{}) (interface{}, error) {
			if b, ok := v.(bool); ok {
				return b, nil
			}
			return false, fmt.Errorf("cannot parse %T as Boolean", v)
		},
	}

	IDScalar = &ScalarType{
		name:        "ID",
		description: "The `ID` scalar type represents a unique identifier.",
		serialize:   func(v interface{}) (interface{}, error) { return fmt.Sprintf("%v", v), nil },
		parseValue:  func(v interface{}) (interface{}, error) { return fmt.Sprintf("%v", v), nil },
	}

	DateTimeScalar = &ScalarType{
		name:        "DateTime",
		description: "The `DateTime` scalar type represents a date and time.",
		serialize: func(v interface{}) (interface{}, error) {
			if t, ok := v.(time.Time); ok {
				return t.Format(time.RFC3339), nil
			}
			return nil, fmt.Errorf("cannot coerce %T to DateTime", v)
		},
		parseValue: func(v interface{}) (interface{}, error) {
			if s, ok := v.(string); ok {
				return time.Parse(time.RFC3339, s)
			}
			return nil, fmt.Errorf("cannot parse %T as DateTime", v)
		},
	}

	JSONScalar = &ScalarType{
		name:        "JSON",
		description: "The `JSON` scalar type represents arbitrary JSON data.",
		serialize:   func(v interface{}) (interface{}, error) { return v, nil },
		parseValue:  func(v interface{}) (interface{}, error) { return v, nil },
	}
)

// NewSchema creates a new schema.
func NewSchema(config SchemaConfig) (*Schema, error) {
	s := &Schema{
		types:      make(map[string]Type),
		directives: make(map[string]*Directive),
	}

	// Register standard scalars
	s.types["String"] = StringScalar
	s.types["Int"] = IntScalar
	s.types["Float"] = FloatScalar
	s.types["Boolean"] = BooleanScalar
	s.types["ID"] = IDScalar
	s.types["DateTime"] = DateTimeScalar
	s.types["JSON"] = JSONScalar

	// Register types from config
	for _, t := range config.Types {
		s.types[t.Name()] = t
	}

	s.queryType = config.Query
	s.mutationType = config.Mutation
	s.subscriptionType = config.Subscription

	return s, nil
}

// SchemaConfig configures schema creation.
type SchemaConfig struct {
	Query        *ObjectType
	Mutation     *ObjectType
	Subscription *ObjectType
	Types        []Type
	Directives   []*Directive
}

// Query returns the query type.
func (s *Schema) Query() *ObjectType {
	return s.queryType
}

// Mutation returns the mutation type.
func (s *Schema) Mutation() *ObjectType {
	return s.mutationType
}

// GetType returns a type by name.
func (s *Schema) GetType(name string) Type {
	return s.types[name]
}

// Object creates a new object type.
func Object(name string, fields map[string]*Field) *ObjectType {
	return &ObjectType{
		name:   name,
		fields: fields,
	}
}

// Input creates a new input object type.
func Input(name string, fields map[string]*InputField) *InputObjectType {
	return &InputObjectType{
		name:   name,
		fields: fields,
	}
}

// Enum creates a new enum type.
func Enum(name string, values []*EnumValue) *EnumType {
	return &EnumType{
		name:   name,
		values: values,
	}
}

// List creates a new list type.
func List(ofType Type) *ListType {
	return &ListType{OfType: ofType}
}

// NonNull creates a new non-null type.
func NonNull(ofType Type) *NonNullType {
	return &NonNullType{OfType: ofType}
}

// Operation represents a GraphQL operation.
type Operation struct {
	Type       string // query, mutation, subscription
	Name       string
	Variables  []*VariableDefinition
	Selections []Selection
}

// VariableDefinition represents a variable definition.
type VariableDefinition struct {
	Name         string
	Type         string
	DefaultValue interface{}
}

// Selection represents a selection in a query.
type Selection interface {
	isSelection()
}

// FieldSelection represents a field selection.
type FieldSelection struct {
	Alias      string
	Name       string
	Arguments  map[string]interface{}
	Selections []Selection
}

func (f *FieldSelection) isSelection() {}

// FragmentSpread represents a fragment spread.
type FragmentSpread struct {
	Name string
}

func (f *FragmentSpread) isSelection() {}

// InlineFragment represents an inline fragment.
type InlineFragment struct {
	TypeCondition string
	Selections    []Selection
}

func (f *InlineFragment) isSelection() {}

// Request represents a GraphQL request.
type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// Response represents a GraphQL response.
type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []Error     `json:"errors,omitempty"`
}

// Error represents a GraphQL error.
type Error struct {
	Message    string                 `json:"message"`
	Locations  []Location             `json:"locations,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Location represents a location in a GraphQL document.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Execute executes a GraphQL query.
func (s *Schema) Execute(ctx context.Context, request Request) *Response {
	// Parse the query
	op, err := parseQuery(request.Query)
	if err != nil {
		return &Response{
			Errors: []Error{{Message: fmt.Sprintf("Parse error: %v", err)}},
		}
	}

	// Determine which type to use
	var rootType *ObjectType
	switch op.Type {
	case "query", "":
		rootType = s.queryType
	case "mutation":
		rootType = s.mutationType
	case "subscription":
		rootType = s.subscriptionType
	default:
		return &Response{
			Errors: []Error{{Message: fmt.Sprintf("Unknown operation type: %s", op.Type)}},
		}
	}

	if rootType == nil {
		return &Response{
			Errors: []Error{{Message: fmt.Sprintf("Schema has no %s type", op.Type)}},
		}
	}

	// Execute
	data, errs := s.executeSelections(ctx, rootType, nil, op.Selections, request.Variables)

	response := &Response{Data: data}
	if len(errs) > 0 {
		response.Errors = errs
	}

	return response
}

func (s *Schema) executeSelections(ctx context.Context, objectType *ObjectType, parent interface{}, selections []Selection, variables map[string]interface{}) (map[string]interface{}, []Error) {
	result := make(map[string]interface{})
	var errors []Error

	for _, sel := range selections {
		switch selection := sel.(type) {
		case *FieldSelection:
			fieldName := selection.Name
			alias := selection.Alias
			if alias == "" {
				alias = fieldName
			}

			// Handle __typename
			if fieldName == "__typename" {
				result[alias] = objectType.Name()
				continue
			}

			field, ok := objectType.fields[fieldName]
			if !ok {
				errors = append(errors, Error{Message: fmt.Sprintf("Field '%s' not found on type '%s'", fieldName, objectType.Name())})
				continue
			}

			// Resolve arguments
			args := resolveArguments(selection.Arguments, variables)

			// Execute resolver
			resolvedValue, err := field.Resolver(ctx, parent, args)
			if err != nil {
				errors = append(errors, Error{Message: err.Error(), Path: []interface{}{alias}})
				result[alias] = nil
				continue
			}

			// Handle nested selections
			if len(selection.Selections) > 0 {
				value, errs := s.resolveValue(ctx, field.Type, resolvedValue, selection.Selections, variables)
				result[alias] = value
				errors = append(errors, errs...)
			} else {
				result[alias] = resolvedValue
			}
		}
	}

	return result, errors
}

func (s *Schema) resolveValue(ctx context.Context, fieldType Type, value interface{}, selections []Selection, variables map[string]interface{}) (interface{}, []Error) {
	if value == nil {
		return nil, nil
	}

	// Unwrap non-null
	if nn, ok := fieldType.(*NonNullType); ok {
		return s.resolveValue(ctx, nn.OfType, value, selections, variables)
	}

	// Handle lists
	if lt, ok := fieldType.(*ListType); ok {
		return s.resolveList(ctx, lt.OfType, value, selections, variables)
	}

	// Handle objects
	if ot, ok := fieldType.(*ObjectType); ok {
		return s.executeSelections(ctx, ot, value, selections, variables)
	}

	// Scalar - just return value
	return value, nil
}

func (s *Schema) resolveList(ctx context.Context, itemType Type, value interface{}, selections []Selection, variables map[string]interface{}) (interface{}, []Error) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, []Error{{Message: fmt.Sprintf("Expected list, got %T", value)}}
	}

	var errors []Error
	result := make([]interface{}, rv.Len())

	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i).Interface()
		resolvedItem, errs := s.resolveValue(ctx, itemType, item, selections, variables)
		result[i] = resolvedItem
		errors = append(errors, errs...)
	}

	return result, errors
}

func resolveArguments(args map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range args {
		if str, ok := v.(string); ok && strings.HasPrefix(str, "$") {
			varName := str[1:]
			if varValue, exists := variables[varName]; exists {
				result[k] = varValue
				continue
			}
		}
		result[k] = v
	}
	return result
}

// Simple query parser
func parseQuery(query string) (*Operation, error) {
	query = strings.TrimSpace(query)
	op := &Operation{Type: "query"}

	// Find operation type and name
	if strings.HasPrefix(query, "query") {
		query = strings.TrimPrefix(query, "query")
		op.Type = "query"
	} else if strings.HasPrefix(query, "mutation") {
		query = strings.TrimPrefix(query, "mutation")
		op.Type = "mutation"
	} else if strings.HasPrefix(query, "subscription") {
		query = strings.TrimPrefix(query, "subscription")
		op.Type = "subscription"
	}

	query = strings.TrimSpace(query)

	// Find operation name if any (before variables or selection set)
	if len(query) > 0 && query[0] != '{' && query[0] != '(' {
		endOfName := strings.IndexAny(query, "({")
		if endOfName > 0 {
			op.Name = strings.TrimSpace(query[:endOfName])
			query = query[endOfName:]
		}
	}

	// Skip variables for now
	if len(query) > 0 && query[0] == '(' {
		depth := 1
		i := 1
		for i < len(query) && depth > 0 {
			if query[i] == '(' {
				depth++
			} else if query[i] == ')' {
				depth--
			}
			i++
		}
		query = strings.TrimSpace(query[i:])
	}

	// Parse selection set
	if len(query) > 0 && query[0] == '{' {
		selections, err := parseSelectionSet(query)
		if err != nil {
			return nil, err
		}
		op.Selections = selections
	}

	return op, nil
}

func parseSelectionSet(query string) ([]Selection, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 || query[0] != '{' {
		return nil, fmt.Errorf("expected selection set")
	}

	// Find matching closing brace
	depth := 1
	i := 1
	for i < len(query) && depth > 0 {
		if query[i] == '{' {
			depth++
		} else if query[i] == '}' {
			depth--
		}
		i++
	}

	content := strings.TrimSpace(query[1 : i-1])
	return parseFields(content)
}

func parseFields(content string) ([]Selection, error) {
	var selections []Selection
	content = strings.TrimSpace(content)

	for len(content) > 0 {
		// Skip whitespace and commas
		content = strings.TrimLeft(content, " \t\n\r,")
		if len(content) == 0 {
			break
		}

		// Parse field
		field, remaining, err := parseField(content)
		if err != nil {
			return nil, err
		}
		selections = append(selections, field)
		content = remaining
	}

	return selections, nil
}

func parseField(content string) (*FieldSelection, string, error) {
	field := &FieldSelection{
		Arguments: make(map[string]interface{}),
	}

	// Find field name (and optional alias)
	endOfName := strings.IndexAny(content, "({: \t\n\r,}")
	if endOfName < 0 {
		endOfName = len(content)
	}

	nameOrAlias := strings.TrimSpace(content[:endOfName])
	content = content[endOfName:]

	// Check for alias
	if len(content) > 0 && content[0] == ':' {
		field.Alias = nameOrAlias
		content = strings.TrimSpace(content[1:])
		endOfName = strings.IndexAny(content, "({: \t\n\r,}")
		if endOfName < 0 {
			endOfName = len(content)
		}
		field.Name = strings.TrimSpace(content[:endOfName])
		content = content[endOfName:]
	} else {
		field.Name = nameOrAlias
	}

	content = strings.TrimSpace(content)

	// Parse arguments
	if len(content) > 0 && content[0] == '(' {
		args, remaining, err := parseArguments(content)
		if err != nil {
			return nil, "", err
		}
		field.Arguments = args
		content = remaining
	}

	content = strings.TrimSpace(content)

	// Parse nested selection set
	if len(content) > 0 && content[0] == '{' {
		depth := 1
		i := 1
		for i < len(content) && depth > 0 {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
			}
			i++
		}

		selectionContent := content[:i]
		selections, err := parseSelectionSet(selectionContent)
		if err != nil {
			return nil, "", err
		}
		field.Selections = selections
		content = content[i:]
	}

	return field, strings.TrimSpace(content), nil
}

func parseArguments(content string) (map[string]interface{}, string, error) {
	args := make(map[string]interface{})

	if len(content) == 0 || content[0] != '(' {
		return args, content, nil
	}

	// Find matching closing paren
	depth := 1
	i := 1
	for i < len(content) && depth > 0 {
		if content[i] == '(' {
			depth++
		} else if content[i] == ')' {
			depth--
		}
		i++
	}

	argsContent := strings.TrimSpace(content[1 : i-1])
	remaining := strings.TrimSpace(content[i:])

	// Parse key-value pairs
	for len(argsContent) > 0 {
		argsContent = strings.TrimLeft(argsContent, " \t\n\r,")
		if len(argsContent) == 0 {
			break
		}

		// Find key
		colonIdx := strings.Index(argsContent, ":")
		if colonIdx < 0 {
			break
		}
		key := strings.TrimSpace(argsContent[:colonIdx])
		argsContent = strings.TrimSpace(argsContent[colonIdx+1:])

		// Parse value
		value, rest, err := parseValue(argsContent)
		if err != nil {
			return nil, "", err
		}
		args[key] = value
		argsContent = rest
	}

	return args, remaining, nil
}

func parseValue(content string) (interface{}, string, error) {
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return nil, "", nil
	}

	// String
	if content[0] == '"' {
		i := 1
		for i < len(content) {
			if content[i] == '"' && (i == 0 || content[i-1] != '\\') {
				break
			}
			i++
		}
		return content[1:i], strings.TrimSpace(content[i+1:]), nil
	}

	// Variable
	if content[0] == '$' {
		endIdx := strings.IndexAny(content, " \t\n\r,)}")
		if endIdx < 0 {
			endIdx = len(content)
		}
		return content[:endIdx], strings.TrimSpace(content[endIdx:]), nil
	}

	// Boolean/null
	if strings.HasPrefix(content, "true") {
		return true, strings.TrimSpace(content[4:]), nil
	}
	if strings.HasPrefix(content, "false") {
		return false, strings.TrimSpace(content[5:]), nil
	}
	if strings.HasPrefix(content, "null") {
		return nil, strings.TrimSpace(content[4:]), nil
	}

	// Number
	endIdx := strings.IndexAny(content, " \t\n\r,)}")
	if endIdx < 0 {
		endIdx = len(content)
	}
	numStr := content[:endIdx]

	// Try parsing as JSON number
	var num interface{}
	if err := json.Unmarshal([]byte(numStr), &num); err == nil {
		return num, strings.TrimSpace(content[endIdx:]), nil
	}

	return numStr, strings.TrimSpace(content[endIdx:]), nil
}
