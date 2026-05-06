// Package github is a small REST wrapper used in place of go-github. It only
// exposes the endpoints mgt-be actually needs and centralises auth/error
// handling so handlers stay slim.
package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

type Client struct {
	token string
	http  *http.Client
}

func New(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("github %d: %s", e.Status, e.Message) }

func (c *Client) do(method, path string, body any, out any) error {
	var buf io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, apiBase+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		var apiErr struct{ Message string }
		_ = json.Unmarshal(raw, &apiErr)
		if apiErr.Message == "" {
			apiErr.Message = string(raw)
		}
		return &Error{Status: resp.StatusCode, Message: apiErr.Message}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// ── User ──────────────────────────────────────────────────────────────────

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
}

func (c *Client) Me() (User, error) {
	var u User
	return u, c.do("GET", "/user", nil, &u)
}

// ── Repositories ──────────────────────────────────────────────────────────

type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Owner         struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Client) ListRepos() ([]Repo, error) {
	var out []Repo
	return out, c.do("GET", "/user/repos?sort=pushed&direction=desc&per_page=100&type=owner", nil, &out)
}

func (c *Client) GetRepo(owner, name string) (Repo, error) {
	var r Repo
	return r, c.do("GET", "/repos/"+owner+"/"+name, nil, &r)
}

// ── Pull requests ─────────────────────────────────────────────────────────

type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Draft     bool   `json:"draft"`
	Merged    bool   `json:"merged"`
	HTMLURL   string `json:"html_url"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Comments  int    `json:"comments"`
	User      struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
	MergeableState string    `json:"mergeable_state"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreatePRInput struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body,omitempty"`
	Draft bool   `json:"draft,omitempty"`
}

func (c *Client) CreatePR(owner, name string, in CreatePRInput) (PR, error) {
	var pr PR
	return pr, c.do("POST", "/repos/"+owner+"/"+name+"/pulls", in, &pr)
}

func (c *Client) UpdatePR(owner, name string, number int, body map[string]any) (PR, error) {
	var pr PR
	return pr, c.do("PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, name, number), body, &pr)
}

func (c *Client) ListOpenPRs(owner, name string) ([]PR, error) {
	var out []PR
	return out, c.do("GET", "/repos/"+owner+"/"+name+"/pulls?state=open&per_page=100&sort=updated&direction=desc", nil, &out)
}

func (c *Client) GetPR(owner, name string, number int) (PR, error) {
	var pr PR
	return pr, c.do("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, name, number), nil, &pr)
}

func (c *Client) FindPRByHeadBranch(owner, name, branch string) (PR, bool, error) {
	q := url.Values{}
	q.Set("head", owner+":"+branch)
	q.Set("state", "open")
	var out []PR
	if err := c.do("GET", "/repos/"+owner+"/"+name+"/pulls?"+q.Encode(), nil, &out); err != nil {
		return PR{}, false, err
	}
	if len(out) == 0 {
		return PR{}, false, nil
	}
	return out[0], true, nil
}

type MergeResult struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

func (c *Client) MergePR(owner, name string, number int, method string) (MergeResult, error) {
	var out MergeResult
	return out, c.do("PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, name, number),
		map[string]string{"merge_method": method}, &out)
}

// ── Combined CI status ────────────────────────────────────────────────────

type CombinedStatus struct {
	State string `json:"state"` // success, failure, pending
}

func (c *Client) CombinedStatus(owner, name, ref string) (string, error) {
	var s CombinedStatus
	if err := c.do("GET", fmt.Sprintf("/repos/%s/%s/commits/%s/status", owner, name, ref), nil, &s); err != nil {
		return "", err
	}
	return s.State, nil
}

// ── Contents (PR templates) ───────────────────────────────────────────────

type contentsResponse struct {
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

// ReadFile fetches a file's raw bytes from the default branch. Returns
// (text, true, nil) on success, (zero, false, nil) for 404, or an error.
func (c *Client) ReadFile(owner, name, path string) (string, bool, error) {
	var cr contentsResponse
	err := c.do("GET", "/repos/"+owner+"/"+name+"/contents/"+path, nil, &cr)
	if err != nil {
		var ge *Error
		if errorsAs(err, &ge) && ge.Status == 404 {
			return "", false, nil
		}
		return "", false, err
	}
	if cr.Encoding == "base64" {
		raw, decErr := base64.StdEncoding.DecodeString(strings.ReplaceAll(cr.Content, "\n", ""))
		if decErr != nil {
			return "", false, decErr
		}
		return string(raw), true, nil
	}
	return cr.Content, true, nil
}

// FindPRTemplate returns the first matching PR template under common paths.
func (c *Client) FindPRTemplate(owner, name string) string {
	for _, p := range []string{
		".github/PULL_REQUEST_TEMPLATE.md",
		".github/pull_request_template.md",
		"PULL_REQUEST_TEMPLATE.md",
		"pull_request_template.md",
		"docs/pull_request_template.md",
	} {
		body, ok, err := c.ReadFile(owner, name, p)
		if err == nil && ok {
			if t := strings.TrimSpace(body); t != "" {
				return t
			}
		}
	}
	return ""
}

// ── Webhooks ──────────────────────────────────────────────────────────────

type Hook struct {
	ID int64 `json:"id"`
}

type CreateHookInput struct {
	Name   string         `json:"name"`
	Active bool           `json:"active"`
	Events []string       `json:"events"`
	Config map[string]any `json:"config"`
}

func (c *Client) CreateRepoWebhook(owner, name, url, secret string, events []string) (Hook, error) {
	in := CreateHookInput{
		Name:   "web",
		Active: true,
		Events: events,
		Config: map[string]any{
			"url":          url,
			"content_type": "json",
			"secret":       secret,
			"insecure_ssl": "0",
		},
	}
	var h Hook
	return h, c.do("POST", "/repos/"+owner+"/"+name+"/hooks", in, &h)
}

// ── Diff ──────────────────────────────────────────────────────────────────

// CompareDiff returns the unified diff between base and head for use in AI
// prompts (mirrors `git diff base...head`).
func (c *Client) CompareDiff(owner, name, base, head string) (string, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s/repos/%s/%s/compare/%s...%s", apiBase, owner, name, base, head), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", &Error{Status: resp.StatusCode, Message: string(raw)}
	}
	return string(raw), nil
}

// errorsAs is a tiny stand-in for errors.As to avoid pulling in the package
// for a single use; kept private to this file.
func errorsAs(err error, target **Error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	return false
}
