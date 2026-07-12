package main

import (
	"context"
	"encoding/binary"
	"fmt"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omnivoice-core/tts"
)

// PanelistConfig configures a panelist.
type PanelistConfig struct {
	Name        string       // Display name (e.g., "Alex")
	Personality string       // Personality description
	Voice       string       // TTS voice ID
	AvatarID    string       // HeyGen avatar ID (optional)
	TTSProvider tts.Provider // TTS provider for speech synthesis
	LLMClient   LLMClient    // LLM client for response generation
}

// Panelist represents an AI panelist in the discussion.
type Panelist struct {
	Name        string
	Personality string
	Voice       string
	AvatarID    string
	ttsProvider tts.Provider
	llmClient   LLMClient
	audioWriter livekitagent.AudioWriter
	agent       *livekitagent.Agent
}

// NewPanelist creates a new panelist.
func NewPanelist(cfg PanelistConfig) *Panelist {
	return &Panelist{
		Name:        cfg.Name,
		Personality: cfg.Personality,
		Voice:       cfg.Voice,
		AvatarID:    cfg.AvatarID,
		ttsProvider: cfg.TTSProvider,
		llmClient:   cfg.LLMClient,
	}
}

// SetAgent sets the LiveKit agent for this panelist.
func (p *Panelist) SetAgent(ag *livekitagent.Agent) {
	p.agent = ag
}

// SetAudioWriter sets the audio writer for TTS output.
func (p *Panelist) SetAudioWriter(w livekitagent.AudioWriter) {
	p.audioWriter = w
}

// Speaker interface implementation

// GetVoice returns the TTS voice ID.
func (p *Panelist) GetVoice() string { return p.Voice }

// GetTTSProvider returns the TTS provider.
func (p *Panelist) GetTTSProvider() tts.Provider { return p.ttsProvider }

// GetAudioWriter returns the audio writer.
func (p *Panelist) GetAudioWriter() livekitagent.AudioWriter { return p.audioWriter }

// GetName returns the panelist's name.
func (p *Panelist) GetName() string { return p.Name }

// GetAgent returns the LiveKit agent.
func (p *Panelist) GetAgent() *livekitagent.Agent { return p.agent }

// GenerateResponse generates a response based on the transcript and topic.
func (p *Panelist) GenerateResponse(ctx context.Context, transcript *Transcript, question string) (string, error) {
	systemPrompt := BuildPanelistPrompt(p.Name, p.Personality, transcript, question)
	userMessage := "Please respond to the moderator's question as " + p.Name + "."
	return p.llmClient.Generate(ctx, systemPrompt, userMessage, 200)
}

// BuildPanelistPrompt builds the system prompt for a panelist response.
// Exported for testing.
func BuildPanelistPrompt(name, personality string, transcript *Transcript, question string) string {
	return fmt.Sprintf(`You are %s, a panelist in a discussion about "%s".

Your personality: %s

Guidelines:
- Keep responses to 2-4 sentences (panel format, not lectures)
- Build on what other panelists said - agree, disagree, extend, or offer a different angle
- Don't repeat points already made verbatim; add new perspective
- Address the moderator or reference other panelists by name naturally
- Stay in character with your personality throughout
- Speak conversationally as this is a voice discussion
- Do NOT use markdown formatting, asterisks, or special characters - this is speech

Current discussion transcript:
%s

The moderator just said: "%s"

Respond as %s:`, name, transcript.Topic(), personality, transcript.Format(), question, name)
}

// Speak synthesizes text and sends it to the audio output.
func (p *Panelist) Speak(ctx context.Context, text string) error {
	return SpeakText(ctx, p, text)
}

// DefaultPanelists returns the predefined panelist personalities.
func DefaultPanelists() []PanelistConfig {
	return []PanelistConfig{
		{
			Name:        "Alex",
			Personality: "Optimistic tech enthusiast who sees the benefits and potential in new technologies. Tends to highlight positive outcomes and opportunities.",
			Voice:       "alloy",
		},
		{
			Name:        "Jordan",
			Personality: "Pragmatic skeptic who asks hard questions and plays devil's advocate. Focuses on practical concerns, risks, and unintended consequences.",
			Voice:       "echo",
		},
		{
			Name:        "Morgan",
			Personality: "Academic expert who provides depth and context. Cites research, historical precedents, and nuanced analysis. Speaks thoughtfully.",
			Voice:       "onyx",
		},
		{
			Name:        "Casey",
			Personality: "Creative thinker who offers novel perspectives and unconventional ideas. Makes unexpected connections and thinks outside the box.",
			Voice:       "nova",
		},
	}
}

// resample24to48 upsamples 24kHz audio to 48kHz using linear interpolation.
func resample24to48(audio []byte) []byte {
	if len(audio) < 2 {
		return audio
	}

	numSamples := len(audio) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(audio[i*2:])) //nolint:gosec // G115: PCM audio uses full uint16->int16 range
	}

	resampled := make([]int16, numSamples*2)
	for i := 0; i < numSamples-1; i++ {
		resampled[i*2] = samples[i]
		resampled[i*2+1] = int16((int32(samples[i]) + int32(samples[i+1])) / 2) //nolint:gosec // G115: average of two int16 fits in int16
	}
	resampled[(numSamples-1)*2] = samples[numSamples-1]
	resampled[(numSamples-1)*2+1] = samples[numSamples-1]

	result := make([]byte, len(resampled)*2)
	for i, s := range resampled {
		binary.LittleEndian.PutUint16(result[i*2:], uint16(s)) //nolint:gosec // G115: int16->uint16 for binary encoding
	}
	return result
}
