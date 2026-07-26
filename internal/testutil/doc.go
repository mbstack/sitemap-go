// Package testutil provides shared helpers for the sitemap-go
// test suite: in-memory SQLite, httptest servers, and a
// thread-safe log capture buffer.
//
// Usage:
//
//	func TestX(t *testing.T) {
//	    db := testutil.NewTestDB(t)
//	    srv := testutil.NewTestServer(t, http.HandlerFunc(handler))
//	    lg, buf := testutil.NewTestLogger(t)
//	    ...
//	}
//
// All helpers register a t.Cleanup that releases their resources,
// so tests are not required to defer Close themselves.
package testutil
