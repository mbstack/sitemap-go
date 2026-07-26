// Package httpx provides a small, configurable builder for the
// *http.Client used by sitemap-go's HTTP layer.
package httpx

import (
	"net/http"
	"net/url"
	"time"
)

// DefaultTimeout is the per-request timeout applied when
// Options.Timeout is the zero value.
const DefaultTimeout = 30 * time.Second

// Options configures the *http.Client returned by New.
//
// All fields are optional; sensible defaults are filled in by New.
type Options struct {
	// Timeout is the per-request timeout. Defaults to
	// DefaultTimeout (30s) when zero. Negative values are
	// clamped to DefaultTimeout.
	Timeout time.Duration

	// ProxyURL, if non-empty, is parsed and used as
	// http.Transport.Proxy. Unparseable values are ignored
	// silently and the transport is built without a proxy.
	ProxyURL string

	// UserAgent is sent on every request via the returned
	// client's Transport. RoundTripper. If empty, the default
	// Go user-agent is used.
	UserAgent string
}

// New returns a configured *http.Client.
//
// The returned client wraps an *http.Transport with optional
// proxy support. The UserAgent is injected by a custom
// RoundTripper; callers that need other transport-level
// behaviour (TLS config, dialer tuning) should construct their
// own *http.Client and pass it via config.Config.HTTPClient.
func New(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	tr := &http.Transport{}
	if opts.ProxyURL != "" {
		if pu, err := url.Parse(opts.ProxyURL); err == nil && pu != nil {
			tr.Proxy = http.ProxyURL(pu)
		}
	}

	rt := http.RoundTripper(tr)
	if opts.UserAgent != "" {
		rt = uaRoundTripper{inner: tr, ua: opts.UserAgent}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: rt,
	}
}

// uaRoundTripper is a RoundTripper that injects a User-Agent
// header on every request. It is intentionally minimal: any
// UA already set by the caller is preserved.
type uaRoundTripper struct {
	inner http.RoundTripper
	ua    string
}

func (u uaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone so we don't mutate the caller's request.
	cloned := req.Clone(req.Context())
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", u.ua)
	}
	return u.inner.RoundTrip(cloned)
}
