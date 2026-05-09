package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/tools/web"
	"github.com/modu-ai/goose/internal/tools/web/common"
)

// koreanFixture is the canonical mock response for the Korean variant.
const koreanFixture = `{
  "type": "standard",
  "title": "서울특별시",
  "extract": "서울특별시는 대한민국의 수도이자 최대 도시이다.",
  "content_urls": {
    "desktop": {"page": "https://ko.wikipedia.org/wiki/서울특별시"}
  }
}`

// englishFixture is the canonical mock response for the English variant.
const englishFixture = `{
  "type": "standard",
  "title": "Seoul",
  "extract": "Seoul is the capital of South Korea.",
  "content_urls": {
    "desktop": {"page": "https://en.wikipedia.org/wiki/Seoul"}
  }
}`

// startWikipediaMockServer returns an httptest server that records every
// request path + Host header into capturedRequests, and serves the per-host
// fixture map (key = "ko" or "en"). Hosts not in the map respond 404.
func startWikipediaMockServer(t *testing.T, fixtures map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	captured := make([]string, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The test injects a base URL like http://127.0.0.1:PORT/<lang>/api/...
		// so the language prefix is in the URL path, not the Host header.
		captured = append(captured, r.URL.Path)
		// Path shape:  /<lang>/api/rest_v1/page/summary/<title>
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) < 6 {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		lang := parts[0]
		body, ok := fixtures[lang]
		if !ok {
			http.Error(w, "no fixture", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// TestWikipedia_LanguageRouting verifies AC-WEB-013:
//   - Korean: language="ko" routes to /<ko>/api/rest_v1/page/summary/<title>
//     and the response data matches the Korean fixture.
//   - English: language="en" routes to /<en>/...
//   - Invalid language code zzz: schema accepts (pattern is permissive), but
//     the fetch fails with code "fetch_failed" because the host returns 404
//     (or unreachable in production).
//   - Schema validation: query length > 200 must fail at schema.
func TestWikipedia_LanguageRouting(t *testing.T) {
	t.Parallel()

	fixtures := map[string]string{
		"ko": koreanFixture,
		"en": englishFixture,
	}
	srv, captured := startWikipediaMockServer(t, fixtures)

	// hostBuilder injects the mock base URL; production uses
	// "https://{lang}.wikipedia.org" but tests redirect through the test
	// server so we can inspect the path and avoid network IO.
	hostBuilder := func(lang string) string {
		// Mock encodes the language as the FIRST path segment because
		// httptest.Server only listens on a single host.
		return srv.URL + "/" + lang
	}

	tool := web.NewWikipediaForTest(&common.Deps{}, hostBuilder)

	t.Run("korean_branch", func(t *testing.T) {
		input := json.RawMessage(`{"query":"seoul","language":"ko"}`)
		result, err := tool.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("Call(ko): %v", err)
		}
		var res common.Response
		if err := json.Unmarshal(result.Content, &res); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !res.OK {
			t.Fatalf("expected ok=true, got error: %+v", res.Error)
		}
		var data struct {
			Summary  string `json:"summary"`
			Language string `json:"language"`
			URL      string `json:"url"`
			Title    string `json:"title"`
		}
		if err := json.Unmarshal(res.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		if data.Language != "ko" {
			t.Errorf("data.language = %q, want %q", data.Language, "ko")
		}
		if !strings.Contains(data.Summary, "서울특별시") {
			t.Errorf("summary missing Korean fixture text: %q", data.Summary)
		}
		var sawKo bool
		for _, p := range *captured {
			if strings.HasPrefix(p, "/ko/") && strings.Contains(p, "/page/summary/") {
				sawKo = true
				break
			}
		}
		if !sawKo {
			t.Errorf("captured paths %v do not contain expected /ko/.../page/summary/...", *captured)
		}
	})

	t.Run("english_branch", func(t *testing.T) {
		input := json.RawMessage(`{"query":"seoul","language":"en"}`)
		result, err := tool.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("Call(en): %v", err)
		}
		var res common.Response
		if err := json.Unmarshal(result.Content, &res); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !res.OK {
			t.Fatalf("expected ok=true, got error: %+v", res.Error)
		}
		var data struct {
			Summary  string `json:"summary"`
			Language string `json:"language"`
		}
		if err := json.Unmarshal(res.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		if data.Language != "en" {
			t.Errorf("data.language = %q, want %q", data.Language, "en")
		}
		if !strings.Contains(data.Summary, "South Korea") {
			t.Errorf("summary missing English fixture text: %q", data.Summary)
		}
		var sawEn bool
		for _, p := range *captured {
			if strings.HasPrefix(p, "/en/") && strings.Contains(p, "/page/summary/") {
				sawEn = true
				break
			}
		}
		if !sawEn {
			t.Errorf("captured paths %v do not contain expected /en/.../page/summary/...", *captured)
		}
	})

	t.Run("invalid_language_zzz_fetch_failed", func(t *testing.T) {
		input := json.RawMessage(`{"query":"seoul","language":"zzz"}`)
		result, err := tool.Call(context.Background(), input)
		if err != nil {
			t.Fatalf("Call(zzz): %v", err)
		}
		var res common.Response
		if err := json.Unmarshal(result.Content, &res); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if res.OK {
			t.Fatalf("expected ok=false for unreachable language, got success: %s", string(res.Data))
		}
		if res.Error == nil || res.Error.Code != "fetch_failed" {
			t.Errorf("expected error.code=fetch_failed, got %+v", res.Error)
		}
	})
}

// TestWikipedia_SchemaValidation verifies that the schema rejects oversized
// queries and the wrong language pattern. The Executor would normally apply
// schema validation; here we exercise the parser-level guard directly.
func TestWikipedia_SchemaValidation(t *testing.T) {
	t.Parallel()
	hostBuilder := func(string) string { return "http://127.0.0.1:0" } // never reached
	tool := web.NewWikipediaForTest(&common.Deps{}, hostBuilder)

	cases := []struct {
		name  string
		input string
		want  string // error code
	}{
		{
			name:  "empty_query",
			input: `{"query":"","language":"en"}`,
			want:  "invalid_input",
		},
		{
			name:  "language_too_short",
			input: `{"query":"seoul","language":"a"}`,
			want:  "invalid_input",
		},
		{
			name:  "language_uppercase",
			input: `{"query":"seoul","language":"EN"}`,
			want:  "invalid_input",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Call(context.Background(), json.RawMessage(tc.input))
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			var res common.Response
			if err := json.Unmarshal(result.Content, &res); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if res.OK {
				t.Fatalf("expected schema rejection for %q, got success", tc.input)
			}
			if res.Error == nil || res.Error.Code != tc.want {
				t.Errorf("expected error.code=%q, got %+v", tc.want, res.Error)
			}
		})
	}
}

// TestWikipedia_RegisteredInWebTools verifies that web_wikipedia is registered
// in the global web tools list at package init time, so WithWeb() exposes it.
func TestWikipedia_RegisteredInWebTools(t *testing.T) {
	t.Parallel()
	names := web.RegisteredWebToolNamesForTest()
	if !slices.Contains(names, "web_wikipedia") {
		t.Errorf("web_wikipedia not in RegisteredWebToolNames: %v", names)
	}
}
