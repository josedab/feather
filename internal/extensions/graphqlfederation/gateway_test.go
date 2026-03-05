package graphqlfederation

import (
	"context"
	"testing"
)

func TestGatewayServiceCRUD(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	err := gw.RegisterService(ServiceConfig{Name: "users", URL: "http://users:8080/graphql"})
	if err != nil {
		t.Fatal(err)
	}

	services := gw.ListServices()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	if err := gw.RemoveService("users"); err != nil {
		t.Fatal(err)
	}
	if len(gw.ListServices()) != 0 {
		t.Fatal("expected 0 services")
	}
}

func TestGatewayFieldRegistration(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())
	gw.RegisterService(ServiceConfig{Name: "users", URL: "http://users:8080"})

	err := gw.RegisterField(FederatedField{
		FieldName:   "email",
		TypeName:    "User",
		ServiceName: "users",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Unregistered service
	err = gw.RegisterField(FederatedField{
		FieldName:   "orders",
		TypeName:    "User",
		ServiceName: "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown service")
	}

	fields := gw.ListFields()
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
}

func TestGatewayPlanAndExecute(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())
	gw.RegisterService(ServiceConfig{Name: "features", URL: "http://features:8080"})
	gw.RegisterField(FederatedField{FieldName: "score", TypeName: "User", ServiceName: "features"})

	plan, err := gw.PlanQuery("{ user { score } }")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected at least one step")
	}

	result, err := gw.ExecuteQuery(context.Background(), "{ user { score } }")
	if err != nil {
		t.Fatal(err)
	}
	if result["data"] == nil {
		t.Fatal("expected data in result")
	}
}

func TestGatewayBatchResolve(t *testing.T) {
	gw := NewGateway(DefaultGatewayConfig())

	results, err := gw.BatchResolve(context.Background(), BatchRequest{
		Entities: []EntityRef{
			{TypeName: "User", KeyFields: map[string]interface{}{"id": "1"}},
			{TypeName: "User", KeyFields: map[string]interface{}{"id": "2"}},
		},
		Fields: []string{"name", "email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
