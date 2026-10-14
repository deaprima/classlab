package app_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var (
	specOnce   sync.Once
	specRouter routers.Router
	specErr    error
)

func loadSpec() (routers.Router, error) {
	specOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		path := filepath.Join(filepath.Dir(file), "..", "..", "openapi.yaml")

		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromFile(path)
		if err != nil {
			specErr = err
			return
		}
		if err := doc.Validate(context.Background()); err != nil {
			specErr = err
			return
		}
		specRouter, specErr = gorillamux.NewRouter(doc)
	})
	return specRouter, specErr
}

// checkContract validates the real response against api/openapi.yaml: status code, headers
// and body schema. Paths outside the spec (404 handlers, /health under no prefix) are skipped.
func (e *env) checkContract(req *http.Request, body []byte, res *httptest.ResponseRecorder) {
	e.t.Helper()
	if !strings.HasPrefix(req.URL.Path, "/api/v1/") {
		return
	}
	router, err := loadSpec()
	if err != nil {
		e.t.Fatalf("openapi.yaml is not a valid OpenAPI document: %v", err)
	}

	// The spec has server URL /api/v1, and the router matches against the full URL.
	probe := req.Clone(req.Context())
	probe.URL.Scheme, probe.URL.Host = "http", "localhost"
	probe.Body = io.NopCloser(bytes.NewReader(body))
	route, params, err := router.FindRoute(probe)
	if err != nil {
		return // no such path in the contract, e.g. the deliberate 404 test
	}

	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: probe, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status: res.Code,
		Header: res.Header(),
	}
	in.SetBodyBytes(res.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		e.t.Errorf("response violates openapi.yaml: %s %s -> %d: %v\nbody: %s", req.Method, req.URL.Path, res.Code, err, res.Body.String())
	}
}
