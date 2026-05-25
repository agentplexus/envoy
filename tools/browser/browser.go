// Package browser provides browser automation tools for omniagent.
package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/plexusone/omniagent/agent"
)

// Tool provides browser automation capabilities.
type Tool struct {
	browser  *rod.Browser
	page     *rod.Page
	headless bool
	logger   *slog.Logger

	// Dialog tracking
	dialogs        []Dialog
	dialogCallback func(Dialog)
}

// Config configures the browser tool.
type Config struct {
	Headless bool
	UserData string
	Logger   *slog.Logger

	// EvaluateTimeout is the default timeout for JavaScript evaluation.
	// Zero means use the action timeout.
	EvaluateTimeout time.Duration

	// DialogCallback is called when a dialog is observed.
	DialogCallback func(Dialog)
}

// Dialog represents a browser dialog (alert, confirm, prompt).
type Dialog struct {
	// Type is the dialog type: "alert", "confirm", "prompt", "beforeunload".
	Type string

	// Message is the dialog message text.
	Message string

	// DefaultValue is the default value for prompt dialogs.
	DefaultValue string

	// URL is the page URL where the dialog appeared.
	URL string

	// Timestamp is when the dialog was observed.
	Timestamp time.Time

	// Handled indicates if the dialog was automatically handled.
	Handled bool

	// Response is the response given (for confirm: "true"/"false", for prompt: input text).
	Response string
}

// New creates a new browser tool.
func New(config Config) (*Tool, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &Tool{
		headless:       config.Headless,
		logger:         config.Logger,
		dialogCallback: config.DialogCallback,
	}, nil
}

// Name returns the tool name.
func (t *Tool) Name() string {
	return "browser"
}

// Description returns the tool description.
func (t *Tool) Description() string {
	return "Control a web browser to navigate pages, click elements, fill forms, and take screenshots."
}

// Parameters returns the JSON schema for tool parameters.
func (t *Tool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "The browser action to perform",
				"enum":        []string{"navigate", "click", "type", "screenshot", "get_text", "wait", "evaluate", "get_dialogs", "dismiss_dialog"},
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to navigate to (for navigate action)",
			},
			"selector": map[string]interface{}{
				"type":        "string",
				"description": "CSS selector for the element (for click, type, get_text actions)",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to type (for type action)",
			},
			"script": map[string]interface{}{
				"type":        "string",
				"description": "JavaScript code to evaluate (for evaluate action)",
			},
			"timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
		},
		"required": []string{"action"},
	}
}

// Execute runs the browser tool.
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action   string `json:"action"`
		URL      string `json:"url"`
		Selector string `json:"selector"`
		Text     string `json:"text"`
		Script   string `json:"script"`
		Timeout  int    `json:"timeout"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse parameters: %w", err)
	}

	if params.Timeout == 0 {
		params.Timeout = 30
	}

	// Ensure browser is launched
	if err := t.ensureBrowser(); err != nil {
		return "", err
	}

	timeout := time.Duration(params.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch params.Action {
	case "navigate":
		return t.navigate(ctx, params.URL)
	case "click":
		return t.click(ctx, params.Selector)
	case "type":
		return t.typeText(ctx, params.Selector, params.Text)
	case "screenshot":
		return t.screenshot(ctx)
	case "get_text":
		return t.getText(ctx, params.Selector)
	case "wait":
		return t.wait(ctx, params.Selector)
	case "evaluate":
		return t.evaluate(ctx, params.Script)
	case "get_dialogs":
		return t.getDialogs()
	case "dismiss_dialog":
		return t.dismissDialog(ctx)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// ensureBrowser ensures the browser is launched.
func (t *Tool) ensureBrowser() error {
	if t.browser != nil {
		return nil
	}

	l := launcher.New().Headless(t.headless)
	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}

	t.browser = rod.New().ControlURL(url)
	if err := t.browser.Connect(); err != nil {
		return fmt.Errorf("connect browser: %w", err)
	}

	t.page, err = t.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	// Set up dialog handling
	t.setupDialogHandler()

	t.logger.Info("browser launched", "headless", t.headless)
	return nil
}

// setupDialogHandler sets up JavaScript dialog event handling.
func (t *Tool) setupDialogHandler() {
	go t.page.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
		dialog := Dialog{
			Type:         string(e.Type),
			Message:      e.Message,
			DefaultValue: e.DefaultPrompt,
			URL:          e.URL,
			Timestamp:    time.Now(),
		}

		// Store the dialog
		t.dialogs = append(t.dialogs, dialog)

		// Call callback if set
		if t.dialogCallback != nil {
			t.dialogCallback(dialog)
		}

		t.logger.Info("dialog observed",
			"type", e.Type,
			"message", e.Message,
			"url", e.URL)
	})()
}

// navigate navigates to a URL.
func (t *Tool) navigate(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url required for navigate action")
	}

	if err := t.page.Context(ctx).Navigate(url); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}

	if err := t.page.WaitStable(time.Second); err != nil {
		return "", fmt.Errorf("wait stable: %w", err)
	}

	title := t.page.MustInfo().Title

	return fmt.Sprintf("Navigated to: %s (title: %s)", url, title), nil
}

// click clicks an element.
func (t *Tool) click(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for click action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("click: %w", err)
	}

	return fmt.Sprintf("Clicked element: %s", selector), nil
}

// typeText types text into an element.
func (t *Tool) typeText(ctx context.Context, selector, text string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for type action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	if err := el.Input(text); err != nil {
		return "", fmt.Errorf("type: %w", err)
	}

	return fmt.Sprintf("Typed text into: %s", selector), nil
}

// screenshot takes a screenshot.
func (t *Tool) screenshot(ctx context.Context) (string, error) {
	data, err := t.page.Context(ctx).Screenshot(false, nil)
	if err != nil {
		return "", fmt.Errorf("screenshot: %w", err)
	}

	// In a real implementation, you might save this or return the base64 data
	_ = base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("Screenshot taken (%d bytes)", len(data)), nil
}

// getText gets text from an element.
func (t *Tool) getText(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for get_text action")
	}

	el, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("find element: %w", err)
	}

	text, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("get text: %w", err)
	}

	return text, nil
}

// wait waits for an element to appear.
func (t *Tool) wait(ctx context.Context, selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector required for wait action")
	}

	_, err := t.page.Context(ctx).Element(selector)
	if err != nil {
		return "", fmt.Errorf("wait for element: %w", err)
	}

	return fmt.Sprintf("Element found: %s", selector), nil
}

// evaluate executes JavaScript in the page context.
func (t *Tool) evaluate(ctx context.Context, script string) (string, error) {
	if script == "" {
		return "", fmt.Errorf("script required for evaluate action")
	}

	result, err := t.page.Context(ctx).Eval(script)
	if err != nil {
		return "", fmt.Errorf("evaluate script: %w", err)
	}

	// Convert result to string
	if result.Value.Nil() {
		return "undefined", nil
	}

	return result.Value.String(), nil
}

// getDialogs returns all observed dialogs.
func (t *Tool) getDialogs() (string, error) {
	if len(t.dialogs) == 0 {
		return "No dialogs observed", nil
	}

	data, err := json.Marshal(t.dialogs)
	if err != nil {
		return "", fmt.Errorf("marshal dialogs: %w", err)
	}

	return string(data), nil
}

// dismissDialog dismisses the current dialog.
func (t *Tool) dismissDialog(ctx context.Context) (string, error) {
	// Accept the dialog (dismiss with default action)
	err := proto.PageHandleJavaScriptDialog{
		Accept: true,
	}.Call(t.page)

	if err != nil {
		return "", fmt.Errorf("dismiss dialog: %w", err)
	}

	// Mark last dialog as handled
	if len(t.dialogs) > 0 {
		t.dialogs[len(t.dialogs)-1].Handled = true
		t.dialogs[len(t.dialogs)-1].Response = "accepted"
	}

	return "Dialog dismissed", nil
}

// AcceptDialog accepts a dialog with an optional response.
func (t *Tool) AcceptDialog(ctx context.Context, response string) error {
	err := proto.PageHandleJavaScriptDialog{
		Accept:     true,
		PromptText: response,
	}.Call(t.page)

	if err != nil {
		return fmt.Errorf("accept dialog: %w", err)
	}

	if len(t.dialogs) > 0 {
		t.dialogs[len(t.dialogs)-1].Handled = true
		t.dialogs[len(t.dialogs)-1].Response = response
	}

	return nil
}

// DismissDialogWithCancel dismisses a dialog by clicking cancel.
func (t *Tool) DismissDialogWithCancel(ctx context.Context) error {
	err := proto.PageHandleJavaScriptDialog{
		Accept: false,
	}.Call(t.page)

	if err != nil {
		return fmt.Errorf("cancel dialog: %w", err)
	}

	if len(t.dialogs) > 0 {
		t.dialogs[len(t.dialogs)-1].Handled = true
		t.dialogs[len(t.dialogs)-1].Response = "cancelled"
	}

	return nil
}

// GetObservedDialogs returns the list of observed dialogs.
func (t *Tool) GetObservedDialogs() []Dialog {
	return t.dialogs
}

// ClearDialogs clears the dialog history.
func (t *Tool) ClearDialogs() {
	t.dialogs = nil
}

// SetDialogCallback sets a callback for dialog events.
func (t *Tool) SetDialogCallback(callback func(Dialog)) {
	t.dialogCallback = callback
}

// Close closes the browser.
func (t *Tool) Close() error {
	if t.browser != nil {
		return t.browser.Close()
	}
	return nil
}

// Ensure Tool implements agent.Tool interface.
var _ agent.Tool = (*Tool)(nil)
