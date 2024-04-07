package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	C_MARK = "\033[32m"
	C_RST  = "\033[0m"
)

type Hit struct {
	Repo  string            `json:"repo"`
	Path  string            `json:"path"`
	Lines map[string]string `json:"lines"`
}

type Hits struct {
	Hits []Hit `json:"hits"`
}

func (h *Hits) AddHit(repo, path, lineNum, line string) {
	for i := range h.Hits {
		hit := &h.Hits[i]
		if hit.Repo == repo && hit.Path == path {
			hit.Lines[lineNum] = line
			return
		}
	}
	h.Hits = append(h.Hits, Hit{
		Repo:  repo,
		Path:  path,
		Lines: map[string]string{lineNum: line},
	})
}

func (h *Hits) Merge(hits2 *Hits) {
	for _, hit2 := range hits2.Hits {
		h.AddHit(hit2.Repo, hit2.Path, "", "")
		for lineNum, line := range hit2.Lines {
			h.AddHit(hit2.Repo, hit2.Path, lineNum, line)
		}
	}
}

func fail(errorMsg string) {
	log.Fatalf("Error: %s", errorMsg)
}

func fetchGrepApp(page int, args *Arguments) (*Hits, int, error) {
	query := args.Query
	url := fmt.Sprintf("https://grep.app/api/search?q=%s&page=%d", query, page)

	if args.UseRegex {
		url += "&regexp=true"
	} else if args.WholeWords {
		url += "&words=true"
	}

	if args.CaseSensitive {
		url += "&case=true"
	}
	if args.RepoFilter != "" {
		url += fmt.Sprintf("&f.repo.pattern=%s", args.RepoFilter)
	}
	if args.PathFilter != "" {
		url += fmt.Sprintf("&f.path.pattern=%s", args.PathFilter)
	}
	if args.LangFilter != "" {
		url += fmt.Sprintf("&f.lang=%s", args.LangFilter)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("HTTP %d %s", resp.StatusCode, url)
	}

	var data struct {
		Facets struct {
			Count int `json:"count"`
		} `json:"facets"`
		Hits struct {
			Hits []struct {
				Repo struct {
					Raw string `json:"raw"`
				} `json:"repo"`
				Path struct {
					Raw string `json:"raw"`
				} `json:"path"`
				Content struct {
					Snippet string `json:"snippet"`
				} `json:"content"`
			} `json:"hits"`
		} `json:"hits"`
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, 0, err
	}

	hits := &Hits{}
	for _, hitData := range data.Hits.Hits {
		repo := hitData.Repo.Raw
		path := hitData.Path.Raw
		snippet := hitData.Content.Snippet
		hits.AddHit(repo, path, "", "")
		lines := strings.Split(snippet, "\n")
		for _, line := range lines {
			if strings.Contains(line, "<mark") {
				line = strings.ReplaceAll(line, "<mark", C_MARK)
				line = strings.ReplaceAll(line, "</mark>", C_RST)
				line = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(line, "")
				line = strings.ReplaceAll(line, C_MARK, C_RST+C_MARK)
				hits.AddHit(repo, path, line, line)
			}
		}
	}

	count := data.Facets.Count
	return hits, count, nil
}

type Arguments struct {
	Query         string
	CaseSensitive bool
	UseRegex      bool
	WholeWords    bool
	RepoFilter    string
	PathFilter    string
	LangFilter    string
	JsonOutput    bool
	Monochrome    bool
}

func parseArguments() *Arguments {
	args := &Arguments{}
	flag.StringVar(&args.Query, "q", "", "Query string, required")
	flag.BoolVar(&args.CaseSensitive, "c", false, "Case sensitive search")
	flag.BoolVar(&args.UseRegex, "r", false, "Use regex query. Cannot be used with -w")
	flag.BoolVar(&args.WholeWords, "w", false, "Search whole words. Cannot be used with -r")
	flag.StringVar(&args.RepoFilter, "frepo", "", "Filter repository")
	flag.StringVar(&args.PathFilter, "fpath", "", "Filter path")
	flag.StringVar(&args.LangFilter, "flang", "", "Filter language (eg. Python,C,Java). Use comma for multiple values")
	flag.BoolVar(&args.JsonOutput, "json", false, "JSON output")
	flag.BoolVar(&args.Monochrome, "m", false, "Monochrome output")
	flag.Parse()

	if args.Query == "" {
		fail("Query string is required")
	}

	return args
}

func main() {
	args := parseArguments()

	if !args.JsonOutput {
		log.Fatal("JSONL output is required")
	}

	hits := &Hits{}
	nextPage := 1
	for nextPage != 0 && nextPage < 101 {
		time.Sleep(1 * time.Second)
		pageHits, _, err := fetchGrepApp(nextPage, args)
		if err != nil {
			fail(err.Error())
		}
		hits.Merge(pageHits)
		nextPage++
	}

	jsonOut, err := json.Marshal(hits)
	if err != nil {
		fail(err.Error())
	}
	fmt.Println(string(jsonOut))
}
