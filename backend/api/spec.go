package api

import _ "embed"

// SpecYAML is the OpenAPI specification, embedded at build time so the running
// binary can serve it (see the /api/docs and /api/openapi.yaml routes). Because
// it is baked in at compile time, it works identically in local compose and in
// GKE with no path configuration.
//
//go:embed api.yaml
var SpecYAML []byte
