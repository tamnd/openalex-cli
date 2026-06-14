// Package openalex is the library behind the openalex command line:
// the HTTP client, request shaping, and the typed data models for openalex.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package openalex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to OpenAlex. The mailto: hint opts
// into the polite pool for higher rate limits.
const DefaultUserAgent = "openalex-cli/dev (https://github.com/tamnd/openalex-cli; mailto:bot@example.com)"

// Host is the site this client talks to.
const Host = "api.openalex.org"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Config holds all tunables for the client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults for the OpenAlex API.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Client talks to OpenAlex over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Work is a single scholarly work (paper, preprint, book, dataset, etc.).
type Work struct {
	Rank         int      `json:"rank"`
	ID           string   `json:"id"` // OpenAlex ID e.g. W2741809807
	DOI          string   `json:"doi"`
	Title        string   `json:"title"`
	Year         int      `json:"year"`
	Type         string   `json:"type"` // article, preprint, book, etc.
	OpenAccess   bool     `json:"open_access"`
	CitedByCount int      `json:"cited_by_count"`
	Authors      []string `json:"authors"`       // first 3 author display names
	PrimaryVenue string   `json:"primary_venue"` // journal or source name
	URL          string   `json:"url"`           // doi URL or landing page
}

// Author is an individual researcher with career statistics.
type Author struct {
	Rank         int    `json:"rank"`
	ID           string `json:"id"` // OpenAlex ID e.g. A27320202
	Name         string `json:"name"`
	WorksCount   int    `json:"works_count"`
	CitedByCount int    `json:"cited_by_count"`
	HIndex       int    `json:"h_index"`     // from summary_stats.h_index
	Affiliation  string `json:"affiliation"` // last institution name
}

// Institution is a research institution (university, lab, etc.).
type Institution struct {
	Rank         int    `json:"rank"`
	ID           string `json:"id"` // OpenAlex ID e.g. I63966007
	Name         string `json:"name"`
	CountryCode  string `json:"country_code,omitempty"`
	Type         string `json:"type,omitempty"`
	WorksCount   int    `json:"works_count"`
	CitedByCount int    `json:"cited_by_count"`
	URL          string `json:"url,omitempty"` // homepage_url
}

// Topic is a research topic cluster.
type Topic struct {
	Rank        int    `json:"rank"`
	ID          string `json:"id"` // OpenAlex ID e.g. T12345
	Name        string `json:"name"`
	WorksCount  int    `json:"works_count"`
	Description string `json:"description,omitempty"`
	Field       string `json:"field,omitempty"` // subfield display name
}

// --- internal API response types ---

type apiWork struct {
	ID              string `json:"id"`
	DOI             string `json:"doi"`
	Title           string `json:"title"`
	DisplayName     string `json:"display_name"`
	PublicationYear int    `json:"publication_year"`
	Type            string `json:"type"`
	OpenAccess      struct {
		IsOA bool `json:"is_oa"`
	} `json:"open_access"`
	CitedByCount int `json:"cited_by_count"`
	Authorships  []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation *struct {
		Source *struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
		LandingPageURL string `json:"landing_page_url"`
	} `json:"primary_location"`
}

type apiAuthor struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	WorksCount   int    `json:"works_count"`
	CitedByCount int    `json:"cited_by_count"`
	SummaryStats struct {
		HIndex int `json:"h_index"`
	} `json:"summary_stats"`
	Affiliations []struct {
		Institution struct {
			DisplayName string `json:"display_name"`
		} `json:"institution"`
	} `json:"affiliations"`
}

type apiResponse[T any] struct {
	Meta struct {
		Count   int `json:"count"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
	Results []T `json:"results"`
}

type apiInstitution struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	CountryCode  string `json:"country_code"`
	Type         string `json:"type"`
	WorksCount   int    `json:"works_count"`
	CitedByCount int    `json:"cited_by_count"`
	HomepageURL  string `json:"homepage_url"`
}

type apiTopic struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	WorksCount   int    `json:"works_count"`
	Description  string `json:"description"`
	Subfield     struct {
		DisplayName string `json:"display_name"`
	} `json:"subfield"`
}

// SearchWorks queries the OpenAlex /works endpoint and returns up to limit results.
func (c *Client) SearchWorks(ctx context.Context, query string, limit int) ([]Work, error) {
	u := fmt.Sprintf("%s/works?search=%s&per-page=%d", c.cfg.BaseURL, url.QueryEscape(query), limit)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[apiWork]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode works: %w", err)
	}
	works := make([]Work, 0, len(resp.Results))
	for i, aw := range resp.Results {
		works = append(works, convertWork(i+1, aw))
	}
	return works, nil
}

// SearchAuthors queries the OpenAlex /authors endpoint and returns up to limit results.
func (c *Client) SearchAuthors(ctx context.Context, query string, limit int) ([]Author, error) {
	u := fmt.Sprintf("%s/authors?search=%s&per-page=%d", c.cfg.BaseURL, url.QueryEscape(query), limit)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[apiAuthor]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode authors: %w", err)
	}
	authors := make([]Author, 0, len(resp.Results))
	for i, aa := range resp.Results {
		authors = append(authors, convertAuthor(i+1, aa))
	}
	return authors, nil
}

// GetWork fetches a single work by OpenAlex ID or DOI.
func (c *Client) GetWork(ctx context.Context, id string) (*Work, error) {
	// Normalize: strip full URL prefix if given
	if strings.HasPrefix(id, "https://openalex.org/") {
		id = path.Base(id)
	}
	// DOI normalization
	var u string
	if strings.HasPrefix(id, "10.") {
		u = fmt.Sprintf("%s/works/https://doi.org/%s", c.cfg.BaseURL, url.QueryEscape(id))
	} else if strings.HasPrefix(id, "https://doi.org/") || strings.HasPrefix(id, "doi.org/") {
		u = fmt.Sprintf("%s/works/%s", c.cfg.BaseURL, url.QueryEscape(id))
	} else {
		u = fmt.Sprintf("%s/works/%s", c.cfg.BaseURL, id)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var aw apiWork
	if err := json.Unmarshal(body, &aw); err != nil {
		return nil, fmt.Errorf("decode work: %w", err)
	}
	w := convertWork(0, aw)
	return &w, nil
}

// SearchInstitutions queries the OpenAlex /institutions endpoint.
func (c *Client) SearchInstitutions(ctx context.Context, query string, limit int) ([]Institution, error) {
	u := fmt.Sprintf("%s/institutions?search=%s&per-page=%d", c.cfg.BaseURL, url.QueryEscape(query), limit)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[apiInstitution]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode institutions: %w", err)
	}
	out := make([]Institution, 0, len(resp.Results))
	for i, ai := range resp.Results {
		out = append(out, Institution{
			Rank:         i + 1,
			ID:           path.Base(ai.ID),
			Name:         ai.DisplayName,
			CountryCode:  ai.CountryCode,
			Type:         ai.Type,
			WorksCount:   ai.WorksCount,
			CitedByCount: ai.CitedByCount,
			URL:          ai.HomepageURL,
		})
	}
	return out, nil
}

// SearchTopics queries the OpenAlex /topics endpoint.
func (c *Client) SearchTopics(ctx context.Context, query string, limit int) ([]Topic, error) {
	u := fmt.Sprintf("%s/topics?search=%s&per-page=%d", c.cfg.BaseURL, url.QueryEscape(query), limit)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp apiResponse[apiTopic]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode topics: %w", err)
	}
	out := make([]Topic, 0, len(resp.Results))
	for i, at := range resp.Results {
		desc := at.Description
		if len(desc) > 200 {
			desc = desc[:200]
		}
		out = append(out, Topic{
			Rank:        i + 1,
			ID:          path.Base(at.ID),
			Name:        at.DisplayName,
			WorksCount:  at.WorksCount,
			Description: desc,
			Field:       at.Subfield.DisplayName,
		})
	}
	return out, nil
}

func convertWork(rank int, aw apiWork) Work {
	title := aw.DisplayName
	if title == "" {
		title = aw.Title
	}

	// Extract short ID from full URL e.g. https://openalex.org/W2741809807 → W2741809807
	id := path.Base(aw.ID)

	// Collect first 3 authors
	var authors []string
	for _, a := range aw.Authorships {
		if len(authors) >= 3 {
			break
		}
		if n := a.Author.DisplayName; n != "" {
			authors = append(authors, n)
		}
	}

	// Primary venue
	var venue string
	var landingURL string
	if aw.PrimaryLocation != nil {
		if aw.PrimaryLocation.Source != nil {
			venue = aw.PrimaryLocation.Source.DisplayName
		}
		landingURL = aw.PrimaryLocation.LandingPageURL
	}

	// URL: prefer DOI, fall back to landing page
	workURL := aw.DOI
	if workURL == "" {
		workURL = landingURL
	}

	return Work{
		Rank:         rank,
		ID:           id,
		DOI:          aw.DOI,
		Title:        title,
		Year:         aw.PublicationYear,
		Type:         aw.Type,
		OpenAccess:   aw.OpenAccess.IsOA,
		CitedByCount: aw.CitedByCount,
		Authors:      authors,
		PrimaryVenue: venue,
		URL:          workURL,
	}
}

func convertAuthor(rank int, aa apiAuthor) Author {
	id := path.Base(aa.ID)

	// Use last affiliation
	var affiliation string
	if len(aa.Affiliations) > 0 {
		affiliation = aa.Affiliations[len(aa.Affiliations)-1].Institution.DisplayName
	}

	return Author{
		Rank:         rank,
		ID:           id,
		Name:         aa.DisplayName,
		WorksCount:   aa.WorksCount,
		CitedByCount: aa.CitedByCount,
		HIndex:       aa.SummaryStats.HIndex,
		Affiliation:  affiliation,
	}
}

// get fetches url and returns the response body. It paces and retries on
// transient errors (429 / 5xx).
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// --- scaffold compatibility: Page type and GetPage/PageLinks kept for domain.go ---

// Page is used by the kit domain driver as the URI-addressable resource.
type Page struct {
	ID    string `json:"id" kit:"id"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty" kit:"body"`
}

// GetPage fetches one page by its path.
func (c *Client) GetPage(ctx context.Context, p string) (*Page, error) {
	p = strings.Trim(p, "/")
	u := BaseURL + "/" + p
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return &Page{ID: p, URL: u, Title: p, Body: pageText(body)}, nil
}

// PageLinks is kept for the kit domain driver.
func (c *Client) PageLinks(ctx context.Context, p string, limit int) ([]*Page, error) {
	p = strings.Trim(p, "/")
	body, err := c.get(ctx, BaseURL+"/"+p)
	if err != nil {
		return nil, err
	}
	var out []*Page
	seen := map[string]bool{}
	for _, lp := range linkPaths(body) {
		if seen[lp] {
			continue
		}
		seen[lp] = true
		out = append(out, &Page{ID: lp, URL: BaseURL + "/" + lp})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func pageText(body []byte) string {
	s := strings.Join(strings.Fields(tagRE.ReplaceAllString(string(body), " ")), " ")
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}
