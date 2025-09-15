package sitemap

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MegaBytee/sitemap-go/types"
)

type RobotsTxt struct {
	Sitemaps []string
}

// GetSitemapUrlsFromRobotsTxt fetches robots.txt from a given URL and returns all sitemap URLs
func GetSitemapUrlsFromRobotsTxt(url string) (RobotsTxt, error) {
	var txt RobotsTxt
	safeUrl, err := types.SafeUrlParser(url)
	url = safeUrl
	if err != nil {
		return txt, err
	}
	// Ensure the URL ends with robots.txt
	if !strings.HasSuffix(url, "/robots.txt") {
		if strings.HasSuffix(url, "/") {
			url = url + "robots.txt"
		} else {
			url = url + "/robots.txt"
		}
	}
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Set timeout to 10 seconds
	defer cancel()                                                           // Ensure the context is canceled after the request

	// Create a new HTTP request with the context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("failed to create request: %v\n", err)
		return txt, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "MBot/1.0")

	// Make HTTP request to fetch robots.txt
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return txt, fmt.Errorf("failed to fetch robots.txt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return txt, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Slice to store sitemap URLs
	var sitemapURLs []string

	// Read the robots.txt file line by line
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fmt.Println("line:", line)

		// Look for lines starting with "Sitemap:"
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			// Extract the URL after "Sitemap:"
			sitemapURL := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if sitemapURL != "" {
				sitemapURLs = append(sitemapURLs, sitemapURL)
			}
		}
	}
	txt.Sitemaps = sitemapURLs

	if len(sitemapURLs) == 0 {
		return txt, fmt.Errorf("no sitemap URLs found in robots.txt")
	}

	return txt, nil
}
