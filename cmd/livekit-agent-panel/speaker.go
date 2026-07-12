package main

import (
	"context"
	"fmt"
	"time"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omnivoice-core/tts"
)

// Speaker is the interface for entities that can speak.
type Speaker interface {
	GetVoice() string
	GetTTSProvider() tts.Provider
	GetAudioWriter() livekitagent.AudioWriter
	GetAgent() *livekitagent.Agent
	GetName() string
}

// SpeakText synthesizes text and sends it to the audio output.
// This is a shared implementation used by both Panelist and Moderator.
// If the speaker has an avatar, audio is routed to the avatar for lip-sync.
func SpeakText(ctx context.Context, speaker Speaker, text string) error {
	agent := speaker.GetAgent()

	// Check if we have an avatar to route audio through
	if agent != nil && agent.HasAvatar() {
		return speakToAvatar(ctx, speaker, text)
	}

	// Fall back to direct LiveKit audio
	return speakToLiveKit(ctx, speaker, text)
}

// speakToAvatar routes TTS audio through the avatar for lip-sync.
func speakToAvatar(ctx context.Context, speaker Speaker, text string) error {
	agent := speaker.GetAgent()
	avatarOut := agent.AvatarAudioOutput()
	if avatarOut == nil {
		return fmt.Errorf("avatar audio output not available for %s", speaker.GetName())
	}

	// Avatar expects 24kHz audio - no resampling needed
	result, err := speaker.GetTTSProvider().Synthesize(ctx, text, tts.SynthesisConfig{
		VoiceID:      speaker.GetVoice(),
		SampleRate:   24000,
		OutputFormat: "pcm",
	})
	if err != nil {
		return fmt.Errorf("TTS synthesis: %w", err)
	}

	// Write to avatar in 20ms frames (480 samples at 24kHz = 960 bytes)
	frameSize := 960
	for i := 0; i < len(result.Audio); i += frameSize {
		end := i + frameSize
		if end > len(result.Audio) {
			end = len(result.Audio)
		}
		frame := result.Audio[i:end]

		if err := avatarOut.CaptureFrame(ctx, frame); err != nil {
			return fmt.Errorf("write avatar audio: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Flush to signal end of utterance
	if err := avatarOut.Flush(ctx); err != nil {
		return fmt.Errorf("flush avatar audio: %w", err)
	}

	return nil
}

// speakToLiveKit writes TTS audio directly to LiveKit.
func speakToLiveKit(ctx context.Context, speaker Speaker, text string) error {
	audioWriter := speaker.GetAudioWriter()
	if audioWriter == nil {
		return fmt.Errorf("audio writer not set for %s", speaker.GetName())
	}

	result, err := speaker.GetTTSProvider().Synthesize(ctx, text, tts.SynthesisConfig{
		VoiceID:      speaker.GetVoice(),
		SampleRate:   24000,
		OutputFormat: "pcm",
	})
	if err != nil {
		return fmt.Errorf("TTS synthesis: %w", err)
	}

	audioData := result.Audio
	if result.SampleRate == 24000 {
		audioData = resample24to48(audioData)
	}

	// Write to LiveKit in 20ms frames (960 samples at 48kHz = 1920 bytes)
	frameSize := 1920
	for i := 0; i < len(audioData); i += frameSize {
		end := i + frameSize
		if end > len(audioData) {
			end = len(audioData)
		}
		frame := audioData[i:end]

		if _, err := audioWriter.Write(frame); err != nil {
			return fmt.Errorf("write audio: %w", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	return nil
}
