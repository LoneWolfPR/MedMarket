// Package api holds the OpenAPI specification (api.yaml), the code generation
// directive for the backend HTTP API, and the spec embedded for serving at
// runtime (see spec.go). Regenerate with `task backend:generate-api`.
package api

//go:generate go tool oapi-codegen -config oapi-codegen.yaml api.yaml
