# GitHub Skill

OmniAgent includes a GitHub skill for interacting with GitHub repositories, issues, pull requests, and code search. The skill uses the GitHub API to enable agents to manage development workflows.

## Overview

The GitHub skill provides:

- **Issue Management** - List, create, update, and comment on issues
- **Pull Request Operations** - List, view, and comment on PRs
- **Code Search** - Search for code across repositories
- **Issue Search** - Search for issues and PRs with queries

## Quick Start

### Enable GitHub Skill

```go
import (
    "github.com/plexusone/omniagent/agent"
    "github.com/plexusone/omniagent/skills/github"
)

githubSkill := github.NewSkill(github.Config{
    // Token is optional - uses GITHUB_TOKEN or GH_TOKEN env var if not set
    Token: os.Getenv("GITHUB_TOKEN"),
})

a, err := agent.New(config,
    agent.WithCompiledSkill(githubSkill),
)
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | GitHub personal access token |
| `GH_TOKEN` | Alternative token variable (GitHub CLI compatible) |

### Usage in Conversation

Once enabled, the agent can use GitHub tools:

```
User: Show me open issues in plexusone/omniagent
Agent: [calls github_list_issues] Here are the open issues...

User: Create an issue about the login bug
Agent: [calls github_create_issue] Created issue #123: "Login bug in authentication flow"

User: Search for code containing "handleAuth" in our repos
Agent: [calls github_search_code] Found 5 matches...
```

## Configuration

```go
type Config struct {
    // Token is the GitHub personal access token.
    // If empty, uses GITHUB_TOKEN or GH_TOKEN environment variable.
    Token string

    // BaseURL is the GitHub API base URL.
    // If empty, uses the public GitHub API (api.github.com).
    // Set this for GitHub Enterprise installations.
    BaseURL string
}
```

### GitHub Enterprise

For GitHub Enterprise installations:

```go
githubSkill := github.NewSkill(github.Config{
    Token:   os.Getenv("GHE_TOKEN"),
    BaseURL: "https://github.mycompany.com/api/v3",
})
```

## GitHub Tools

The GitHub skill provides ten tools:

### github_list_issues

List issues in a repository.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner (user or organization) |
| `repo` | string | Yes | Repository name |
| `state` | string | No | Issue state: `open`, `closed`, or `all` (default: `open`) |
| `labels` | string | No | Comma-separated list of label names |
| `per_page` | integer | No | Results per page (default: 30, max: 100) |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "state": "open",
  "labels": "bug,high-priority"
}
```

### github_get_issue

Get a specific issue by number.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `number` | integer | Yes | Issue number |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "number": 42
}
```

### github_create_issue

Create a new issue.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `title` | string | Yes | Issue title |
| `body` | string | No | Issue body (Markdown supported) |
| `labels` | array | No | Labels to apply |
| `assignees` | array | No | Users to assign |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "title": "Bug: Login fails with special characters",
  "body": "## Description\n\nLogin fails when password contains `&` or `=`.\n\n## Steps to Reproduce\n1. ...",
  "labels": ["bug", "authentication"],
  "assignees": ["johnwang"]
}
```

### github_update_issue

Update an existing issue.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `number` | integer | Yes | Issue number |
| `title` | string | No | New title |
| `body` | string | No | New body |
| `state` | string | No | New state: `open` or `closed` |
| `labels` | array | No | Labels to set (replaces existing) |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "number": 42,
  "state": "closed",
  "labels": ["bug", "fixed"]
}
```

### github_add_issue_comment

Add a comment to an issue.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `number` | integer | Yes | Issue number |
| `body` | string | Yes | Comment body (Markdown supported) |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "number": 42,
  "body": "Fixed in PR #45. Closing this issue."
}
```

### github_list_pull_requests

List pull requests in a repository.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `state` | string | No | PR state: `open`, `closed`, or `all` (default: `open`) |
| `base` | string | No | Filter by base branch |
| `head` | string | No | Filter by head branch (user:branch format) |
| `per_page` | integer | No | Results per page (default: 30, max: 100) |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "state": "open",
  "base": "main"
}
```

### github_get_pull_request

Get a specific pull request by number.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `number` | integer | Yes | Pull request number |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "number": 45
}
```

### github_add_pr_comment

Add a comment to a pull request.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `owner` | string | Yes | Repository owner |
| `repo` | string | Yes | Repository name |
| `number` | integer | Yes | Pull request number |
| `body` | string | Yes | Comment body (Markdown supported) |

**Example:**

```json
{
  "owner": "plexusone",
  "repo": "omniagent",
  "number": 45,
  "body": "LGTM! Just one minor suggestion on line 42."
}
```

### github_search_code

Search for code across repositories.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query (GitHub code search syntax) |
| `per_page` | integer | No | Results per page (default: 30, max: 100) |

**Query syntax:**

- `handleAuth repo:plexusone/omniagent` - Search in specific repo
- `OAuth2 language:go` - Filter by language
- `func main path:cmd/` - Filter by path
- `TODO extension:go` - Filter by file extension

**Example:**

```json
{
  "query": "handleAuth repo:plexusone/omniagent language:go",
  "per_page": 10
}
```

### github_search_issues

Search for issues and pull requests.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Search query (GitHub issues search syntax) |
| `per_page` | integer | No | Results per page (default: 30, max: 100) |

**Query syntax:**

- `bug is:open` - Open issues with "bug"
- `is:pr is:merged` - Merged pull requests
- `label:bug label:high-priority` - Filter by labels
- `author:johnwang` - Filter by author
- `assignee:johnwang` - Filter by assignee

**Example:**

```json
{
  "query": "is:issue is:open label:bug repo:plexusone/omniagent",
  "per_page": 20
}
```

## Token Permissions

The GitHub skill requires a personal access token with appropriate permissions:

| Scope | Required For |
|-------|--------------|
| `repo` | Full repository access (issues, PRs, code) |
| `public_repo` | Public repository access only |

### Creating a Token

1. Go to [GitHub Settings > Developer settings > Personal access tokens](https://github.com/settings/tokens)
2. Click "Generate new token (classic)"
3. Select scopes: `repo` for private repos, or `public_repo` for public only
4. Copy the token and set as `GITHUB_TOKEN` environment variable

## Rate Limits

GitHub API has rate limits:

- **Authenticated requests**: 5,000 requests per hour
- **Unauthenticated requests**: 60 requests per hour

The skill automatically uses authentication when a token is provided, ensuring higher rate limits.

## Examples

### Development Workflow Integration

```go
// Create agent with GitHub skill
githubSkill := github.NewSkill(github.Config{})
agent, _ := agent.New(config, agent.WithCompiledSkill(githubSkill))

// Agent can now help with development tasks:
// - "Show me my assigned issues"
// - "Create a bug report for the auth issue"
// - "List open PRs that need review"
// - "Search for TODO comments in the codebase"
```

### Multi-Repository Management

```
User: Show me open bugs across all plexusone repos
Agent: [calls github_search_issues with "is:issue is:open label:bug org:plexusone"]
       Found 12 open bugs across repositories...

User: Assign the authentication bug to me
Agent: [calls github_update_issue]
       Assigned issue #42 to you.
```
