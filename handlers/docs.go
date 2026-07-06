package handlers

import (
	_ "embed"
	"net/http"
)

//go:embed docs/openapi.yaml
var openapiSpec []byte

// HandleOpenAPISpec serves the raw OpenAPI YAML.
func HandleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapiSpec)
}

// HandleSwaggerUI serves the Swagger UI HTML (assets loaded from CDN).
func HandleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerHTML))
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CVFit API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; }
    #swagger-ui .topbar { background: #0f172a; }
    #swagger-ui .topbar-wrapper img { content: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="white"><text y="18" font-size="14" font-weight="700">CV</text></svg>'); }
    #swagger-ui .topbar-wrapper .link span { color: #f8fafc; font-weight: 700; }
  </style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
<script>
  window.onload = () => {
    SwaggerUIBundle({
      url: "/api/docs/openapi.yaml",
      dom_id: "#swagger-ui",
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "StandaloneLayout",
      persistAuthorization: true,
      displayRequestDuration: true,
      tryItOutEnabled: true,
      defaultModelsExpandDepth: 1,
    });
  };
</script>
</body>
</html>`
