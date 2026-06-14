package openalex_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/openalex-cli/openalex"
)

const fakeWorksJSON = `{
  "meta": {"count": 2, "page": 1, "per_page": 2},
  "results": [
    {
      "id": "https://openalex.org/W2741809807",
      "doi": "https://doi.org/10.48550/arxiv.1201.0490",
      "title": "Scikit-learn: Machine Learning in Python",
      "display_name": "Scikit-learn: Machine Learning in Python",
      "publication_year": 2012,
      "type": "article",
      "open_access": {"is_oa": true},
      "cited_by_count": 76543,
      "authorships": [
        {"author": {"display_name": "Fabian Pedregosa"}},
        {"author": {"display_name": "Gaël Varoquaux"}},
        {"author": {"display_name": "Alexandre Gramfort"}},
        {"author": {"display_name": "Vincent Michel"}}
      ],
      "primary_location": {
        "source": {"display_name": "Journal of Machine Learning Research"},
        "landing_page_url": "https://jmlr.org/papers/v12/pedregosa11a.html"
      }
    },
    {
      "id": "https://openalex.org/W2964220222",
      "doi": "https://doi.org/10.48550/arxiv.1706.03762",
      "title": "Attention Is All You Need",
      "display_name": "Attention Is All You Need",
      "publication_year": 2017,
      "type": "preprint",
      "open_access": {"is_oa": true},
      "cited_by_count": 99999,
      "authorships": [
        {"author": {"display_name": "Ashish Vaswani"}}
      ],
      "primary_location": {
        "source": null,
        "landing_page_url": "https://arxiv.org/abs/1706.03762"
      }
    }
  ]
}`

const fakeAuthorsJSON = `{
  "meta": {"count": 2, "page": 1, "per_page": 2},
  "results": [
    {
      "id": "https://openalex.org/A27320202",
      "display_name": "Alan Turing",
      "works_count": 23,
      "cited_by_count": 9876,
      "summary_stats": {"h_index": 12},
      "last_known_institutions": [
        {"display_name": "University of Manchester"}
      ],
      "x_concepts": [
        {"display_name": "Computer science", "score": 0.9},
        {"display_name": "Mathematics", "score": 0.8},
        {"display_name": "Artificial intelligence", "score": 0.7},
        {"display_name": "Logic", "score": 0.6}
      ]
    },
    {
      "id": "https://openalex.org/A12345678",
      "display_name": "Alan Smith",
      "works_count": 5,
      "cited_by_count": 100,
      "summary_stats": {"h_index": 3},
      "last_known_institutions": [],
      "x_concepts": []
    }
  ]
}`

const emptyWorksJSON = `{"meta":{"count":0,"page":1,"per_page":1},"results":[]}`

const fakeSourcesJSON = `{
  "meta": {"count": 1, "page": 1, "per_page": 1},
  "results": [
    {
      "id": "https://openalex.org/S137773608",
      "display_name": "Nature",
      "host_organization_name": "Springer Nature",
      "works_count": 120000,
      "cited_by_count": 10000000,
      "is_oa": false
    }
  ]
}`

const fakeInstitutionsJSON = `{
  "meta": {"count": 1, "page": 1, "per_page": 1},
  "results": [
    {
      "id": "https://openalex.org/I63966007",
      "display_name": "Massachusetts Institute of Technology",
      "country_code": "US",
      "type": "education",
      "works_count": 234567,
      "cited_by_count": 12345678,
      "homepage_url": "http://web.mit.edu/"
    }
  ]
}`

const fakeTopicsJSON = `{
  "meta": {"count": 1, "page": 1, "per_page": 1},
  "results": [
    {
      "id": "https://openalex.org/T12345",
      "display_name": "Climate Change",
      "works_count": 234567,
      "description": "Research on anthropogenic climate change and its effects.",
      "subfield": {"display_name": "Environmental Science"}
    }
  ]
}`

const fakeSingleWorkJSON = `{
  "id": "https://openalex.org/W2741809807",
  "doi": "https://doi.org/10.48550/arxiv.1201.0490",
  "title": "Scikit-learn: Machine Learning in Python",
  "display_name": "Scikit-learn: Machine Learning in Python",
  "publication_year": 2012,
  "type": "article",
  "open_access": {"is_oa": true},
  "cited_by_count": 76543,
  "authorships": [
    {"author": {"display_name": "Fabian Pedregosa"}}
  ],
  "primary_location": {
    "source": {"display_name": "Journal of Machine Learning Research"},
    "landing_page_url": "https://jmlr.org/papers/v12/pedregosa11a.html"
  }
}`

func newTestClient(ts *httptest.Server) *openalex.Client {
	cfg := openalex.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return openalex.NewClient(cfg)
}

func TestSearchWorksSendsUserAgent(t *testing.T) {
	var got string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, emptyWorksJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.SearchWorks(context.Background(), "test", 1, 0)
	if !strings.Contains(got, "openalex-cli") {
		t.Errorf("User-Agent = %q, want to contain openalex-cli", got)
	}
}

func TestSearchWorksParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeWorksJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	works, err := c.SearchWorks(context.Background(), "ml", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 2 {
		t.Fatalf("want 2 works, got %d", len(works))
	}

	// First work: Scikit-learn
	w0 := works[0]
	if w0.ID != "W2741809807" {
		t.Errorf("ID = %q, want W2741809807", w0.ID)
	}
	if w0.Year != 2012 {
		t.Errorf("Year = %d, want 2012", w0.Year)
	}
	if w0.Type != "article" {
		t.Errorf("Type = %q, want article", w0.Type)
	}
	if !w0.OpenAccess {
		t.Error("OpenAccess should be true")
	}
	if w0.CitedByCount != 76543 {
		t.Errorf("CitedByCount = %d, want 76543", w0.CitedByCount)
	}
	if len(w0.Authors) != 3 {
		t.Errorf("Authors count = %d, want 3 (capped at 3)", len(w0.Authors))
	}
	if w0.Authors[0] != "Fabian Pedregosa" {
		t.Errorf("Authors[0] = %q", w0.Authors[0])
	}
	if w0.PrimaryVenue != "Journal of Machine Learning Research" {
		t.Errorf("PrimaryVenue = %q", w0.PrimaryVenue)
	}
	if w0.URL != "https://doi.org/10.48550/arxiv.1201.0490" {
		t.Errorf("URL = %q", w0.URL)
	}
	if w0.Rank != 1 {
		t.Errorf("Rank = %d, want 1", w0.Rank)
	}

	// Second work: Attention (source is null, landing_page_url is fallback)
	w1 := works[1]
	if w1.ID != "W2964220222" {
		t.Errorf("ID = %q, want W2964220222", w1.ID)
	}
	if w1.PrimaryVenue != "" {
		t.Errorf("PrimaryVenue should be empty for null source, got %q", w1.PrimaryVenue)
	}
	// DOI is set, so URL should be the DOI
	if w1.URL != "https://doi.org/10.48550/arxiv.1706.03762" {
		t.Errorf("URL = %q", w1.URL)
	}
	if w1.Rank != 2 {
		t.Errorf("Rank = %d, want 2", w1.Rank)
	}
}

func TestSearchWorksLimitRespected(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, emptyWorksJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.SearchWorks(context.Background(), "test", 7, 0)
	if !strings.Contains(gotURL, "per-page=7") {
		t.Errorf("URL = %q, want per-page=7", gotURL)
	}
}

func TestSearchWorksRetriesOn503(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	cfg := openalex.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 2
	c := openalex.NewClient(cfg)
	_, err := c.SearchWorks(context.Background(), "test", 1, 0)
	if err == nil {
		t.Fatal("expected error on persistent 503")
	}
	// initial attempt + 2 retries = 3 total calls
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestSearchAuthorsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeAuthorsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	authors, err := c.SearchAuthors(context.Background(), "turing", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(authors) != 2 {
		t.Fatalf("want 2 authors, got %d", len(authors))
	}

	a0 := authors[0]
	if a0.ID != "A27320202" {
		t.Errorf("ID = %q, want A27320202", a0.ID)
	}
	if a0.Name != "Alan Turing" {
		t.Errorf("Name = %q, want Alan Turing", a0.Name)
	}
	if a0.WorksCount != 23 {
		t.Errorf("WorksCount = %d, want 23", a0.WorksCount)
	}
	if a0.CitedByCount != 9876 {
		t.Errorf("CitedByCount = %d, want 9876", a0.CitedByCount)
	}
	if a0.HIndex != 12 {
		t.Errorf("HIndex = %d, want 12", a0.HIndex)
	}
	if a0.Institution != "University of Manchester" {
		t.Errorf("Institution = %q, want University of Manchester", a0.Institution)
	}
	if a0.Concepts != "Computer science, Mathematics, Artificial intelligence" {
		t.Errorf("Concepts = %q, want first 3 joined", a0.Concepts)
	}
	if a0.Rank != 1 {
		t.Errorf("Rank = %d, want 1", a0.Rank)
	}

	// Second author: no institutions or concepts
	a1 := authors[1]
	if a1.Institution != "" {
		t.Errorf("Institution should be empty for no institutions, got %q", a1.Institution)
	}
	if a1.Concepts != "" {
		t.Errorf("Concepts should be empty, got %q", a1.Concepts)
	}
}

func TestSearchInstitutionsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeInstitutionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	insts, err := c.SearchInstitutions(context.Background(), "MIT", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(insts) != 1 {
		t.Fatalf("want 1 institution, got %d", len(insts))
	}
	i0 := insts[0]
	if i0.ID != "I63966007" {
		t.Errorf("ID = %q, want I63966007", i0.ID)
	}
	if i0.Name != "Massachusetts Institute of Technology" {
		t.Errorf("Name = %q", i0.Name)
	}
	if i0.Country != "US" {
		t.Errorf("Country = %q, want US", i0.Country)
	}
	if i0.Works != 234567 {
		t.Errorf("Works = %d, want 234567", i0.Works)
	}
	if i0.Citations != 12345678 {
		t.Errorf("Citations = %d, want 12345678", i0.Citations)
	}
	if i0.URL != "http://web.mit.edu/" {
		t.Errorf("URL = %q, want http://web.mit.edu/", i0.URL)
	}
	if i0.Rank != 1 {
		t.Errorf("Rank = %d, want 1", i0.Rank)
	}
}

func TestSearchTopicsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeTopicsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	topics, err := c.SearchTopics(context.Background(), "climate", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 {
		t.Fatalf("want 1 topic, got %d", len(topics))
	}
	t0 := topics[0]
	if t0.ID != "T12345" {
		t.Errorf("ID = %q, want T12345", t0.ID)
	}
	if t0.Name != "Climate Change" {
		t.Errorf("Name = %q, want Climate Change", t0.Name)
	}
	if t0.WorksCount != 234567 {
		t.Errorf("WorksCount = %d, want 234567", t0.WorksCount)
	}
	if t0.Field != "Environmental Science" {
		t.Errorf("Field = %q, want Environmental Science", t0.Field)
	}
	if t0.Rank != 1 {
		t.Errorf("Rank = %d, want 1", t0.Rank)
	}
}

func TestGetWorkByID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSingleWorkJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	work, err := c.GetWork(context.Background(), "W2741809807")
	if err != nil {
		t.Fatal(err)
	}
	if work.ID != "W2741809807" {
		t.Errorf("ID = %q, want W2741809807", work.ID)
	}
	if work.Year != 2012 {
		t.Errorf("Year = %d, want 2012", work.Year)
	}
	if work.CitedByCount != 76543 {
		t.Errorf("CitedByCount = %d, want 76543", work.CitedByCount)
	}
}

func TestSearchJournalsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeSourcesJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	journals, err := c.SearchJournals(context.Background(), "nature", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(journals) != 1 {
		t.Fatalf("want 1 journal, got %d", len(journals))
	}
	j0 := journals[0]
	if j0.ID != "S137773608" {
		t.Errorf("ID = %q, want S137773608", j0.ID)
	}
	if j0.Name != "Nature" {
		t.Errorf("Name = %q, want Nature", j0.Name)
	}
	if j0.Publisher != "Springer Nature" {
		t.Errorf("Publisher = %q, want Springer Nature", j0.Publisher)
	}
	if j0.Works != 120000 {
		t.Errorf("Works = %d, want 120000", j0.Works)
	}
	if j0.Citations != 10000000 {
		t.Errorf("Citations = %d, want 10000000", j0.Citations)
	}
	if j0.IsOA {
		t.Error("IsOA should be false")
	}
	if j0.Rank != 1 {
		t.Errorf("Rank = %d, want 1", j0.Rank)
	}
}

func TestSearchJournalsURLContainsQuery(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, fakeSourcesJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.SearchJournals(context.Background(), "science", 5)
	if !strings.Contains(gotURL, "science") {
		t.Errorf("URL = %q, want to contain science", gotURL)
	}
	if !strings.Contains(gotURL, "per-page=5") {
		t.Errorf("URL = %q, want to contain per-page=5", gotURL)
	}
}

func TestSearchWorksYearFilter(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, emptyWorksJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.SearchWorks(context.Background(), "ml", 5, 2023)
	if !strings.Contains(gotURL, "publication_year%3A2023") && !strings.Contains(gotURL, "publication_year:2023") {
		t.Errorf("URL = %q, want to contain publication_year filter for 2023", gotURL)
	}
}

func TestSearchInstitutionsURLContainsQuery(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, fakeInstitutionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.SearchInstitutions(context.Background(), "Stanford", 5)
	if !strings.Contains(gotURL, "Stanford") {
		t.Errorf("URL = %q, want to contain Stanford", gotURL)
	}
	if !strings.Contains(gotURL, "per-page=5") {
		t.Errorf("URL = %q, want to contain per-page=5", gotURL)
	}
}
