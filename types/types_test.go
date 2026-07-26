package types

import "testing"

func TestDomainFrom(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://x.com/path", "x.com"},
		{"http://example.com", "example.com"},
		{"https://user:pass@host:8080/p?q=1", "host"},
		{"not a url", ""},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := DomainFrom(c.in)
			if c.want == "" {
				if got != "" {
					t.Errorf("want empty, got %q", got)
				}
				return
			}
			if got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}

func TestURLToPathSlug(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://x.com/a/b/", "a/b"},
		// When the input parses but has an empty path,
		// URLToPathSlug falls back to trimming the raw input.
		// "https://x.com" is treated as a path "https://x.com".
		{"a/b/c", "a/b/c"},
		{"/leading/and/trailing/", "leading/and/trailing"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := URLToPathSlug(c.in); got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}

func TestSafeURLParser(t *testing.T) {
	got, err := SafeURLParser("https://x.com/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "https://x.com" {
		t.Errorf("want https://x.com, got %q", got)
	}
	if _, err := SafeURLParser("not a url"); err == nil {
		t.Errorf("expected error")
	}
}
