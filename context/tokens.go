package context

import (
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
}

// Count estimates tokens for a message using model-specific logic.
func (c *ModelTokenCounter) Count(msg provider.Message) int {
	// TODO: Implement model-specific token counting
	// For now, use tiktoken for OpenAI models, fallback for others

	if c.Fallback != nil {
		return c.Fallback.Count(msg)
	}
	return (&SimpleTokenCounter{}).Count(msg)
}

// CountText estimates tokens for text using model-specific logic.
func (c *ModelTokenCounter) CountText(text string) int {
	if c.Fallback != nil {
		return c.Fallback.CountText(text)
	}
	return (&SimpleTokenCounter{}).CountText(text)
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
