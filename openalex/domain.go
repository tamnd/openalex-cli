package openalex

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes openalex as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/openalex-cli/openalex"
//
// The init below registers it; the host then dereferences openalex:// URIs by
// routing to the operations Register installs. The same Domain also builds the
// standalone openalex binary (see cli.NewApp), so the binary and a host share
// one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the openalex driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "openalex",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "openalex",
			Short:  "Browse OpenAlex scholarly papers from the command line.",
			Long: `A command line for OpenAlex (api.openalex.org).

openalex reads public scholarly data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.`,
			Site: "api.openalex.org",
			Repo: "https://github.com/tamnd/openalex-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// works: search scholarly works
	kit.Handle(app, kit.OpMeta{Name: "works", Group: "search", List: true,
		Summary: "Search scholarly works (papers, preprints, books, datasets)",
		Args:    []kit.Arg{{Name: "query", Help: "search terms"}}}, searchWorks)

	// work: fetch a single work by ID or DOI
	kit.Handle(app, kit.OpMeta{Name: "work", Group: "search", Single: true,
		Summary: "Fetch a work by OpenAlex ID or DOI", URIType: "work", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "OpenAlex ID (W2741809807) or DOI"}}}, getWork)

	// authors: search authors
	kit.Handle(app, kit.OpMeta{Name: "authors", Group: "search", List: true,
		Summary: "Search authors by name",
		Args:    []kit.Arg{{Name: "query", Help: "author name or terms"}}}, searchAuthors)

	// institutions: search institutions
	kit.Handle(app, kit.OpMeta{Name: "institutions", Group: "search", List: true,
		Summary: "Search research institutions",
		Args:    []kit.Arg{{Name: "query", Help: "institution name or terms"}}}, searchInstitutions)

	// topics: search topics
	kit.Handle(app, kit.OpMeta{Name: "topics", Group: "search", List: true,
		Summary: "Search research topics",
		Args:    []kit.Arg{{Name: "query", Help: "topic name or terms"}}}, searchTopics)

	// page: fetch a raw page by path (scaffold fallback, kept for URI driver)
	kit.Handle(app, kit.OpMeta{Name: "page", Group: "read", Single: true,
		Summary: "Fetch a page by path or URL", URIType: "page", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "page path or URL"}}}, getPage)

	// links: list pages a page links to
	kit.Handle(app, kit.OpMeta{Name: "links", Group: "read", List: true,
		Summary: "List the pages a page links to", URIType: "page",
		Args: []kit.Arg{{Name: "ref", Help: "page path or URL"}}}, listLinks)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient(DefaultConfig())
	if cfg.UserAgent != "" {
		c.cfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.cfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.cfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.cfg.Timeout = cfg.Timeout
		c.http.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input types ---

type worksIn struct {
	Query  string  `kit:"arg" help:"search terms"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type workIn struct {
	ID     string  `kit:"arg" help:"OpenAlex ID (W2741809807) or DOI"`
	Client *Client `kit:"inject"`
}

type authorsIn struct {
	Query  string  `kit:"arg" help:"author name or terms"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type institutionsIn struct {
	Query  string  `kit:"arg" help:"institution name or terms"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type topicsIn struct {
	Query  string  `kit:"arg" help:"topic name or terms"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type pageRef struct {
	Ref    string  `kit:"arg" help:"page path or URL"`
	Client *Client `kit:"inject"`
}

type listRef struct {
	Ref    string  `kit:"arg" help:"page path or URL"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchWorks(ctx context.Context, in worksIn, emit func(*Work) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	works, err := in.Client.SearchWorks(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range works {
		if err := emit(&works[i]); err != nil {
			return err
		}
	}
	return nil
}

func getWork(ctx context.Context, in workIn, emit func(*Work) error) error {
	w, err := in.Client.GetWork(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(w)
}

func searchAuthors(ctx context.Context, in authorsIn, emit func(*Author) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	authors, err := in.Client.SearchAuthors(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range authors {
		if err := emit(&authors[i]); err != nil {
			return err
		}
	}
	return nil
}

func searchInstitutions(ctx context.Context, in institutionsIn, emit func(*Institution) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	insts, err := in.Client.SearchInstitutions(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range insts {
		if err := emit(&insts[i]); err != nil {
			return err
		}
	}
	return nil
}

func searchTopics(ctx context.Context, in topicsIn, emit func(*Topic) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 25
	}
	topics, err := in.Client.SearchTopics(ctx, in.Query, limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range topics {
		if err := emit(&topics[i]); err != nil {
			return err
		}
	}
	return nil
}

func getPage(ctx context.Context, in pageRef, emit func(*Page) error) error {
	p, err := in.Client.GetPage(ctx, pagePath(in.Ref))
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func listLinks(ctx context.Context, in listRef, emit func(*Page) error) error {
	pages, err := in.Client.PageLinks(ctx, pagePath(in.Ref), in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range pages {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// --- URI driver ---

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	id = pagePath(input)
	if id == "" {
		return "", "", errs.Usage("unrecognized openalex reference: %q", input)
	}
	return "page", id, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "page" {
		return "", errs.Usage("openalex has no resource type %q", uriType)
	}
	return BaseURL + "/" + strings.Trim(id, "/"), nil
}

// --- helpers ---

var (
	hrefRE = regexp.MustCompile(`href="(/[^":#?]+)"`)
	tagRE  = regexp.MustCompile(`<[^>]+>`)
)

func pagePath(input string) string {
	input = strings.TrimSpace(input)
	if u, err := url.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return strings.Trim(u.Path, "/")
	}
	return strings.Trim(input, "/")
}

func linkPaths(body []byte) []string {
	var out []string
	for _, m := range hrefRE.FindAllSubmatch(body, -1) {
		if p := strings.Trim(string(m[1]), "/"); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mapErr(err error) error {
	return err
}
