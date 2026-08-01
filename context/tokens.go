package context

import (
	"strings"

	"github.com/pkoukk/tiktoken-go"
	"github.com/plexusone/omnillm/provider"
)

// TokenCounter estimates the token count for messages.
type TokenCounter interface {
	// Count returns the estimated token count for a message.
	Count(msg provider.Message) int

	// CountText returns the estimated token count for text.
	CountText(text string) int
}

// SimpleTokenCounter provides a rough token estimate based on character count.
// It uses a simple heuristic: ~4 characters per token for English text.
// This is suitable for rough estimates but not precise billing calculations.
type SimpleTokenCounter struct {
	// CharsPerToken is the average characters per token.
	// Default is 4 for English text.
	CharsPerToken int
}

// Count estimates tokens for a message.
func (c *SimpleTokenCounter) Count(msg provider.Message) int {
	// Count content
	tokens := c.CountText(msg.Content)

	// Add overhead for role and structure (~4 tokens per message)
	tokens += 4

	// Add tokens for tool calls if present
	for _, tc := range msg.ToolCalls {
		tokens += c.CountText(tc.Function.Name)
		tokens += c.CountText(tc.Function.Arguments)
		tokens += 4 // Overhead for tool call structure
	}

	return tokens
}

// CountText estimates tokens for text.
func (c *SimpleTokenCounter) CountText(text string) int {
	charsPerToken := c.CharsPerToken
	if charsPerToken <= 0 {
		charsPerToken = 4
	}

	if len(text) == 0 {
		return 0
	}

	return (len(text) + charsPerToken - 1) / charsPerToken
}

// ModelTokenCounter uses model-specific token counting.
// This provides more accurate counts for specific models.
type ModelTokenCounter struct {
	// Model is the model name for token counting.
	Model string

	// Fallback is used if model-specific counting is unavailable.
	Fallback TokenCounter

	// encoding is the cached tiktoken encoding for OpenAI models.
	encoding *tiktoken.Tiktoken
}

// NewModelTokenCounter creates a new model-specific token counter.
func NewModelTokenCounter(model string) *ModelTokenCounter {
	c := &ModelTokenCounter{
		Model:    model,
		Fallback: &SimpleTokenCounter{},
	}

	// Try to get tiktoken encoding for OpenAI models
	if isOpenAIModel(model) {
		enc, err := tiktoken.EncodingForModel(normalizeModelName(model))
		if err == nil {
			c.encoding = enc
		}
	}

	return c
}

// isOpenAIModel checks if the model is an OpenAI model.
func isOpenAIModel(model string) bool {
	model = strings.ToLower(model)
	return strings.HasPrefix(model, "gpt-") ||
		strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "text-") ||
		strings.HasPrefix(model, "davinci") ||
		strings.HasPrefix(model, "curie") ||
		strings.HasPrefix(model, "babbage") ||
		strings.HasPrefix(model, "ada")
}

// normalizeModelName normalizes model names for tiktoken lookup.
func normalizeModelName(model string) string {
	model = strings.ToLower(model)
	// Map common model aliases to tiktoken-recognized names
	switch {
	case strings.HasPrefix(model, "gpt-4o"):
		return "gpt-4o"
	case strings.HasPrefix(model, "gpt-4-turbo"):
		return "gpt-4-turbo"
	case strings.HasPrefix(model, "gpt-4"):
		return "gpt-4"
	case strings.HasPrefix(model, "gpt-3.5-turbo"):
		return "gpt-3.5-turbo"
	case strings.HasPrefix(model, "o1"):
		return "o1"
	default:
		return model
	}
}

// Count estimates tokens for a message using model-specific logic.
func (c *ModelTokenCounter) Count(msg provider.Message) int {
	tokens := c.CountText(msg.Content)

	// Add overhead for role and structure (~4 tokens per message for OpenAI)
	tokens += 4

	// Add tokens for tool calls if present
	for _, tc := range msg.ToolCalls {
		tokens += c.CountText(tc.Function.Name)
		tokens += c.CountText(tc.Function.Arguments)
		tokens += 4 // Overhead for tool call structure
	}

	return tokens
}

// CountText estimates tokens for text using model-specific logic.
func (c *ModelTokenCounter) CountText(text string) int {
	if text == "" {
		return 0
	}

	// Use tiktoken for OpenAI models
	if c.encoding != nil {
		return len(c.encoding.Encode(text, nil, nil))
	}

	// For Anthropic models, use ~3.5 chars per token (slightly more efficient than OpenAI)
	if isAnthropicModel(c.Model) {
		return (len(text)*10 + 34) / 35 // Equivalent to len/3.5 rounded up
	}

	// Fallback for other models
	if c.Fallback != nil {
		return c.Fallback.CountText(text)
	}
	return (&SimpleTokenCounter{}).CountText(text)
}

// isAnthropicModel checks if the model is an Anthropic model.
func isAnthropicModel(model string) bool {
	model = strings.ToLower(model)
	return strings.HasPrefix(model, "claude")
}

// TokenBudget tracks token usage against a budget.
type TokenBudget struct {
	// Total is the total token budget.
	Total int

	// Reserved is tokens reserved for response.
	Reserved int

	// Used is tokens already used.
	Used int
}

// Available returns the available tokens for context.
func (b *TokenBudget) Available() int {
	avail := b.Total - b.Reserved - b.Used
	if avail < 0 {
		return 0
	}
	return avail
}

// Consume adds to the used token count.
func (b *TokenBudget) Consume(tokens int) {
	b.Used += tokens
}

// Reset clears the used token count.
func (b *TokenBudget) Reset() {
	b.Used = 0
}

// OverBudget returns true if usage exceeds the budget.
func (b *TokenBudget) OverBudget() bool {
	return b.Used > b.Total-b.Reserved
}
