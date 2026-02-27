package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/core/logging"
	"github.com/feather-store/feather/internal/extensions/graphql"
)

// GraphQLHandler handles GraphQL API requests.
type GraphQLHandler struct {
	schema *graphql.FeatureStoreSchema
}

// NewGraphQLHandler creates a new GraphQL handler.
func NewGraphQLHandler(schema *graphql.FeatureStoreSchema) *GraphQLHandler {
	return &GraphQLHandler{
		schema: schema,
	}
}

// RegisterRoutes registers GraphQL API routes.
func (h *GraphQLHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /graphql", h.handleQuery)
	mux.HandleFunc("GET /graphql", h.handlePlayground)
}

// handleQuery handles POST /graphql
func (h *GraphQLHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	if h.schema == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "GraphQL schema not configured")
		return
	}

	var req graphql.Request
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "query is required")
		return
	}

	response := h.schema.Execute(r.Context(), req)

	w.Header().Set("Content-Type", "application/json")
	if len(response.Errors) > 0 && response.Data == nil {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logging.FromContext(r.Context(), nil).Error("failed to encode GraphQL response", "error", err)
	}
}

// handlePlayground handles GET /graphql - serves GraphQL playground
func (h *GraphQLHandler) handlePlayground(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(graphiqlHTML)); err != nil {
		logging.FromContext(r.Context(), nil).Error("failed to write GraphQL playground response", "error", err)
	}
}

const graphiqlHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Feather GraphQL Playground</title>
  <style>
    body {
      height: 100%;
      margin: 0;
      width: 100%;
      overflow: hidden;
    }
    #graphiql {
      height: 100vh;
    }
  </style>
  <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
</head>
<body>
  <div id="graphiql">Loading...</div>
  <script
    crossorigin
    src="https://unpkg.com/react/umd/react.production.min.js"
  ></script>
  <script
    crossorigin
    src="https://unpkg.com/react-dom/umd/react-dom.production.min.js"
  ></script>
  <script
    crossorigin
    src="https://unpkg.com/graphiql/graphiql.min.js"
  ></script>
  <script>
    const fetcher = GraphiQL.createFetcher({ url: '/graphql' });
    ReactDOM.render(
      React.createElement(GraphiQL, {
        fetcher: fetcher,
        defaultEditorToolsVisibility: true,
        defaultQuery: ` + "`" + `# Welcome to Feather GraphQL Playground
#
# Example queries:
#
# Get a feature value:
# query {
#   feature(entity: "user:123", feature: "purchase_count") {
#     entity
#     feature
#     value
#     timestamp
#   }
# }
#
# Get multiple features:
# query {
#   features(entity: "user:123", features: ["purchase_count", "total_spend"]) {
#     feature
#     value
#   }
# }
#
# List feature groups:
# query {
#   featureGroups {
#     name
#     description
#     featureCount
#   }
# }
#
# Set a feature value:
# mutation {
#   setFeature(entity: "user:123", feature: "score", value: 0.85) {
#     entity
#     feature
#     value
#     timestamp
#   }
# }

query {
  healthCheck {
    status
    timestamp
    version
  }
}
` + "`" + `,
      }),
      document.getElementById('graphiql'),
    );
  </script>
</body>
</html>`

func (h *GraphQLHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(graphql.Response{
		Errors: []graphql.Error{{Message: message}},
	}); err != nil {
		logging.FromContext(ctx, nil).Error("failed to encode GraphQL error response", "error", err)
	}
}
