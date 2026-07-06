package main

import (
	"context"
	"fmt"
	"sync"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omnivoice"
	"github.com/plexusone/omnivoice-core/gateway"
	corereal "github.com/plexusone/omnivoice-core/realtime"
)

// RealtimeVoiceAgent handles voice processing using a realtime provider.
// This provides native voice-to-voice with ~100-300ms latency instead of
// the traditional STT → LLM → TTS pipeline.
type RealtimeVoiceAgent struct {
	provider     corereal.Provider
	audioWriter  livekitagent.AudioWriter
	sampleRate   int
	instructions string

	// Channels for the current session
	audioCh      <-chan corereal.AudioChunk
	transcriptCh <-chan corereal.Transcript

	speaking   bool
	speakingMu sync.Mutex
	closed     bool
	closedMu   sync.Mutex
}

// RealtimeConfig holds configuration for creating a RealtimeVoiceAgent.
type RealtimeConfig struct {
	ProviderName string // "openai", "gemini", "deepgram"
	APIKey       string
	Voice        string
	Instructions string
	SampleRate   int
}

// NewRealtimeVoiceAgent creates a new RealtimeVoiceAgent using the specified provider.
func NewRealtimeVoiceAgent(cfg RealtimeConfig) (*RealtimeVoiceAgent, error) {
	factory, err := omnivoice.GetRealtimeFactory(cfg.ProviderName)
	if err != nil {
		return nil, fmt.Errorf("realtime factory: %w", err)
	}

	provider, err := factory.Create(&gateway.RealtimeConfig{
		Provider:     cfg.ProviderName,
		APIKey:       cfg.APIKey,
		Voice:        cfg.Voice,
		Instructions: cfg.Instructions,
	})
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	return &RealtimeVoiceAgent{
		provider:     provider,
		sampleRate:   cfg.SampleRate,
		instructions: cfg.Instructions,
	}, nil
}

// Start begins processing audio with the realtime provider.
// Audio from the participant is sent to the provider, and responses are played back.
func (rva *RealtimeVoiceAgent) Start(ctx context.Context, ag *livekitagent.Agent, audioIn <-chan livekitagent.AudioFrame) error {
	// Convert AudioFrame channel to []byte channel
	audioBytes := make(chan []byte, 100)

	// Forward audio frames to bytes channel
	go func() {
		defer close(audioBytes)
		frameCount := 0
		skippedCount := 0
		sentCount := 0
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("[REALTIME-FWD] Context done. frames=%d sent=%d skipped=%d\n", frameCount, sentCount, skippedCount)
				return
			case frame, ok := <-audioIn:
				if !ok {
					fmt.Printf("[REALTIME-FWD] Input closed. frames=%d sent=%d skipped=%d\n", frameCount, sentCount, skippedCount)
					return
				}
				frameCount++
				// Skip if we're speaking (echo cancellation)
				rva.speakingMu.Lock()
				speaking := rva.speaking
				rva.speakingMu.Unlock()
				if speaking {
					skippedCount++
					if skippedCount%100 == 1 {
						fmt.Printf("[REALTIME-FWD] Skipping frames (echo cancel): %d skipped\n", skippedCount)
					}
					continue
				}
				select {
				case audioBytes <- frame.Data:
					sentCount++
					if sentCount == 1 {
						fmt.Printf("[REALTIME-FWD] First frame sent to provider: %d bytes\n", len(frame.Data))
					}
					if sentCount%200 == 0 {
						fmt.Printf("[REALTIME-FWD] Sent %d frames to provider\n", sentCount)
					}
				case <-ctx.Done():
					return
				default:
					// Channel full, skip this frame
					if frameCount%100 == 0 {
						fmt.Printf("[REALTIME-FWD] Channel full, dropping frame %d\n", frameCount)
					}
				}
			}
		}
	}()

	// Start the realtime session
	audioCh, transcriptCh, err := rva.provider.ProcessAudioStream(ctx, audioBytes, corereal.ProcessConfig{
		Instructions: rva.instructions,
	})
	if err != nil {
		return fmt.Errorf("start audio stream: %w", err)
	}

	rva.audioCh = audioCh
	rva.transcriptCh = transcriptCh

	// Process responses
	go rva.processResponses(ctx, ag)

	return nil
}

// processResponses handles audio and transcript output from the realtime provider.
func (rva *RealtimeVoiceAgent) processResponses(ctx context.Context, _ *livekitagent.Agent) {
	for {
		select {
		case <-ctx.Done():
			return

		case chunk, ok := <-rva.audioCh:
			if !ok {
				return
			}
			if len(chunk.Audio) > 0 {
				rva.speakingMu.Lock()
				rva.speaking = true
				rva.speakingMu.Unlock()

				if rva.audioWriter != nil {
					// Resample/convert if needed (provider outputs 24kHz, LiveKit may need 48kHz)
					audioData := chunk.Audio
					if rva.sampleRate == 48000 {
						// Simple 2x upsampling (24kHz → 48kHz)
						audioData = upsample2x(chunk.Audio)
					}
					rva.writeAudioToLiveKit(audioData)
				}
			}
			if chunk.IsFinal {
				rva.speakingMu.Lock()
				rva.speaking = false
				rva.speakingMu.Unlock()
				fmt.Println("[REALTIME] Agent finished speaking")
			}

		case transcript, ok := <-rva.transcriptCh:
			if !ok {
				return
			}
			if transcript.Text != "" {
				role := "AGENT"
				if transcript.IsInput {
					role = "USER"
				}
				fmt.Printf("[REALTIME] %s: %q\n", role, transcript.Text)
			}
		}
	}
}

// writeAudioToLiveKit writes audio data to the LiveKit audio track.
func (rva *RealtimeVoiceAgent) writeAudioToLiveKit(data []byte) {
	if rva.audioWriter == nil {
		return
	}

	// Write all data at once - the audio writer handles framing internally
	if _, err := rva.audioWriter.Write(data); err != nil {
		fmt.Printf("[REALTIME] Error writing audio: %v\n", err)
	}
}

// SetAudioWriter sets the LiveKit audio writer for output.
func (rva *RealtimeVoiceAgent) SetAudioWriter(w livekitagent.AudioWriter) {
	rva.audioWriter = w
}

// Close releases resources.
func (rva *RealtimeVoiceAgent) Close() error {
	rva.closedMu.Lock()
	defer rva.closedMu.Unlock()
	if rva.closed {
		return nil
	}
	rva.closed = true
	return rva.provider.Close()
}

// upsample2x performs simple 2x upsampling by duplicating samples.
// Input: 24kHz 16-bit mono PCM
// Output: 48kHz 16-bit mono PCM
func upsample2x(data []byte) []byte {
	result := make([]byte, len(data)*2)
	for i := 0; i < len(data); i += 2 {
		// Copy each sample twice
		result[i*2] = data[i]
		result[i*2+1] = data[i+1]
		result[i*2+2] = data[i]
		result[i*2+3] = data[i+1]
	}
	return result
}
