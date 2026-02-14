package server

import (
	_ "embed"
	"encoding/json"
	"net/http"
)

//go:embed openapi.yaml
var openapiSpec []byte

// registerDocsRoutes adds API documentation endpoints.
func (s *HTTPServer) registerDocsRoutes() {
	s.mux.HandleFunc("GET /v1/openapi.yaml", s.handleOpenAPISpec)
	s.mux.HandleFunc("GET /v1/openapi.json", s.handleOpenAPIJSON)
	s.mux.HandleFunc("GET /docs", s.handleAPIDocs)
}

func (s *HTTPServer) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(openapiSpec)
}

// handleOpenAPIJSON serves a dynamically generated API inventory from the handler registry.
func (s *HTTPServer) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	specs := RegisteredHandlerSpecs()

	type routeGroup struct {
		Handler  string `json:"handler"`
		Maturity string `json:"maturity"`
	}

	info := struct {
		OpenAPI  string       `json:"openapi"`
		Title    string       `json:"title"`
		Version  string       `json:"version"`
		Handlers []routeGroup `json:"handlers"`
		Total    int          `json:"total_handlers"`
	}{
		OpenAPI:  "3.1.0",
		Title:    "Feather Feature Store API",
		Version:  "1.0.0",
		Handlers: make([]routeGroup, 0, len(specs)),
		Total:    len(specs),
	}

	for _, spec := range specs {
		info.Handlers = append(info.Handlers, routeGroup{
			Handler:  spec.Name,
			Maturity: string(spec.Maturity),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(info)
}

func (s *HTTPServer) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	specURL := scheme + "://" + r.Host + "/v1/openapi.yaml"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!doctype html>
<html>
<head>
  <title>Feather API Reference</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="` + specURL + `"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`))
}
