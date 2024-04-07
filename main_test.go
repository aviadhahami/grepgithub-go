package main_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Hit struct {
	Repo    string `json:"repo"`
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

type Hits struct {
	Hits []Hit `json:"hits"`
}

type Arguments struct {
	Query         string
	UseRegex      bool
	WholeWords    bool
	CaseSensitive bool
	RepoFilter    string
	PathFilter    string
	LangFilter    string
}

func TestFetchGrepApp(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the URL and query parameters
		assert.Equal(t, "/api/search", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Check the query parameters
		expectedQuery := "test"
		expectedPage := "1"
		expectedRegex := "true"
		expectedWords := "false"
		expectedCase := "true"
		expectedRepoFilter := "example/repo"
		expectedPathFilter := "example/path"
		expectedLangFilter := "go"

		assert.Equal(t, expectedQuery, r.URL.Query().Get("q"))
		assert.Equal(t, expectedPage, r.URL.Query().Get("page"))
		assert.Equal(t, expectedRegex, r.URL.Query().Get("regexp"))
		assert.Equal(t, expectedWords, r.URL.Query().Get("words"))
		assert.Equal(t, expectedCase, r.URL.Query().Get("case"))
		assert.Equal(t, expectedRepoFilter, r.URL.Query().Get("f.repo.pattern"))
		assert.Equal(t, expectedPathFilter, r.URL.Query().Get("f.path.pattern"))
		assert.Equal(t, expectedLangFilter, r.URL.Query().Get("f.lang"))

		// Return a mock response
		response := `{
			"facets": {
				"count": 10
			},
			"hits": {
				"hits": [
					{
						"repo": {
							"raw": "example/repo"
						},
						"path": {
							"raw": "example/path"
						},
						"content": {
							"snippet": "example snippet"
						}
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Set up the test arguments
	args := &Arguments{
		Query:         "test",
		UseRegex:      true,
		WholeWords:    false,
		CaseSensitive: true,
		RepoFilter:    "example/repo",
		PathFilter:    "example/path",
		LangFilter:    "go",
	}

	// Call the function under test
	hits, count, err := fetchGrepApp(1, args)

	// Check the returned values
	assert.NoError(t, err)
	assert.NotNil(t, hits)
	assert.Equal(t, 1, count)
	assert.Equal(t, 1, len(hits.Hits))

	// Check the hit data
	hit := hits.Hits[0]
	assert.Equal(t, "example/repo", hit.Repo)
	assert.Equal(t, "example/path", hit.Path)
	assert.Equal(t, "example snippet", hit.Snippet)
}

func fetchGrepApp(i int, args *Arguments) (*Hits, string, error) {
	// Create the request URL
	url := fmt.Sprintf("%s/api/search?q=%s&page=%d&regexp=%t&words=%t&case=%t&f.repo.pattern=%s&f.path.pattern=%s&f.lang=%s",
		"https://grep.app",
		args.Query,
		i,
		args.UseRegex,
		args.WholeWords,
		args.CaseSensitive,
		args.RepoFilter,
		args.PathFilter,
		args.LangFilter,
	)

	// Send the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Parse the JSON response
	var result Hits
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, "", err
	}

	// Return the hits and count
	return &result, result.Hits[0].Snippet, nil
}
