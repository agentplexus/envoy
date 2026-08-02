// Package github provides a compiled skill for GitHub operations.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grokify/gogithub"
	"github.com/grokify/gogithub/clientv1"
	"github.com/plexusone/omniskill/skill"
)

const (
	// SkillName is the name of the GitHub skill.
	SkillName = "github"
)

// Skill implements compiled.Skill for GitHub operations.
type Skill struct {
	client clientv1.Client
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

	var (
		client clientv1.Client
		err    error
	)
	switch {
	case s.config.BaseURL != "":
		// GitHub Enterprise
		client, err = clientv1.NewClientWithOptions(ctx, clientv1.ClientOptions{
			Token:     token,
			BaseURL:   s.config.BaseURL,
			UploadURL: s.config.BaseURL,
		})
	case token != "":
		client, err = clientv1.NewClient(ctx, token)
	default:
		client, err = clientv1.NewClientWithHTTP(nil)
	}
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
	opts := &clientv1.ListIssuesOptions{
		State:   input.State,
		PerPage: perPage,
	}
	if input.Labels != "" {
		opts.Labels = strings.Split(input.Labels, ",")
	}
	if opts.State == "" {
		opts.State = "open"
	}

	// ListIssues paginates through all matching issues internally; truncate
	// to the requested page size to preserve the tool's per_page contract.
	issues, err := s.client.ListIssues(ctx, input.Owner, input.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	output := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests (they appear in issues API)
		if issue.IsPullRequest {
			continue
		}
		output = append(output, issueToMap(issue))
		if len(output) >= perPage {
			break
		}
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

	issue, err := s.client.GetIssue(ctx, input.Owner, input.Repo, input.Number)
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

	issue, err := s.client.CreateIssue(ctx, input.Owner, input.Repo, &clientv1.CreateIssueInput{
		Title:     input.Title,
		Body:      input.Body,
		Labels:    input.Labels,
		Assignees: input.Assignees,
	})
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

	req := &clientv1.UpdateIssueInput{}
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
		req.Labels = input.Labels
	}

	issue, err := s.client.UpdateIssue(ctx, input.Owner, input.Repo, input.Number, req)
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

	comment, err := s.client.CreateIssueComment(ctx, input.Owner, input.Repo, input.Number, input.Body)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	return map[string]any{
		"success":    true,
		"comment_id": comment.ID,
		"url":        comment.HTMLURL,
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
	opts := &clientv1.ListPullRequestsOptions{
		State: input.State,
		Base:  input.Base,
		Head:  input.Head,
	}
	if opts.State == "" {
		opts.State = "open"
	}

	// ListPullRequests paginates through all matches internally; truncate to
	// the requested page size to preserve the tool's per_page contract.
	prs, err := s.client.ListPullRequests(ctx, input.Owner, input.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}
	if len(prs) > perPage {
		prs = prs[:perPage]
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

	pr, err := s.client.GetPullRequest(ctx, input.Owner, input.Repo, input.Number)
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
	comment, err := s.client.CreateIssueComment(ctx, input.Owner, input.Repo, input.Number, input.Body)
	if err != nil {
		return nil, fmt.Errorf("create PR comment: %w", err)
	}

	return map[string]any{
		"success":    true,
		"comment_id": comment.ID,
		"url":        comment.HTMLURL,
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
	opts := &clientv1.SearchOptions{
		PerPage: perPage,
	}

	result, err := s.client.SearchCode(ctx, input.Query, opts)
	if err != nil {
		return nil, fmt.Errorf("search code: %w", err)
	}

	output := make([]map[string]any, len(result.Items))
	for i, code := range result.Items {
		repoFullName := ""
		if code.Repository != nil {
			repoFullName = code.Repository.FullName
		}
		output[i] = map[string]any{
			"name":       code.Name,
			"path":       code.Path,
			"sha":        code.SHA,
			"url":        code.HTMLURL,
			"repository": repoFullName,
		}
	}

	return map[string]any{
		"results":     output,
		"total_count": result.Total,
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
	opts := &clientv1.SearchOptions{
		PerPage: perPage,
	}

	result, err := s.client.SearchIssues(ctx, input.Query, opts)
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	output := make([]map[string]any, len(result.Items))
	for i, issue := range result.Items {
		output[i] = issueToMap(issue)
	}

	return map[string]any{
		"results":     output,
		"total_count": result.Total,
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

func issueToMap(issue *gogithub.Issue) map[string]any {
	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.Name
	}

	assignees := make([]string, len(issue.Assignees))
	for i, a := range issue.Assignees {
		if a != nil {
			assignees[i] = a.Login
		}
	}

	user := ""
	if issue.User != nil {
		user = issue.User.Login
	}

	m := map[string]any{
		"number":     issue.Number,
		"title":      issue.Title,
		"state":      issue.State,
		"url":        issue.HTMLURL,
		"user":       user,
		"labels":     labels,
		"assignees":  assignees,
		"created_at": issue.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": issue.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		"comments":   issue.Comments,
	}

	if issue.Body != "" {
		// Truncate body for list views
		if len(issue.Body) > 500 {
			m["body"] = issue.Body[:500] + "..."
		} else {
			m["body"] = issue.Body
		}
	}

	return m
}

func prToMap(pr *gogithub.PullRequest) map[string]any {
	labels := make([]string, len(pr.Labels))
	for i, l := range pr.Labels {
		labels[i] = l.Name
	}

	assignees := make([]string, len(pr.Assignees))
	for i, a := range pr.Assignees {
		if a != nil {
			assignees[i] = a.Login
		}
	}

	user := ""
	if pr.User != nil {
		user = pr.User.Login
	}
	base := ""
	if pr.Base != nil {
		base = pr.Base.Ref
	}
	head := ""
	if pr.Head != nil {
		head = pr.Head.Ref
	}
	mergeable := false
	if pr.Mergeable != nil {
		mergeable = *pr.Mergeable
	}

	m := map[string]any{
		"number":     pr.Number,
		"title":      pr.Title,
		"state":      pr.State,
		"url":        pr.HTMLURL,
		"user":       user,
		"labels":     labels,
		"assignees":  assignees,
		"base":       base,
		"head":       head,
		"draft":      pr.Draft,
		"mergeable":  mergeable,
		"merged":     pr.Merged,
		"created_at": pr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": pr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		"additions":  pr.Additions,
		"deletions":  pr.Deletions,
		"commits":    pr.Commits,
	}

	if pr.Body != "" {
		if len(pr.Body) > 500 {
			m["body"] = pr.Body[:500] + "..."
		} else {
			m["body"] = pr.Body
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
