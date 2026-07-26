// Package sitemap: robots.txt parsing.
package sitemap

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mbstack/sitemap-go/config"
	"github.com/mbstack/sitemap-go/internal/sitemaperr"
)

// RobotsTxt is the parsed result of a robots.txt document.
// Only the Sitemaps directive is captured; allow/disallow
// rules are out of scope for sitemap-go.
type RobotsTxt struct {
	Sitemaps []string
}

// GetSitemapURLs fetches robots.txt for the given site URL
// and returns the Sitemap URLs declared inside.
//
// siteURL may be a scheme://host or a scheme://host/path; the
// scheme and host are extracted and <scheme>://<host>/robots.txt
// is fetched.
//
// cfg provides the UserAgent (and optionally an HTTPClient for
// tests). The fetch deliberately bypasses the colly-backed
// Fetcher used for sitemap bodies, so a hostile HTTP/2 peer
// can't take down robots.txt discovery with a transport-level
// INTERNAL_ERROR.
//
// Returns ErrNoSitemap if the document has no Sitemap
// directives. Returns an error wrapping ErrFetch on any HTTP
// failure.
func GetSitemapURLs(ctx context.Context, siteURL string, cfg config.Config) (RobotsTxt, error) {
	if ctx == nil {
		return RobotsTxt{}, fmt.Errorf("%w: nil context", sitemaperr.ErrFetch)
	}
	robotsURL, err := robotsURLFor(siteURL)
	if err != nil {
		return RobotsTxt{}, err
	}

	body, err := fetchRobots(ctx, robotsURL, cfg.UserAgent)
	if err != nil {
		return RobotsTxt{}, err
	}

	return parseRobotsTxt(body)
}

// robotsURLFor returns the absolute URL of the robots.txt
// document for a given site URL. siteURL must have a scheme
// and a host.
func robotsURLFor(siteURL string) (string, error) {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return "", fmt.Errorf("%w: invalid site url %q: %v", sitemaperr.ErrFetch, siteURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: site url needs scheme and host: %q", sitemaperr.ErrFetch, siteURL)
	}
	return parsed.Scheme + "://" + parsed.Host + "/robots.txt", nil
}

// parseRobotsTxt parses a robots.txt body and returns the
// declared Sitemap URLs. Comments (# ...) are skipped. The
// directive match is case-insensitive on the leading word
// "sitemap". Whitespace is trimmed. Empty entries are skipped.
func parseRobotsTxt(body []byte) (RobotsTxt, error) {
	var out RobotsTxt
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	// Allow long lines just in case.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and blank lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip any inline comment.
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		// Split on the first colon.
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if key != "sitemap" || val == "" {
			continue
		}
		out.Sitemaps = append(out.Sitemaps, val)
	}
	if err := scanner.Err(); err != nil {
		return RobotsTxt{}, fmt.Errorf("read robots.txt: %w", err)
	}
	if len(out.Sitemaps) == 0 {
		return RobotsTxt{}, sitemaperr.ErrNoSitemap
	}
	return out, nil
}
