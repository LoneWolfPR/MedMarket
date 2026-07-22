package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOpenAPISpecHandler checks the embedded spec is served as YAML and is the
// real spec (a broken embed path would compile but serve empty bytes).
func TestOpenAPISpecHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	openAPISpecHandler(rec, httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "openapi:", "should serve the embedded OpenAPI spec")
}

// TestDocsHandler checks the Swagger UI page is served as HTML and points at the
// spec route.
func TestDocsHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	docsHandler(rec, httptest.NewRequest(http.MethodGet, "/api/docs", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html"))
	body := rec.Body.String()
	assert.Contains(t, body, "swagger-ui")
	assert.Contains(t, body, "/api/openapi.yaml")
}
