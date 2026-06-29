package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestSkillInterface(t *testing.T) {
	skill := NewSkill(Config{})

	if skill.Name() != SkillName {
		t.Errorf("expected name %q, got %q", SkillName, skill.Name())
	}

	if skill.Description() == "" {
		t.Error("description should not be empty")
	}

	tools := skill.Tools()
	if len(tools) == 0 {
		t.Error("should have at least one tool")
	}

	expectedTools := map[string]bool{
		"web_fetch":          false,
		"web_metadata":       false,
		"web_links":          false,
		"web_search_content": false,
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool.Name()]; ok {
			expectedTools[tool.Name()] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

func TestInit(t *testing.T) {
	skill := NewSkill(Config{})

	if err := skill.Init(context.Background()); err != nil {
		t.Errorf("Init should succeed: %v", err)
	}

	if skill.client == nil {
		t.Error("client should be initialized")
	}

	if err := skill.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "simple title",
			html: "<html><head><title>Test Page</title></head></html>",
			want: "Test Page",
		},
		{
			name: "no title",
			html: "<html><head></head></html>",
			want: "",
		},
		{
			name: "title with whitespace",
			html: "<html><head><title>  Test Page  </title></head></html>",
			want: "Test Page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _ := html.Parse(strings.NewReader(tt.html))
			got := extractTitle(doc)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "simple text",
			html: "<html><body><p>Hello World</p></body></html>",
			want: "Hello World",
		},
		{
			name: "skip script",
			html: "<html><body><p>Hello</p><script>alert('hi')</script><p>World</p></body></html>",
			want: "Hello\nWorld",
		},
		{
			name: "skip style",
			html: "<html><body><p>Hello</p><style>.foo{}</style><p>World</p></body></html>",
			want: "Hello\nWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, _ := html.Parse(strings.NewReader(tt.html))
			got := extractText(doc)
			// Normalize whitespace for comparison
			got = strings.TrimSpace(got)
			got = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(got, "\n")
			if got != tt.want {
				t.Errorf("extractText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractLinks(t *testing.T) {
	htmlContent := `<html><body>
		<a href="/page1">Page 1</a>
		<a href="https://example.com/page2">Page 2</a>
	</body></html>`

	doc, _ := html.Parse(strings.NewReader(htmlContent))
	links := extractLinks(doc, "https://example.com")

	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}

	// First link should be resolved to absolute URL
	if links[0]["href"] != "https://example.com/page1" {
		t.Errorf("expected resolved URL, got %s", links[0]["href"])
	}

	if links[0]["text"] != "Page 1" {
		t.Errorf("expected 'Page 1', got %s", links[0]["text"])
	}
}

func TestExtractMetadata(t *testing.T) {
	htmlContent := `<html>
	<head>
		<title>Test Page</title>
		<meta name="description" content="A test page">
		<meta name="keywords" content="test, page">
		<meta property="og:title" content="OG Title">
		<meta property="og:image" content="https://example.com/image.png">
		<link rel="canonical" href="https://example.com/canonical">
	</head>
	</html>`

	doc, _ := html.Parse(strings.NewReader(htmlContent))
	metadata := extractMetadata(doc)

	if metadata["title"] != "Test Page" {
		t.Errorf("expected title 'Test Page', got %v", metadata["title"])
	}

	if metadata["description"] != "A test page" {
		t.Errorf("expected description 'A test page', got %v", metadata["description"])
	}

	og, ok := metadata["open_graph"].(map[string]string)
	if !ok {
		t.Error("expected open_graph map")
	} else {
		if og["title"] != "OG Title" {
			t.Errorf("expected og:title 'OG Title', got %v", og["title"])
		}
	}
}

func TestSearchText(t *testing.T) {
	text := `Line 1: Hello
Line 2: World
Line 3: Hello World
Line 4: Foo
Line 5: Bar`

	result := searchText(text, "Hello", 1)

	matches, ok := result["matches"].([]map[string]any)
	if !ok {
		t.Fatal("expected matches array")
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	count, ok := result["count"].(int)
	if !ok || count != 2 {
		t.Errorf("expected count 2, got %v", result["count"])
	}
}

func TestHandleFetch(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(`<html><head><title>Test</title></head><body><p>Hello World</p></body></html>`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	skill := NewSkill(Config{})
	if err := skill.Init(context.Background()); err != nil {
		t.Fatalf("failed to init skill: %v", err)
	}

	result, err := skill.handleFetch(context.Background(), map[string]any{
		"url": server.URL,
	})

	if err != nil {
		t.Fatalf("handleFetch failed: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}

	if m["title"] != "Test" {
		t.Errorf("expected title 'Test', got %v", m["title"])
	}

	text, ok := m["text"].(string)
	if !ok || !strings.Contains(text, "Hello World") {
		t.Errorf("expected text to contain 'Hello World', got %v", m["text"])
	}
}
