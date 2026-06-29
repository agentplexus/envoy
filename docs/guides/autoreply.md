# Auto-Reply

The auto-reply system allows configuring rules to automatically respond to messages without invoking the LLM. This is useful for:

- FAQ responses
- Out-of-office messages
- Command shortcuts
- Rate limiting responses

## Rule Structure

An auto-reply rule consists of conditions, a response, and optional rate limiting:

```yaml
rules:
  - id: greeting
    name: Greeting Response
    enabled: true
    priority: 1
    conditions:
      patterns:
        - "^hello"
        - "^hi"
      channels:
        - telegram
        - discord
    response:
      text: "Hello! How can I help you today?"
      stop_processing: true
    rate_limit:
      max_count: 5
      window: 1m
      per_sender: true
```

## Conditions

Rules match based on multiple conditions. All specified conditions must pass for the rule to trigger.

### Patterns

Regex patterns to match against message content. Any pattern match triggers the rule.

```yaml
conditions:
  patterns:
    - "^hello"        # Starts with "hello"
    - "help\\s+me"    # Contains "help me"
    - "\\bprice\\b"   # Contains word "price"
```

Patterns are case-insensitive by default.

### Keywords

Simple keyword matching (case-insensitive substring match):

```yaml
conditions:
  keywords:
    - "help"
    - "support"
    - "question"
```

### Channels

Limit the rule to specific channel types:

```yaml
conditions:
  channels:
    - telegram
    - discord
    - whatsapp
```

### Senders

Limit the rule to specific sender IDs:

```yaml
conditions:
  senders:
    - "user123"
    - "admin456"
```

### Time Range

Restrict the rule to specific times:

```yaml
conditions:
  time_range:
    start: "09:00"      # 24-hour format
    end: "17:00"
    days: [1, 2, 3, 4, 5]  # Monday-Friday (0=Sunday)
    timezone: "America/New_York"
```

## Response Options

### Text Response

Simple static text response:

```yaml
response:
  text: "Thank you for your message!"
```

### Stop Processing

Prevent further rule evaluation after this rule matches:

```yaml
response:
  text: "This is handled."
  stop_processing: true
```

### Pass Through

Allow the message to continue to the LLM even after auto-reply:

```yaml
response:
  text: "I'll look into this..."
  pass_through: true
```

## Rate Limiting

Prevent rules from triggering too frequently:

```yaml
rate_limit:
  max_count: 3        # Maximum triggers
  window: 1m          # Time window (s, m, h)
  per_sender: false   # Global or per-sender limit
```

### Global Rate Limit

Limit total triggers across all users:

```yaml
rate_limit:
  max_count: 10
  window: 1h
  per_sender: false
```

### Per-Sender Rate Limit

Limit triggers per individual user:

```yaml
rate_limit:
  max_count: 2
  window: 5m
  per_sender: true
```

## Priority

Rules are evaluated in priority order (lower number = higher priority):

```yaml
rules:
  - id: vip-support
    priority: 1         # Evaluated first
    conditions:
      senders: ["vip-user"]
    response:
      text: "VIP support activated!"

  - id: general-help
    priority: 10        # Evaluated after
    conditions:
      keywords: ["help"]
    response:
      text: "How can I help?"
```

## Example Configurations

### Out-of-Office

```yaml
rules:
  - id: out-of-office
    name: Out of Office
    enabled: true
    priority: 1
    conditions:
      time_range:
        start: "18:00"
        end: "09:00"
        timezone: "UTC"
    response:
      text: |
        Thanks for your message! I'm currently away.
        I'll respond during business hours (9 AM - 6 PM UTC).
      stop_processing: true
```

### FAQ Responses

```yaml
rules:
  - id: faq-pricing
    name: Pricing FAQ
    enabled: true
    priority: 5
    conditions:
      keywords: ["price", "cost", "pricing", "how much"]
    response:
      text: "Our pricing information is available at https://example.com/pricing"
      stop_processing: true

  - id: faq-hours
    name: Hours FAQ
    enabled: true
    priority: 5
    conditions:
      patterns: ["(business|office|opening)\\s+hours"]
    response:
      text: "We're open Monday-Friday, 9 AM - 6 PM EST."
      stop_processing: true
```

### Spam Protection

```yaml
rules:
  - id: rate-limit-messages
    name: Message Rate Limit
    enabled: true
    priority: 0
    conditions:
      patterns: [".*"]  # Match everything
    response:
      text: "Please slow down. You're sending messages too quickly."
      stop_processing: true
    rate_limit:
      max_count: 10
      window: 1m
      per_sender: true
```

## Programmatic Usage

```go
import "github.com/plexusone/omniagent/autoreply"

// Create handler
handler := autoreply.NewHandler()

// Add rule
rule := &autoreply.Rule{
    ID:      "greeting",
    Enabled: true,
    Conditions: autoreply.Conditions{
        Patterns: []string{"^hello", "^hi"},
    },
    Response: autoreply.Response{
        Text:           "Hello! How can I help?",
        StopProcessing: true,
    },
}

if err := handler.AddRule(rule); err != nil {
    log.Fatal(err)
}

// Evaluate message
msg := &autoreply.Message{
    Content:   "hello there",
    Channel:   "telegram",
    SenderID:  "user123",
    Timestamp: time.Now(),
}

result := handler.Evaluate(context.Background(), msg)

if result.Matched {
    fmt.Printf("Rule %s matched: %s\n", result.RuleID, result.Response)
}
```

## Integration

The auto-reply handler can be integrated into the message processing pipeline:

```go
func handleMessage(ctx context.Context, msg *Message) {
    // Check auto-reply first
    result := autoReplyHandler.Evaluate(ctx, &autoreply.Message{
        Content:   msg.Text,
        Channel:   msg.Channel,
        SenderID:  msg.SenderID,
        Timestamp: msg.Timestamp,
    })

    if result.Matched {
        sendReply(msg, result.Response)

        if result.StopProcessing {
            return // Don't continue to LLM
        }
    }

    // Continue to LLM processing
    llmResponse := processWithLLM(ctx, msg)
    sendReply(msg, llmResponse)
}
```
