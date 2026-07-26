package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewTestServer starts an httptest.Server with the given handler
// and registers a t.Cleanup that closes it. The returned
// *httptest.Server.URL is the absolute base URL.
func NewTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
