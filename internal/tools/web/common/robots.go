package common

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/temoto/robotstxt"
)

const (
	// robotsCacheTTL is how long a fetched robots.txt entry is considered fresh.
	robotsCacheTTL = 24 * time.Hour

	// robotsCacheSize is the maximum number of host entries in the LRU.
	robotsCacheSize = 256
)

// robotsEntry holds a parsed robots.txt group set and the time it was fetched.
type robotsEntry struct {
	data      *robotstxt.RobotsData
	fetchedAt time.Time
}

// RobotsChecker fetches and caches robots.txt files for hosts, answering
// IsAllowed queries.  The internal LRU cache holds up to robotsCacheSize
// entries for robotsCacheTTL each.
//
// @MX:ANCHOR: [AUTO] robots.txt enforcement gate for all outbound web requests
// @MX:REASON: SPEC-GOOSE-TOOLS-WEB-001 AC-WEB-004 — called by http_fetch and web_search; fan_in >= 2
type RobotsChecker struct {
	cache *lru.Cache[string, robotsEntry]
	clock func() time.Time
}

// NewRobotsChecker creates a RobotsChecker with a 24h LRU-backed cache.
// clock is injected for testability (use time.Now in production).
func NewRobotsChecker(clock func() time.Time) (*RobotsChecker, error) {
	c, err := lru.New[string, robotsEntry](robotsCacheSize)
	if err != nil {
		return nil, fmt.Errorf("robots: failed to create LRU cache: %w", err)
	}
	return &RobotsChecker{cache: c, clock: clock}, nil
}

// IsAllowed checks whether the given User-Agent is allowed to access path on
// baseURL according to the host's robots.txt.
//
// When path is "/robots.txt", the self-fetch recursion guard activates and
// this function returns true without performing another robots.txt check.
func (rc *RobotsChecker) IsAllowed(baseURL, path, userAgent string) (bool, error) {
	// DC-15 / strategy.md §5 Risk #2: self-fetch recursion guard.
	if path == "/robots.txt" {
		return true, nil
	}

	host := extractHost(baseURL)
	data, err := rc.fetchRobots(host, baseURL)
	if err != nil {
		// If robots.txt cannot be fetched, default to allowing access.
		return true, nil
	}

	group := data.FindGroup(userAgent)
	return group.Test(path), nil
}

// IsAllowedExempt is the same as IsAllowed but skips the check entirely for
// known search-provider API base URLs (e.g. api.search.brave.com).
// This satisfies the REQ-WEB-005 exemption for provider API endpoints.
func (rc *RobotsChecker) IsAllowedExempt(baseURL, path, userAgent string) (bool, error) {
	if isExemptSearchProvider(baseURL) {
		return true, nil
	}
	return rc.IsAllowed(baseURL, path, userAgent)
}

// fetchRobots returns cached or freshly-fetched robotstxt.RobotsData for host.
// The host key is used for LRU lookup; baseURL is used to construct the fetch URL.
func (rc *RobotsChecker) fetchRobots(host, baseURL string) (*robotstxt.RobotsData, error) {
	now := rc.clock()

	if entry, ok := rc.cache.Get(host); ok {
		if now.Sub(entry.fetchedAt) < robotsCacheTTL {
			return entry.data, nil
		}
		// Expired — evict and re-fetch.
		rc.cache.Remove(host)
	}

	robotsURL := strings.TrimRight(baseURL, "/") + "/robots.txt"
	// Fetch robots.txt with explicit timeout and goose-agent User-Agent (REQ-WEB-003).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512 KB limit
	if err != nil {
		return nil, err
	}

	data, err := robotstxt.FromBytes(body)
	if err != nil {
		// A malformed robots.txt is treated as "allow all".
		data = &robotstxt.RobotsData{}
	}

	rc.cache.Add(host, robotsEntry{data: data, fetchedAt: now})
	return data, nil
}

// extractHost returns the host portion of rawURL.
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}

// isExemptSearchProvider returns true for known search-provider API base URLs
// that should be exempt from robots.txt enforcement (REQ-WEB-005).
func isExemptSearchProvider(baseURL string) bool {
	exemptPrefixes := []string{
		"https://api.search.brave.com",
		"https://api.tavily.com",
		"https://api.exa.ai",
	}
	for _, prefix := range exemptPrefixes {
		if strings.HasPrefix(baseURL, prefix) {
			return true
		}
	}
	return false
}
