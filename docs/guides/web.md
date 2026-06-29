# Web Skill

The web skill provides tools for fetching and analyzing web content. It extracts readable text, metadata, and links from web pages.

## Tools

### web_fetch

Fetch a URL and extract readable text content.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch |
| `include_links` | boolean | No | Include extracted links in response |
| `max_length` | integer | No | Maximum content length to return |

**Example:**

```json
{
  "url": "https://example.com/article",
  "include_links": true,
  "max_length": 10000
}
```

**Response:**

```json
{
  "title": "Article Title",
  "text": "The article content...",
  "links": [
    {"href": "https://example.com/page1", "text": "Related Article"}
  ],
  "url": "https://example.com/article",
  "truncated": false
}
```

### web_metadata

Extract metadata from a web page including title, description, and OpenGraph tags.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch |

**Response:**

```json
{
  "title": "Page Title",
  "description": "Page description",
  "keywords": "keyword1, keyword2",
  "canonical_url": "https://example.com/canonical",
  "open_graph": {
    "title": "OG Title",
    "description": "OG Description",
    "image": "https://example.com/image.png",
    "type": "article"
  },
  "twitter_card": {
    "card": "summary_large_image",
    "title": "Twitter Title"
  }
}
```

### web_links

Extract all links from a web page with resolved absolute URLs.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch |

**Response:**

```json
{
  "links": [
    {"href": "https://example.com/page1", "text": "Link Text"},
    {"href": "https://example.com/page2", "text": "Another Link"}
  ],
  "count": 2
}
```

### web_search_content

Search within fetched page content for specific terms.

**Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch |
| `query` | string | Yes | Search term to find |
| `context_lines` | integer | No | Number of context lines around matches (default: 2) |

**Response:**

```json
{
  "matches": [
    {
      "line": 42,
      "text": "The search term appears here",
      "context": ["Line before", "The search term appears here", "Line after"]
    }
  ],
  "count": 1
}
```

## Configuration

The web skill can be configured with custom settings:

```go
import "github.com/plexusone/omniagent/skills/web"

skill := web.NewSkill(web.Config{
    UserAgent:  "MyBot/1.0",
    Timeout:    30 * time.Second,
    MaxBodySize: 5 * 1024 * 1024, // 5MB
})
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `UserAgent` | string | `omniagent/1.0` | User-Agent header for requests |
| `Timeout` | duration | 30s | Request timeout |
| `MaxBodySize` | int64 | 5MB | Maximum response body size |

## Safety Features

The web skill includes safety features to prevent issues:

- **Bounded reads**: Response bodies are limited to prevent OOM
- **Timeout enforcement**: Requests timeout after configured duration
- **Script/style filtering**: JavaScript and CSS content is stripped from text extraction
- **Relative URL resolution**: All extracted links are resolved to absolute URLs

## Use Cases

### Link Previews

When a user shares a URL, fetch metadata for a preview:

```
User: Check out this article https://example.com/news

Agent: [Uses web_metadata to get title, description, image]
       "Example News Article - A summary of recent events..."
```

### Content Analysis

Search within a page for specific information:

```
User: Does this page mention pricing?

Agent: [Uses web_search_content with query="pricing"]
       "Found 3 mentions of pricing on the page..."
```

### Research

Extract all links from a page for research:

```
User: What resources does this documentation page link to?

Agent: [Uses web_links to extract all links]
       "This page links to 15 resources: API Reference, Tutorials..."
```
