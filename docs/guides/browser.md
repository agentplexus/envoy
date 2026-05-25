# Browser Automation

OmniAgent includes a built-in browser tool powered by [Rod](https://github.com/go-rod/rod) for web automation and scraping.

## Overview

The browser tool enables agents to:

- Navigate to URLs and capture screenshots
- Click elements and fill forms
- Execute JavaScript code
- Handle browser dialogs (alerts, confirms, prompts)
- Extract text content from pages

## Basic Usage

```go
import "github.com/plexusone/omniagent/tools/browser"

// Create the browser tool
tool, err := browser.New(browser.Config{
    Headless: true,
    Logger:   logger,
})
if err != nil {
    return err
}
defer tool.Close()

// The agent can now use browser actions
```

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Headless` | bool | `true` | Run browser without UI |
| `UserData` | string | - | Chrome user data directory |
| `Logger` | *slog.Logger | - | Logger instance |
| `EvaluateTimeout` | duration | - | JavaScript evaluation timeout |
| `DialogCallback` | func(Dialog) | - | Callback for observed dialogs |

```go
tool, err := browser.New(browser.Config{
    Headless:        true,
    UserData:        "/tmp/chrome-data",
    EvaluateTimeout: 30 * time.Second,
    DialogCallback: func(d browser.Dialog) {
        log.Printf("Dialog observed: %s - %s", d.Type, d.Message)
    },
})
```

## Actions

The browser tool supports these actions:

| Action | Description |
|--------|-------------|
| `navigate` | Go to a URL |
| `click` | Click an element |
| `type` | Enter text in an input |
| `screenshot` | Capture page screenshot |
| `get_text` | Extract text from element |
| `wait` | Wait for element or timeout |
| `evaluate` | Execute JavaScript |
| `get_dialogs` | Get observed dialog history |
| `dismiss_dialog` | Dismiss an active dialog |

### Navigate

```json
{
  "action": "navigate",
  "url": "https://example.com"
}
```

### Click

```json
{
  "action": "click",
  "selector": "button#submit"
}
```

### Type

```json
{
  "action": "type",
  "selector": "input[name='email']",
  "text": "user@example.com"
}
```

### Screenshot

```json
{
  "action": "screenshot"
}
```

Returns the screenshot as a base64-encoded image.

### Get Text

```json
{
  "action": "get_text",
  "selector": "h1.title"
}
```

### Wait

```json
{
  "action": "wait",
  "selector": ".loading-complete",
  "timeout": 10000
}
```

## JavaScript Evaluation

Execute arbitrary JavaScript in the page context:

```json
{
  "action": "evaluate",
  "script": "return document.title"
}
```

### Complex Scripts

```json
{
  "action": "evaluate",
  "script": "return Array.from(document.querySelectorAll('a')).map(a => a.href)"
}
```

### Async Scripts

```json
{
  "action": "evaluate",
  "script": "return await fetch('/api/data').then(r => r.json())"
}
```

## Dialog Handling

The browser tool automatically tracks JavaScript dialogs (alert, confirm, prompt).

### Dialog Types

| Type | Description |
|------|-------------|
| `alert` | Information message, OK button only |
| `confirm` | Yes/No choice |
| `prompt` | Text input |
| `beforeunload` | Page leave confirmation |

### Dialog Structure

```go
type Dialog struct {
    Type         string    // "alert", "confirm", "prompt", "beforeunload"
    Message      string    // Dialog message text
    DefaultValue string    // Default value for prompt dialogs
    URL          string    // Page URL where dialog appeared
    Timestamp    time.Time // When observed
    Handled      bool      // If automatically handled
    Response     string    // Response given
}
```

### Get Dialog History

```json
{
  "action": "get_dialogs"
}
```

Returns all observed dialogs:

```json
[
  {
    "type": "alert",
    "message": "Form submitted successfully!",
    "url": "https://example.com/form",
    "timestamp": "2025-01-15T10:30:00Z",
    "handled": true
  },
  {
    "type": "confirm",
    "message": "Are you sure you want to delete?",
    "url": "https://example.com/settings",
    "timestamp": "2025-01-15T10:31:00Z",
    "handled": true,
    "response": "true"
  }
]
```

### Dismiss Active Dialog

```json
{
  "action": "dismiss_dialog",
  "response": "false"
}
```

For confirm dialogs, use `"true"` or `"false"`. For prompt dialogs, provide the text to enter.

### Dialog Callback

Handle dialogs programmatically:

```go
tool, err := browser.New(browser.Config{
    Headless: true,
    DialogCallback: func(d browser.Dialog) {
        switch d.Type {
        case "alert":
            log.Printf("Alert: %s", d.Message)
        case "confirm":
            log.Printf("Confirm: %s (answered: %s)", d.Message, d.Response)
        case "prompt":
            log.Printf("Prompt: %s (input: %s)", d.Message, d.Response)
        }
    },
})
```

## Selectors

The browser tool supports CSS selectors and XPath:

### CSS Selectors

```json
{"selector": "#submit-button"}
{"selector": ".form-input"}
{"selector": "button[type='submit']"}
{"selector": "div.container > p:first-child"}
```

### XPath

```json
{"selector": "//button[contains(text(), 'Submit')]"}
{"selector": "//div[@class='results']//a"}
```

## Error Handling

The browser tool returns errors for common failure cases:

| Error | Cause |
|-------|-------|
| `element not found` | Selector didn't match any element |
| `timeout waiting for element` | Element didn't appear within timeout |
| `navigation failed` | URL couldn't be loaded |
| `script evaluation failed` | JavaScript error |

## Best Practices

### Wait for Elements

Always wait for elements before interacting:

```json
[
  {"action": "navigate", "url": "https://example.com"},
  {"action": "wait", "selector": ".page-loaded"},
  {"action": "click", "selector": "button#action"}
]
```

### Handle Dynamic Content

Use JavaScript evaluation for dynamic pages:

```json
{
  "action": "evaluate",
  "script": "return new Promise(resolve => { const check = () => { const el = document.querySelector('.data-loaded'); if (el) resolve(el.textContent); else setTimeout(check, 100); }; check(); })"
}
```

### Clean Up

Always close the browser when done:

```go
tool, err := browser.New(config)
if err != nil {
    return err
}
defer tool.Close()
```

### Headless Mode

Use headless mode in production for better performance and resource usage:

```go
tool, err := browser.New(browser.Config{
    Headless: true,
})
```

Disable headless for debugging:

```go
tool, err := browser.New(browser.Config{
    Headless: false,  // Shows browser window
})
```
