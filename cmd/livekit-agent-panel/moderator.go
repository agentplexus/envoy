package main

import (
	"context"
	"fmt"
	"strings"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omnivoice-core/tts"
)

// ModeratorConfig configures an AI moderator.
type ModeratorConfig struct {
	Name        string       // Display name (e.g., "Sam")
	Personality string       // Personality description
	Voice       string       // TTS voice ID
	AvatarID    string       // HeyGen avatar ID (optional)
	TTSProvider tts.Provider // TTS provider for speech synthesis
	LLMClient   LLMClient    // LLM client for question generation
	Questions   []string     // Pre-defined questions (optional)
	NumRounds   int          // Number of discussion rounds (if no pre-defined questions)
}

// Moderator represents an AI moderator for the panel.
type Moderator struct {
	Name        string
	Personality string
	Voice       string
	AvatarID    string
	ttsProvider tts.Provider
	llmClient   LLMClient
	questions   []string
	numRounds   int
	audioWriter livekitagent.AudioWriter
	agent       *livekitagent.Agent
}

// NewModerator creates a new AI moderator.
func NewModerator(cfg ModeratorConfig) *Moderator {
	numRounds := cfg.NumRounds
	if numRounds == 0 && len(cfg.Questions) == 0 {
		numRounds = 5 // Default to 5 rounds if no questions provided
	}

	return &Moderator{
		Name:        cfg.Name,
		Personality: cfg.Personality,
		Voice:       cfg.Voice,
		AvatarID:    cfg.AvatarID,
		ttsProvider: cfg.TTSProvider,
		llmClient:   cfg.LLMClient,
		questions:   cfg.Questions,
		numRounds:   numRounds,
	}
}

// SetAgent sets the LiveKit agent for this moderator.
func (m *Moderator) SetAgent(ag *livekitagent.Agent) {
	m.agent = ag
}

// SetAudioWriter sets the audio writer for TTS output.
func (m *Moderator) SetAudioWriter(w livekitagent.AudioWriter) {
	m.audioWriter = w
}

// Speaker interface implementation

// GetVoice returns the TTS voice ID.
func (m *Moderator) GetVoice() string { return m.Voice }

// GetTTSProvider returns the TTS provider.
func (m *Moderator) GetTTSProvider() tts.Provider { return m.ttsProvider }

// GetAudioWriter returns the audio writer.
func (m *Moderator) GetAudioWriter() livekitagent.AudioWriter { return m.audioWriter }

// GetName returns the moderator's name.
func (m *Moderator) GetName() string { return m.Name }

// GetAgent returns the LiveKit agent.
func (m *Moderator) GetAgent() *livekitagent.Agent { return m.agent }

// GenerateIntroduction generates the opening statement for the panel.
func (m *Moderator) GenerateIntroduction(ctx context.Context, topic string, panelistNames []string) (string, error) {
	systemPrompt := BuildModeratorIntroPrompt(m.Name, m.Personality, topic, panelistNames)
	return m.llmClient.Generate(ctx, systemPrompt, "Please introduce the panel discussion.", 150)
}

// GenerateQuestion generates a question for the panel.
// If pre-defined questions exist, returns the next one.
// Otherwise, generates a dynamic question based on the transcript.
func (m *Moderator) GenerateQuestion(ctx context.Context, transcript *Transcript, roundNum int) (string, error) {
	// Use pre-defined questions if available
	if len(m.questions) > 0 {
		idx := roundNum - 1
		if idx < len(m.questions) {
			return m.questions[idx], nil
		}
		// Fall through to generate dynamic question if we've exhausted pre-defined ones
	}

	systemPrompt := BuildModeratorQuestionPrompt(m.Name, m.Personality, transcript)
	return m.llmClient.Generate(ctx, systemPrompt, "Please ask your next question to the panel.", 100)
}

// GenerateClosing generates the closing statement for the panel.
func (m *Moderator) GenerateClosing(ctx context.Context, transcript *Transcript) (string, error) {
	systemPrompt := BuildModeratorClosingPrompt(m.Name, m.Personality, transcript)
	return m.llmClient.Generate(ctx, systemPrompt, "Please provide a brief closing statement.", 150)
}

// Speak synthesizes text and sends it to the audio output.
func (m *Moderator) Speak(ctx context.Context, text string) error {
	return SpeakText(ctx, m, text)
}

// TotalRounds returns the total number of rounds to run.
func (m *Moderator) TotalRounds() int {
	if len(m.questions) > 0 {
		return len(m.questions)
	}
	return m.numRounds
}

// BuildModeratorIntroPrompt builds the system prompt for the moderator's introduction.
func BuildModeratorIntroPrompt(name, personality, topic string, panelistNames []string) string {
	return fmt.Sprintf(`You are %s, the moderator of a panel discussion about "%s".

Your personality: %s

You are introducing the panel discussion. The panelists are: %s.

Guidelines:
- Keep it brief (2-3 sentences)
- Welcome the audience and introduce the topic
- Briefly mention you have an excellent panel today
- Do NOT use markdown formatting, asterisks, or special characters - this is speech
- Speak naturally and conversationally`, name, topic, personality, strings.Join(panelistNames, ", "))
}

// BuildModeratorQuestionPrompt builds the system prompt for generating questions.
func BuildModeratorQuestionPrompt(name, personality string, transcript *Transcript) string {
	return fmt.Sprintf(`You are %s, the moderator of a panel discussion about "%s".

Your personality: %s

Current discussion transcript:
%s

Guidelines:
- Ask a thought-provoking question that advances the discussion
- Build on what panelists have said - ask for clarification, deeper exploration, or new angles
- Keep questions concise (1-2 sentences)
- You can address a specific panelist or the whole panel
- Do NOT use markdown formatting, asterisks, or special characters - this is speech
- Do NOT repeat questions that have already been asked`, name, transcript.Topic(), personality, transcript.Format())
}

// BuildModeratorClosingPrompt builds the system prompt for the closing statement.
func BuildModeratorClosingPrompt(name, personality string, transcript *Transcript) string {
	return fmt.Sprintf(`You are %s, the moderator of a panel discussion about "%s".

Your personality: %s

The discussion transcript:
%s

Guidelines:
- Summarize 1-2 key takeaways from the discussion
- Thank the panelists
- Keep it brief (2-3 sentences)
- Do NOT use markdown formatting, asterisks, or special characters - this is speech`, name, transcript.Topic(), personality, transcript.Format())
}

// DefaultModeratorConfig returns the default moderator configuration.
func DefaultModeratorConfig() ModeratorConfig {
	return ModeratorConfig{
		Name:        "Sam",
		Personality: "Engaging and thoughtful moderator who asks probing questions, keeps the discussion on track, and ensures all panelists have a chance to contribute.",
		Voice:       "shimmer",
		NumRounds:   5,
	}
}
