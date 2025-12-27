package graphql

import (
	"context"
	"testing"
)

func TestSchema_Execute_SimpleQuery(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"hello": {
			Name: "hello",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return "world", nil
			},
		},
		"number": {
			Name: "number",
			Type: IntScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return 42, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	// Test simple query
	response := schema.Execute(ctx, Request{
		Query: `{ hello }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", response.Data)
	}

	if data["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", data["hello"])
	}

	// Test multiple fields
	response = schema.Execute(ctx, Request{
		Query: `{ hello number }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ = response.Data.(map[string]interface{})
	if data["hello"] != "world" || data["number"] != 42 {
		t.Errorf("unexpected data: %v", data)
	}
}

func TestSchema_Execute_WithArguments(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"greet": {
			Name: "greet",
			Type: StringScalar,
			Args: []*Argument{
				{Name: "name", Type: NonNull(StringScalar)},
			},
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				name := args["name"].(string)
				return "Hello, " + name + "!", nil
			},
		},
		"add": {
			Name: "add",
			Type: IntScalar,
			Args: []*Argument{
				{Name: "a", Type: IntScalar},
				{Name: "b", Type: IntScalar},
			},
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				a := int(args["a"].(float64))
				b := int(args["b"].(float64))
				return a + b, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	// Test with string argument
	response := schema.Execute(ctx, Request{
		Query: `{ greet(name: "World") }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	if data["greet"] != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", data["greet"])
	}

	// Test with numeric arguments
	response = schema.Execute(ctx, Request{
		Query: `{ add(a: 5, b: 3) }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ = response.Data.(map[string]interface{})
	if data["add"] != 8 {
		t.Errorf("expected 8, got %v", data["add"])
	}
}

func TestSchema_Execute_WithVariables(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"greet": {
			Name: "greet",
			Type: StringScalar,
			Args: []*Argument{
				{Name: "name", Type: NonNull(StringScalar)},
			},
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				name := args["name"].(string)
				return "Hello, " + name + "!", nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `query($name: String!) { greet(name: $name) }`,
		Variables: map[string]interface{}{
			"name": "Variable",
		},
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	if data["greet"] != "Hello, Variable!" {
		t.Errorf("expected 'Hello, Variable!', got %v", data["greet"])
	}
}

func TestSchema_Execute_WithAlias(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"hello": {
			Name: "hello",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return "world", nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ greeting: hello }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	if data["greeting"] != "world" {
		t.Errorf("expected greeting=world, got %v", data)
	}
}

func TestSchema_Execute_NestedObject(t *testing.T) {
	userType := Object("User", map[string]*Field{
		"id": {
			Name: "id",
			Type: NonNull(IDScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if user, ok := parent.(map[string]interface{}); ok {
					return user["id"], nil
				}
				return nil, nil
			},
		},
		"name": {
			Name: "name",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if user, ok := parent.(map[string]interface{}); ok {
					return user["name"], nil
				}
				return nil, nil
			},
		},
		"email": {
			Name: "email",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if user, ok := parent.(map[string]interface{}); ok {
					return user["email"], nil
				}
				return nil, nil
			},
		},
	})

	queryType := Object("Query", map[string]*Field{
		"user": {
			Name: "user",
			Type: userType,
			Args: []*Argument{
				{Name: "id", Type: NonNull(IDScalar)},
			},
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{
					"id":    args["id"],
					"name":  "John Doe",
					"email": "john@example.com",
				}, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
		Types: []Type{userType},
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ user(id: "123") { id name email } }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	user, _ := data["user"].(map[string]interface{})

	if user["id"] != "123" {
		t.Errorf("expected id=123, got %v", user["id"])
	}
	if user["name"] != "John Doe" {
		t.Errorf("expected name='John Doe', got %v", user["name"])
	}
}

func TestSchema_Execute_List(t *testing.T) {
	itemType := Object("Item", map[string]*Field{
		"id": {
			Name: "id",
			Type: NonNull(IDScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if item, ok := parent.(map[string]interface{}); ok {
					return item["id"], nil
				}
				return nil, nil
			},
		},
		"name": {
			Name: "name",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if item, ok := parent.(map[string]interface{}); ok {
					return item["name"], nil
				}
				return nil, nil
			},
		},
	})

	queryType := Object("Query", map[string]*Field{
		"items": {
			Name: "items",
			Type: List(itemType),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return []map[string]interface{}{
					{"id": "1", "name": "Item 1"},
					{"id": "2", "name": "Item 2"},
					{"id": "3", "name": "Item 3"},
				}, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
		Types: []Type{itemType},
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ items { id name } }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatalf("expected items to be a list, got %T", data["items"])
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	item1, _ := items[0].(map[string]interface{})
	if item1["id"] != "1" || item1["name"] != "Item 1" {
		t.Errorf("unexpected item 1: %v", item1)
	}
}

func TestSchema_Execute_Mutation(t *testing.T) {
	counter := 0

	mutationType := Object("Mutation", map[string]*Field{
		"increment": {
			Name: "increment",
			Type: IntScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				counter++
				return counter, nil
			},
		},
		"add": {
			Name: "add",
			Type: IntScalar,
			Args: []*Argument{
				{Name: "amount", Type: IntScalar},
			},
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				amount := int(args["amount"].(float64))
				counter += amount
				return counter, nil
			},
		},
	})

	queryType := Object("Query", map[string]*Field{
		"counter": {
			Name: "counter",
			Type: IntScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return counter, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	// Execute mutation
	response := schema.Execute(ctx, Request{
		Query: `mutation { increment }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	if data["increment"] != 1 {
		t.Errorf("expected increment=1, got %v", data["increment"])
	}

	// Execute another mutation
	response = schema.Execute(ctx, Request{
		Query: `mutation { add(amount: 5) }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ = response.Data.(map[string]interface{})
	if data["add"] != 6 {
		t.Errorf("expected add=6, got %v", data["add"])
	}
}

func TestSchema_Execute_Typename(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"hello": {
			Name: "hello",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return "world", nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ __typename hello }`,
	})

	if len(response.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", response.Errors)
	}

	data, _ := response.Data.(map[string]interface{})
	if data["__typename"] != "Query" {
		t.Errorf("expected __typename=Query, got %v", data["__typename"])
	}
}

func TestSchema_Execute_UnknownField(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"hello": {
			Name: "hello",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return "world", nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ unknown }`,
	})

	if len(response.Errors) == 0 {
		t.Error("expected error for unknown field")
	}
}

func TestSchema_Execute_ResolverError(t *testing.T) {
	queryType := Object("Query", map[string]*Field{
		"error": {
			Name: "error",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				return nil, nil
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("NewSchema() error = %v", err)
	}

	ctx := context.Background()

	response := schema.Execute(ctx, Request{
		Query: `{ error }`,
	})

	// Should return data with nil value, not error
	data, _ := response.Data.(map[string]interface{})
	if data["error"] != nil {
		t.Errorf("expected error=nil, got %v", data["error"])
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantType string
		wantName string
	}{
		{"simple query", "{ hello }", "query", ""},
		{"explicit query", "query { hello }", "query", ""},
		{"named query", "query GetHello { hello }", "query", "GetHello"},
		{"mutation", "mutation { doSomething }", "mutation", ""},
		{"named mutation", "mutation DoIt { doSomething }", "mutation", "DoIt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := parseQuery(tt.query)
			if err != nil {
				t.Fatalf("parseQuery() error = %v", err)
			}
			if op.Type != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, op.Type)
			}
			if op.Name != tt.wantName {
				t.Errorf("expected name %s, got %s", tt.wantName, op.Name)
			}
		})
	}
}

func TestScalarTypes(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		result, err := StringScalar.serialize("hello")
		if err != nil || result != "hello" {
			t.Errorf("StringScalar.serialize failed: %v, %v", result, err)
		}
	})

	t.Run("Int", func(t *testing.T) {
		result, err := IntScalar.serialize(42)
		if err != nil || result != 42 {
			t.Errorf("IntScalar.serialize failed: %v, %v", result, err)
		}

		result, err = IntScalar.parseValue(42.0)
		if err != nil || result != 42 {
			t.Errorf("IntScalar.parseValue failed: %v, %v", result, err)
		}
	})

	t.Run("Float", func(t *testing.T) {
		result, err := FloatScalar.serialize(3.14)
		if err != nil || result != 3.14 {
			t.Errorf("FloatScalar.serialize failed: %v, %v", result, err)
		}
	})

	t.Run("Boolean", func(t *testing.T) {
		result, err := BooleanScalar.serialize(true)
		if err != nil || result != true {
			t.Errorf("BooleanScalar.serialize failed: %v, %v", result, err)
		}
	})

	t.Run("ID", func(t *testing.T) {
		result, err := IDScalar.serialize("abc123")
		if err != nil || result != "abc123" {
			t.Errorf("IDScalar.serialize failed: %v, %v", result, err)
		}
	})
}

func TestTypeHelpers(t *testing.T) {
	t.Run("List", func(t *testing.T) {
		listType := List(StringScalar)
		if listType.Kind() != TypeKindList {
			t.Errorf("expected List kind, got %s", listType.Kind())
		}
		if listType.Name() != "[String]" {
			t.Errorf("expected name [String], got %s", listType.Name())
		}
	})

	t.Run("NonNull", func(t *testing.T) {
		nonNullType := NonNull(StringScalar)
		if nonNullType.Kind() != TypeKindNonNull {
			t.Errorf("expected NonNull kind, got %s", nonNullType.Kind())
		}
		if nonNullType.Name() != "String!" {
			t.Errorf("expected name String!, got %s", nonNullType.Name())
		}
	})

	t.Run("Object", func(t *testing.T) {
		objType := Object("Test", map[string]*Field{})
		if objType.Kind() != TypeKindObject {
			t.Errorf("expected Object kind, got %s", objType.Kind())
		}
		if objType.Name() != "Test" {
			t.Errorf("expected name Test, got %s", objType.Name())
		}
	})

	t.Run("Enum", func(t *testing.T) {
		enumType := Enum("Status", []*EnumValue{
			{Name: "ACTIVE", Value: "active"},
			{Name: "INACTIVE", Value: "inactive"},
		})
		if enumType.Kind() != TypeKindEnum {
			t.Errorf("expected Enum kind, got %s", enumType.Kind())
		}
		if enumType.Name() != "Status" {
			t.Errorf("expected name Status, got %s", enumType.Name())
		}
	})
}
