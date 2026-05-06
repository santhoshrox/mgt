// Package search wraps OpenSearch for full-text PR search. It runs in
// best-effort mode: failures are logged and never break the request path
// (Postgres remains the source of truth).
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/santhosh/mgt-be/internal/db"
)

const indexName = "pull_requests"

type Client struct {
	baseURL string
	http    *http.Client
	enabled bool
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
		enabled: baseURL != "",
	}
}

func (c *Client) Enabled() bool { return c.enabled }

// EnsureIndex is idempotent; safe to call on every boot.
func (c *Client) EnsureIndex(ctx context.Context) error {
	if !c.enabled {
		return nil
	}
	req, _ := http.NewRequestWithContext(ctx, "HEAD", c.baseURL+"/"+indexName, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}
	body := strings.NewReader(`{
		"mappings": {
			"properties": {
				"repo_id":      {"type":"long"},
				"number":       {"type":"integer"},
				"title":        {"type":"text"},
				"body":         {"type":"text"},
				"author_login": {"type":"keyword"},
				"head_branch":  {"type":"keyword"},
				"state":        {"type":"keyword"},
				"merged":       {"type":"boolean"},
				"draft":        {"type":"boolean"},
				"updated_at":   {"type":"date"}
			}
		}
	}`)
	creq, _ := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/"+indexName, body)
	creq.Header.Set("Content-Type", "application/json")
	cresp, err := c.http.Do(creq)
	if err != nil {
		return err
	}
	defer cresp.Body.Close()
	if cresp.StatusCode >= 300 {
		raw, _ := io.ReadAll(cresp.Body)
		return fmt.Errorf("create index: %s", string(raw))
	}
	return nil
}

func (c *Client) IndexPR(ctx context.Context, pr db.PullRequest) {
	if !c.enabled {
		return
	}
	doc := map[string]any{
		"repo_id":      pr.RepoID,
		"number":       pr.Number,
		"title":        pr.Title,
		"body":         pr.Body,
		"author_login": pr.AuthorLogin,
		"head_branch":  pr.HeadBranch,
		"state":        pr.State,
		"merged":       pr.Merged,
		"draft":        pr.Draft,
		"updated_at":   pr.UpdatedAtGH,
	}
	raw, _ := json.Marshal(doc)
	id := strconv.FormatInt(pr.ID, 10)
	req, _ := http.NewRequestWithContext(ctx, "PUT", c.baseURL+"/"+indexName+"/_doc/"+id, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("opensearch index", "err", err)
		return
	}
	resp.Body.Close()
}

type searchHit struct {
	Source struct {
		RepoID int64 `json:"repo_id"`
		Number int   `json:"number"`
	} `json:"_source"`
}

type searchResponse struct {
	Hits struct {
		Hits []searchHit `json:"hits"`
	} `json:"hits"`
}

// Query returns matching (repo_id, number) pairs. Caller resolves them to
// rows from Postgres. Empty query short-circuits with no results.
func (c *Client) Query(ctx context.Context, repoID int64, q string, size int) ([]int, error) {
	if !c.enabled || strings.TrimSpace(q) == "" {
		return nil, nil
	}
	if size <= 0 {
		size = 50
	}
	body := map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{"term": map[string]any{"repo_id": repoID}},
					map[string]any{"multi_match": map[string]any{
						"query":  q,
						"fields": []string{"title^3", "body", "head_branch", "author_login"},
					}},
				},
			},
		},
	}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/"+indexName+"/_search", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	out := make([]int, 0, len(sr.Hits.Hits))
	for _, h := range sr.Hits.Hits {
		out = append(out, h.Source.Number)
	}
	return out, nil
}
