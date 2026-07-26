package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestNew_DefaultTimeout(t *testing.T) {
	c := New(Options{})
	if c.Timeout != DefaultTimeout {
		t.Errorf("Timeout: want %s, got %s", DefaultTimeout, c.Timeout)
	}
}

func TestNew_TimeoutRespected(t *testing.T) {
	c := New(Options{Timeout: 5 * time.Second})
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout: want 5s, got %s", c.Timeout)
	}
}

func TestNew_NegativeTimeoutClampedToDefault(t *testing.T) {
	c := New(Options{Timeout: -1 * time.Second})
	if c.Timeout != DefaultTimeout {
		t.Errorf("negative timeout should clamp to default, got %s", c.Timeout)
	}
}

func TestNew_ProxyURLSetOnTransport(t *testing.T) {
	proxy, _ := url.Parse("http://127.0.0.1:9999")
	c := New(Options{ProxyURL: proxy.String()})
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	// http.ProxyURL is a func(*http.Request) (*url.URL, error).
	// Calling it with a dummy request must return our proxy URL.
	got, err := tr.Proxy(&http.Request{URL: proxy})
	if err != nil {
		t.Fatalf("Proxy func returned err: %v", err)
	}
	if got == nil || got.String() != proxy.String() {
		t.Errorf("Proxy URL: want %s, got %v", proxy, got)
	}
}

func TestNew_InvalidProxyURLIgnored(t *testing.T) {
	// "://" is not a valid URL — should be silently ignored
	// (transport still usable, no proxy set).
	c := New(Options{ProxyURL: "://"})
	if c.Timeout != DefaultTimeout {
		t.Errorf("expected default timeout, got %s", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	if tr.Proxy != nil {
		t.Error("invalid proxy URL should leave transport.Proxy nil")
	}
}

func TestNew_UserAgentInjected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "mbstack-test/1.0" {
			t.Errorf("server saw UA %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{UserAgent: "mbstack-test/1.0"})
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
}

func TestNew_UserAgentPreservesCaller(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "caller-ua" {
			t.Errorf("server saw UA %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Options{UserAgent: "should-be-overridden"})
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("User-Agent", "caller-ua")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
}
