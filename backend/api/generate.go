// Package api holds the OpenAPI specification (api.yaml) and the code
// generation directive for the backend HTTP API. It contains no runtime
// code. Regenerate with `task backend:generate-api`.
package api

//go:generate go tool oapi-codegen -config oapi-codegen.yaml api.yaml
