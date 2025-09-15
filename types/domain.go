package types

import (
	"errors"
	"net/url"
	"strings"
)

func DomainFrom(urlStr string) string {
	// Parse the URL

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Return the host (domain)
	return parsedURL.Hostname()
}

// URLToPathSlug returns the path part of a URL (without leading/trailing slashes).
// If input is already a path like "/tag/slug-here/" or "slug-here", it normalizes and returns it.
func URLToPathSlug(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && u.Path != "" {
		return strings.Trim(u.Path, "/")
	}
	// Fallback: treat raw as a path
	return strings.Trim(raw, "/")
}

func SafeUrlParser(url string) (string, error) {
	domain := DomainFrom(url)
	if domain == "" {
		return "", errors.New("something went wrong")
	}
	return "https://" + domain, nil
}
