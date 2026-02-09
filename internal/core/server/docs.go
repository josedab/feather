package server

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiSpec []byte

// registerDocsRoutes adds API documentation endpoints.
func (s *HTTPServer) registerDocsRoutes() {
	s.mux.HandleFunc("GET /v1/openapi.yaml", s.handleOpenAPISpec)
	s.mux.HandleFunc("GET /docs", s.handleAPIDocs)
}

func (s *HTTPServer) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(openapiSpec)
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
