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
	GetName() string
}

// SpeakText synthesizes text and sends it to the audio output.
// This is a shared implementation used by both Panelist and Moderator.
func SpeakText(ctx context.Context, speaker Speaker, text string) error {
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
