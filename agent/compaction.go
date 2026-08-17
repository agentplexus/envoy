package agent

import (
	"context"
	"fmt"

	"github.com/plexusone/omnillm/provider"
)

// defaultCompactionPrompt is used when Config.CompactionPrompt is empty.
const defaultCompactionPrompt = "Summarize the following conversation concisely, preserving key facts, decisions, and open questions. Write the summary as a neutral third-person recap, not as a chat message."

// compactionSummaryMaxTokens caps the summary response size — it should
// stay short regardless of how large Config.MaxTokens is for the main
// conversation.
const compactionSummaryMaxTokens = 512

// summarizeMessages asks the agent's own LLM (same provider/model as the
// main conversation) to condense a batch of older messages into a single
// summary string, for context-window compaction (RMI-OMNIAGENT-027). It
// matches context.SummarizeFunc's signature and is wired in by
// WithCompaction.
func (a *Agent) summarizeMessages(ctx context.Context, messages []provider.Message) (string, error) {
	prompt := a.config.CompactionPrompt
	if prompt == "" {
		prompt = defaultCompactionPrompt
	}
	maxTokens := compactionSummaryMaxTokens

	req := &provider.ChatCompletionRequest{
		Model: a.config.Model,
		Messages: append([]provider.Message{
			{Role: provider.RoleSystem, Content: prompt},
		}, messages...),
		MaxTokens: &maxTokens,
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("summarize messages: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("summarize messages: no choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}
