package main

import (
	"log/slog"
	"net/http"

	"github.com/LoneWolfPR/MedMarket/backend/api"
)

// docsHTML is the Swagger UI page. It is fully static: Swagger UI is client-side
// JS that fetches the spec from /api/openapi.yaml and renders itself. The CDN
// assets are version-pinned so a new Swagger UI release can't change or break the
// page underneath us.
const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>MedMarket API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '/api/openapi.yaml',
      dom_id: '#swagger-ui',
    });
  </script>
</body>
</html>
`

// openAPISpecHandler serves the embedded OpenAPI spec that Swagger UI fetches.
func openAPISpecHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	if _, err := w.Write(api.SpecYAML); err != nil {
		slog.Error("failed to write openapi spec", "error", err)
	}
}

// docsHandler serves the Swagger UI page.
func docsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write([]byte(docsHTML)); err != nil {
		slog.Error("failed to write docs page", "error", err)
	}
}
