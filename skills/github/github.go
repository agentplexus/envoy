// Package github provides a compiled skill for GitHub operations.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v88/github"
	"github.com/plexusone/omniskill/skill"
)

const (
	// SkillName is the name of the GitHub skill.
	SkillName = "github"
)

// Skill implements compiled.Skill for GitHub operations.
type Skill struct {
	client *github.Client
	config Config
}

// Config configures the GitHub skill.
type Config struct {
	// Token is the GitHub personal access token.
	// If empty, uses GITHUB_TOKEN or GH_TOKEN environment variable.
	Token string

	// BaseURL is the GitHub API base URL.
	// If empty, uses the public GitHub API.
	BaseURL string
}

// NewSkill creates a new GitHub skill.
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
	return "GitHub operations: issues, pull requests, and code search"
}

// Init implements compiled.Skill.
func (s *Skill) Init(ctx context.Context) error {
	token := s.config.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	var opts []github.ClientOptionsFunc
	if token != "" {
		opts = append(opts, github.WithAuthToken(token))
	}

	if s.config.BaseURL != "" {
		// GitHub Enterprise
		opts = append(opts, github.WithEnterpriseURLs(s.config.BaseURL, s.config.BaseURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("create github client: %w", err)
	}
	s.client = client

	return nil
}

// Close implements compiled.Skill.
func (s *Skill) Close() error {
	return nil
}

// Tools implements compiled.Skill.
func (s *Skill) Tools() []skill.Tool {
	return []skill.Tool{
		// Issues
		&githubTool{
			name:        "github_list_issues",
			description: "List issues in a GitHub repository",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner (user or organization)",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"state": {
					Type:        "string",
					Description: "Issue state: open, closed, or all (default: open)",
					Enum:        []any{"open", "closed", "all"},
				},
				"labels": {
					Type:        "string",
					Description: "Comma-separated list of label names",
				},
				"per_page": {
					Type:        "integer",
					Description: "Results per page (default: 30, max: 100)",
				},
			},
			handler: s.handleListIssues,
		},
		&githubTool{
			name:        "github_get_issue",
			description: "Get a specific issue by number",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"number": {
					Type:        "integer",
					Description: "Issue number",
					Required:    true,
				},
			},
			handler: s.handleGetIssue,
		},
		&githubTool{
			name:        "github_create_issue",
			description: "Create a new issue",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"title": {
					Type:        "string",
					Description: "Issue title",
					Required:    true,
				},
				"body": {
					Type:        "string",
					Description: "Issue body (Markdown supported)",
				},
				"labels": {
					Type: "array",
					Items: &skill.Parameter{
						Type: "string",
					},
					Description: "Labels to apply",
				},
				"assignees": {
					Type: "array",
					Items: &skill.Parameter{
						Type: "string",
					},
					Description: "Users to assign",
				},
			},
			handler: s.handleCreateIssue,
		},
		&githubTool{
			name:        "github_update_issue",
			description: "Update an existing issue",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"number": {
					Type:        "integer",
					Description: "Issue number",
					Required:    true,
				},
				"title": {
					Type:        "string",
					Description: "New title (optional)",
				},
				"body": {
					Type:        "string",
					Description: "New body (optional)",
				},
				"state": {
					Type:        "string",
					Description: "New state: open or closed",
					Enum:        []any{"open", "closed"},
				},
				"labels": {
					Type: "array",
					Items: &skill.Parameter{
						Type: "string",
					},
					Description: "Labels to set (replaces existing)",
				},
			},
			handler: s.handleUpdateIssue,
		},
		&githubTool{
			name:        "github_add_issue_comment",
			description: "Add a comment to an issue",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"number": {
					Type:        "integer",
					Description: "Issue number",
					Required:    true,
				},
				"body": {
					Type:        "string",
					Description: "Comment body (Markdown supported)",
					Required:    true,
				},
			},
			handler: s.handleAddIssueComment,
		},
		// Pull Requests
		&githubTool{
			name:        "github_list_pull_requests",
			description: "List pull requests in a repository",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"state": {
					Type:        "string",
					Description: "PR state: open, closed, or all (default: open)",
					Enum:        []any{"open", "closed", "all"},
				},
				"base": {
					Type:        "string",
					Description: "Filter by base branch",
				},
				"head": {
					Type:        "string",
					Description: "Filter by head branch (user:branch format)",
				},
				"per_page": {
					Type:        "integer",
					Description: "Results per page (default: 30, max: 100)",
				},
			},
			handler: s.handleListPullRequests,
		},
		&githubTool{
			name:        "github_get_pull_request",
			description: "Get a specific pull request by number",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"number": {
					Type:        "integer",
					Description: "Pull request number",
					Required:    true,
				},
			},
			handler: s.handleGetPullRequest,
		},
		&githubTool{
			name:        "github_add_pr_comment",
			description: "Add a review comment to a pull request",
			params: map[string]skill.Parameter{
				"owner": {
					Type:        "string",
					Description: "Repository owner",
					Required:    true,
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
					Required:    true,
				},
				"number": {
					Type:        "integer",
					Description: "Pull request number",
					Required:    true,
				},
				"body": {
					Type:        "string",
					Description: "Comment body (Markdown supported)",
					Required:    true,
				},
			},
			handler: s.handleAddPRComment,
		},
		// Search
		&githubTool{
			name:        "github_search_code",
			description: "Search for code across repositories",
			params: map[string]skill.Parameter{
				"query": {
					Type:        "string",
					Description: "Search query (GitHub code search syntax)",
					Required:    true,
				},
				"per_page": {
					Type:        "integer",
					Description: "Results per page (default: 30, max: 100)",
				},
			},
			handler: s.handleSearchCode,
		},
		&githubTool{
			name:        "github_search_issues",
			description: "Search for issues and pull requests",
			params: map[string]skill.Parameter{
				"query": {
					Type:        "string",
					Description: "Search query (GitHub issues search syntax)",
					Required:    true,
				},
				"per_page": {
					Type:        "integer",
					Description: "Results per page (default: 30, max: 100)",
				},
			},
			handler: s.handleSearchIssues,
		},
	}
}

// Handlers

type listIssuesInput struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	State   string `json:"state"`
	Labels  string `json:"labels"`
	PerPage int    `json:"per_page"`
}

func (s *Skill) handleListIssues(ctx context.Context, params map[string]any) (any, error) {
	var input listIssuesInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	perPage := input.PerPage
	if perPage == 0 {
		perPage = 30
	}
	opts := &github.IssueListByRepoOptions{
		State: input.State,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}
	if input.Labels != "" {
		opts.Labels = strings.Split(input.Labels, ",")
	}
	if opts.State == "" {
		opts.State = "open"
	}

	issues, _, err := s.client.Issues.ListByRepo(ctx, input.Owner, input.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	output := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests (they appear in issues API)
		if issue.PullRequestLinks != nil {
			continue
		}
		output = append(output, issueToMap(issue))
	}

	return map[string]any{
		"issues": output,
		"count":  len(output),
	}, nil
}

type getIssueInput struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

func (s *Skill) handleGetIssue(ctx context.Context, params map[string]any) (any, error) {
	var input getIssueInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	issue, _, err := s.client.Issues.Get(ctx, input.Owner, input.Repo, input.Number)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	return issueToMap(issue), nil
}

type createIssueInput struct {
	Owner     string   `json:"owner"`
	Repo      string   `json:"repo"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

func (s *Skill) handleCreateIssue(ctx context.Context, params map[string]any) (any, error) {
	var input createIssueInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	req := &github.IssueRequest{
		Title: &input.Title,
	}
	if input.Body != "" {
		req.Body = &input.Body
	}
	if len(input.Labels) > 0 {
		req.Labels = &input.Labels
	}
	if len(input.Assignees) > 0 {
		req.Assignees = &input.Assignees
	}

	issue, _, err := s.client.Issues.Create(ctx, input.Owner, input.Repo, req)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	return map[string]any{
		"success": true,
		"issue":   issueToMap(issue),
	}, nil
}

type updateIssueInput struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	State  string   `json:"state"`
	Labels []string `json:"labels"`
}

func (s *Skill) handleUpdateIssue(ctx context.Context, params map[string]any) (any, error) {
	var input updateIssueInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	req := &github.IssueRequest{}
	if input.Title != "" {
		req.Title = &input.Title
	}
	if input.Body != "" {
		req.Body = &input.Body
	}
	if input.State != "" {
		req.State = &input.State
	}
	if len(input.Labels) > 0 {
		req.Labels = &input.Labels
	}

	issue, _, err := s.client.Issues.Edit(ctx, input.Owner, input.Repo, input.Number, req)
	if err != nil {
		return nil, fmt.Errorf("update issue: %w", err)
	}

	return map[string]any{
		"success": true,
		"issue":   issueToMap(issue),
	}, nil
}

type addIssueCommentInput struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Body   string `json:"body"`
}

func (s *Skill) handleAddIssueComment(ctx context.Context, params map[string]any) (any, error) {
	var input addIssueCommentInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	comment, _, err := s.client.Issues.CreateComment(ctx, input.Owner, input.Repo, input.Number, &github.IssueComment{
		Body: &input.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	return map[string]any{
		"success":    true,
		"comment_id": comment.GetID(),
		"url":        comment.GetHTMLURL(),
	}, nil
}

type listPullRequestsInput struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	State   string `json:"state"`
	Base    string `json:"base"`
	Head    string `json:"head"`
	PerPage int    `json:"per_page"`
}

func (s *Skill) handleListPullRequests(ctx context.Context, params map[string]any) (any, error) {
	var input listPullRequestsInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	perPage := input.PerPage
	if perPage == 0 {
		perPage = 30
	}
	opts := &github.PullRequestListOptions{
		State: input.State,
		Base:  input.Base,
		Head:  input.Head,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}
	if opts.State == "" {
		opts.State = "open"
	}

	prs, _, err := s.client.PullRequests.List(ctx, input.Owner, input.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}

	output := make([]map[string]any, len(prs))
	for i, pr := range prs {
		output[i] = prToMap(pr)
	}

	return map[string]any{
		"pull_requests": output,
		"count":         len(output),
	}, nil
}

type getPullRequestInput struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

func (s *Skill) handleGetPullRequest(ctx context.Context, params map[string]any) (any, error) {
	var input getPullRequestInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	pr, _, err := s.client.PullRequests.Get(ctx, input.Owner, input.Repo, input.Number)
	if err != nil {
		return nil, fmt.Errorf("get pull request: %w", err)
	}

	return prToMap(pr), nil
}

type addPRCommentInput struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Body   string `json:"body"`
}

func (s *Skill) handleAddPRComment(ctx context.Context, params map[string]any) (any, error) {
	var input addPRCommentInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	// Use issues API for general comments (not review comments)
	comment, _, err := s.client.Issues.CreateComment(ctx, input.Owner, input.Repo, input.Number, &github.IssueComment{
		Body: &input.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("create PR comment: %w", err)
	}

	return map[string]any{
		"success":    true,
		"comment_id": comment.GetID(),
		"url":        comment.GetHTMLURL(),
	}, nil
}

type searchCodeInput struct {
	Query   string `json:"query"`
	PerPage int    `json:"per_page"`
}

func (s *Skill) handleSearchCode(ctx context.Context, params map[string]any) (any, error) {
	var input searchCodeInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	perPage := input.PerPage
	if perPage == 0 {
		perPage = 30
	}
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	result, _, err := s.client.Search.Code(ctx, input.Query, opts)
	if err != nil {
		return nil, fmt.Errorf("search code: %w", err)
	}

	output := make([]map[string]any, len(result.CodeResults))
	for i, code := range result.CodeResults {
		output[i] = map[string]any{
			"name":       code.GetName(),
			"path":       code.GetPath(),
			"sha":        code.GetSHA(),
			"url":        code.GetHTMLURL(),
			"repository": code.GetRepository().GetFullName(),
		}
	}

	return map[string]any{
		"results":     output,
		"total_count": result.GetTotal(),
	}, nil
}

type searchIssuesInput struct {
	Query   string `json:"query"`
	PerPage int    `json:"per_page"`
}

func (s *Skill) handleSearchIssues(ctx context.Context, params map[string]any) (any, error) {
	var input searchIssuesInput
	if err := mapToStruct(params, &input); err != nil {
		return nil, err
	}

	perPage := input.PerPage
	if perPage == 0 {
		perPage = 30
	}
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	result, _, err := s.client.Search.Issues(ctx, input.Query, opts)
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	output := make([]map[string]any, len(result.Issues))
	for i, issue := range result.Issues {
		output[i] = issueToMap(issue)
	}

	return map[string]any{
		"results":     output,
		"total_count": result.GetTotal(),
	}, nil
}

// Helpers

func mapToStruct(m map[string]any, v any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func issueToMap(issue *github.Issue) map[string]any {
	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.GetName()
	}

	assignees := make([]string, len(issue.Assignees))
	for i, a := range issue.Assignees {
		assignees[i] = a.GetLogin()
	}

	m := map[string]any{
		"number":     issue.GetNumber(),
		"title":      issue.GetTitle(),
		"state":      issue.GetState(),
		"url":        issue.GetHTMLURL(),
		"user":       issue.GetUser().GetLogin(),
		"labels":     labels,
		"assignees":  assignees,
		"created_at": issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at": issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		"comments":   issue.GetComments(),
	}

	if body := issue.GetBody(); body != "" {
		// Truncate body for list views
		if len(body) > 500 {
			m["body"] = body[:500] + "..."
		} else {
			m["body"] = body
		}
	}

	return m
}

func prToMap(pr *github.PullRequest) map[string]any {
	labels := make([]string, len(pr.Labels))
	for i, l := range pr.Labels {
		labels[i] = l.GetName()
	}

	assignees := make([]string, len(pr.Assignees))
	for i, a := range pr.Assignees {
		assignees[i] = a.GetLogin()
	}

	m := map[string]any{
		"number":     pr.GetNumber(),
		"title":      pr.GetTitle(),
		"state":      pr.GetState(),
		"url":        pr.GetHTMLURL(),
		"user":       pr.GetUser().GetLogin(),
		"labels":     labels,
		"assignees":  assignees,
		"base":       pr.GetBase().GetRef(),
		"head":       pr.GetHead().GetRef(),
		"draft":      pr.GetDraft(),
		"mergeable":  pr.GetMergeable(),
		"merged":     pr.GetMerged(),
		"created_at": pr.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		"updated_at": pr.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		"additions":  pr.GetAdditions(),
		"deletions":  pr.GetDeletions(),
		"commits":    pr.GetCommits(),
	}

	if body := pr.GetBody(); body != "" {
		if len(body) > 500 {
			m["body"] = body[:500] + "..."
		} else {
			m["body"] = body
		}
	}

	return m
}

// githubTool implements skill.Tool.
type githubTool struct {
	name        string
	description string
	params      map[string]skill.Parameter
	handler     func(ctx context.Context, params map[string]any) (any, error)
}

func (t *githubTool) Name() string {
	return t.name
}

func (t *githubTool) Description() string {
	return t.description
}

func (t *githubTool) Parameters() map[string]skill.Parameter {
	return t.params
}

func (t *githubTool) Call(ctx context.Context, params map[string]any) (any, error) {
	return t.handler(ctx, params)
}
