// Package web provides a compiled skill for web content extraction and analysis.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/plexusone/omniagent/internal/httputil"
	"github.com/plexusone/omniskill/skill"
	"golang.org/x/net/html"
)

const (
	// SkillName is the name of the web skill.
	SkillName = "web"

	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxSize is the default maximum response size (5MB).
	DefaultMaxSize = 5 * 1024 * 1024

	// DefaultUserAgent is the default user agent string.
	DefaultUserAgent = "OmniAgent/1.0 (Web Skill)"
)

// Skill implements compiled.Skill for web content operations.
type Skill struct {
	client  *http.Client
	config  Config
	timeout time.Duration
	maxSize int64
}

// Config configures the web skill.
type Config struct {
	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// MaxSize is the maximum response size in bytes.
	MaxSize int64

	// UserAgent is the user agent string for requests.
	UserAgent string
}

// NewSkill creates a new web skill.
func NewSkill(cfg Config) *Skill {
	return &Skill{
		config: cfg,
	}
}

// Name implements compiled.Skill.
func (s *Skill) Name() string {
	return SkillName
}

// Description implements compiled.Skill.
func (s *Skill) Description() string {
	return "Fetch and analyze web content: extract text, metadata, and links from URLs"
}

// Init implements compiled.Skill.
func (s *Skill) Init(ctx context.Context) error {
	s.timeout = s.config.Timeout
	if s.timeout == 0 {
		s.timeout = DefaultTimeout
	}

	s.maxSize = s.config.MaxSize
	if s.maxSize == 0 {
		s.maxSize = DefaultMaxSize
	}

	userAgent := s.config.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	s.client = &http.Client{
		Timeout: s.timeout,
		Transport: &userAgentTransport{
			base:      http.DefaultTransport,
			userAgent: userAgent,
		},
	}

	return nil
}

// Close implements compiled.Skill.
func (s *Skill) Close() error {
	return nil
}

// Tools implements compiled.Skill.
func (s *Skill) Tools() []skill.Tool {
	return []skill.Tool{
		&webTool{
			name:        "web_fetch",
			description: "Fetch a URL and extract readable text content",
			params: map[string]skill.Parameter{
				"url": {
					Type:        "string",
					Description: "The URL to fetch",
					Required:    true,
				},
				"include_links": {
					Type:        "boolean",
					Description: "Include extracted links in the response (default: false)",
				},
				"max_length": {
					Type:        "integer",
					Description: "Maximum content length to return (default: 10000)",
				},
			},
			handler: s.handleFetch,
		},
		&webTool{
			name:        "web_metadata",
			description: "Extract metadata from a URL (title, description, images, Open Graph)",
			params: map[string]skill.Parameter{
				"url": {
					Type:        "string",
					Description: "The URL to analyze",
					Required:    true,
				},
			},
			handler: s.handleMetadata,
		},
		&webTool{
			name:        "web_links",
			description: "Extract all links from a URL",
			params: map[string]skill.Parameter{
				"url": {
					Type:        "string",
					Description: "The URL to extract links from",
					Required:    true,
				},
				"filter": {
					Type:        "string",
					Description: "Filter links by pattern (regex)",
				},
			},
			handler: s.handleLinks,
		},
		&webTool{
			name:        "web_search_content",
			description: "Fetch a URL and search for specific text or patterns",
			params: map[string]skill.Parameter{
				"url": {
					Type:        "string",
					Description: "The URL to search",
					Required:    true,
				},
				"query": {
					Type:        "string",
					Description: "Text or regex pattern to search for",
					Required:    true,
				},
				"context_lines": {
					Type:        "integer",
					Description: "Number of lines of context around matches (default: 2)",
				},
			},
			handler: s.handleSearchContent,
		},
	}
}

// Handlers

type fetchInput struct {
	URL          string `json:"url"`
	IncludeLinks bool   `json:"include_links"`
	MaxLength    int    `json:"max_length"`
}

func (s *Skill) handleFetch(ctx context.Context, params map[string]any) (any, error) {
	var input fetchInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	if input.MaxLength == 0 {
		input.MaxLength = 10000
	}

	// Validate URL
	if _, err := url.Parse(input.URL); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch content
	body, contentType, err := s.fetchURL(ctx, input.URL)
	if err != nil {
		return nil, err
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		// Return raw text for non-HTML content
		if input.MaxLength > 0 && len(body) > input.MaxLength {
			body = body[:input.MaxLength] + "..."
		}
		return map[string]any{
			"url":          input.URL,
			"content_type": contentType,
			"text":         body,
		}, nil
	}

	// Extract readable content
	text := extractText(doc)
	if input.MaxLength > 0 && len(text) > input.MaxLength {
		text = text[:input.MaxLength] + "..."
	}

	result := map[string]any{
		"url":          input.URL,
		"content_type": contentType,
		"title":        extractTitle(doc),
		"text":         text,
	}

	if input.IncludeLinks {
		result["links"] = extractLinks(doc, input.URL)
	}

	return result, nil
}

type metadataInput struct {
	URL string `json:"url"`
}

func (s *Skill) handleMetadata(ctx context.Context, params map[string]any) (any, error) {
	var input metadataInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	body, _, err := s.fetchURL(ctx, input.URL)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	metadata := extractMetadata(doc)
	metadata["url"] = input.URL

	return metadata, nil
}

type linksInput struct {
	URL    string `json:"url"`
	Filter string `json:"filter"`
}

func (s *Skill) handleLinks(ctx context.Context, params map[string]any) (any, error) {
	var input linksInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	body, _, err := s.fetchURL(ctx, input.URL)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	links := extractLinks(doc, input.URL)

	// Apply filter if provided
	if input.Filter != "" {
		re, err := regexp.Compile(input.Filter)
		if err != nil {
			return nil, fmt.Errorf("invalid filter pattern: %w", err)
		}

		var filtered []map[string]string
		for _, link := range links {
			if re.MatchString(link["href"]) || re.MatchString(link["text"]) {
				filtered = append(filtered, link)
			}
		}
		links = filtered
	}

	return map[string]any{
		"url":   input.URL,
		"links": links,
		"count": len(links),
	}, nil
}

type searchContentInput struct {
	URL          string `json:"url"`
	Query        string `json:"query"`
	ContextLines int    `json:"context_lines"`
}

func (s *Skill) handleSearchContent(ctx context.Context, params map[string]any) (any, error) {
	var input searchContentInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	if input.ContextLines == 0 {
		input.ContextLines = 2
	}

	body, _, err := s.fetchURL(ctx, input.URL)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		// Search raw text
		return searchText(body, input.Query, input.ContextLines), nil
	}

	text := extractText(doc)
	return searchText(text, input.Query, input.ContextLines), nil
}

// Helper functions

func (s *Skill) fetchURL(ctx context.Context, urlStr string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Use bounded read
	body, err := httputil.ReadLimited(resp.Body, s.maxSize)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	return string(body), contentType, nil
}

func extractTitle(doc *html.Node) string {
	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return strings.TrimSpace(title)
}

func extractText(doc *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)

	// Tags to skip entirely
	skipTags := map[string]bool{
		"script": true, "style": true, "noscript": true,
		"nav": true, "footer": true, "header": true,
		"aside": true, "form": true,
	}

	// Tags that add line breaks
	blockTags := map[string]bool{
		"p": true, "div": true, "br": true, "li": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"tr": true, "td": true, "th": true,
		"article": true, "section": true,
	}

	f = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n.Type == html.ElementNode && blockTags[n.Data] {
			sb.WriteString("\n")
		}
	}

	f(doc)

	// Clean up whitespace
	text := sb.String()
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func extractLinks(doc *html.Node, baseURL string) []map[string]string {
	var links []map[string]string
	base, _ := url.Parse(baseURL)

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href, text string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
					break
				}
			}
			if href != "" {
				// Resolve relative URLs
				if parsed, err := url.Parse(href); err == nil {
					href = base.ResolveReference(parsed).String()
				}

				// Extract link text
				var extractText func(*html.Node) string
				extractText = func(n *html.Node) string {
					if n.Type == html.TextNode {
						return n.Data
					}
					var s string
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						s += extractText(c)
					}
					return s
				}
				text = strings.TrimSpace(extractText(n))

				links = append(links, map[string]string{
					"href": href,
					"text": text,
				})
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)
	return links
}

func extractMetadata(doc *html.Node) map[string]any {
	metadata := map[string]any{}
	og := map[string]string{}
	twitter := map[string]string{}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if n.FirstChild != nil {
					metadata["title"] = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				var name, property, content string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "name":
						name = attr.Val
					case "property":
						property = attr.Val
					case "content":
						content = attr.Val
					}
				}

				if content != "" {
					if name == "description" {
						metadata["description"] = content
					} else if name == "keywords" {
						metadata["keywords"] = content
					} else if name == "author" {
						metadata["author"] = content
					} else if strings.HasPrefix(property, "og:") {
						og[strings.TrimPrefix(property, "og:")] = content
					} else if strings.HasPrefix(name, "twitter:") {
						twitter[strings.TrimPrefix(name, "twitter:")] = content
					}
				}
			case "link":
				var rel, href string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "rel":
						rel = attr.Val
					case "href":
						href = attr.Val
					}
				}
				if rel == "canonical" && href != "" {
					metadata["canonical"] = href
				} else if rel == "icon" && href != "" {
					metadata["favicon"] = href
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)

	if len(og) > 0 {
		metadata["open_graph"] = og
	}
	if len(twitter) > 0 {
		metadata["twitter"] = twitter
	}

	return metadata
}

func searchText(text, query string, contextLines int) map[string]any {
	lines := strings.Split(text, "\n")
	var matches []map[string]any

	// Try as regex first, fall back to literal search
	re, err := regexp.Compile("(?i)" + query)
	if err != nil {
		// Literal search
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))
	}

	for i, line := range lines {
		if re.MatchString(line) {
			// Get context
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines + 1
			if end > len(lines) {
				end = len(lines)
			}

			context := strings.Join(lines[start:end], "\n")
			matches = append(matches, map[string]any{
				"line":    i + 1,
				"match":   line,
				"context": context,
			})
		}
	}

	return map[string]any{
		"query":   query,
		"matches": matches,
		"count":   len(matches),
	}
}

func mapToStruct(m map[string]any, v any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// userAgentTransport adds a custom User-Agent header to requests.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(req)
}

// webTool implements skill.Tool.
type webTool struct {
	name        string
	description string
	params      map[string]skill.Parameter
	handler     func(ctx context.Context, params map[string]any) (any, error)
}

func (t *webTool) Name() string {
	return t.name
}

func (t *webTool) Description() string {
	return t.description
}

func (t *webTool) Parameters() map[string]skill.Parameter {
	return t.params
}

func (t *webTool) Call(ctx context.Context, params map[string]any) (any, error) {
	return t.handler(ctx, params)
}
