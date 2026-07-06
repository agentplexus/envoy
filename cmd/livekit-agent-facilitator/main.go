// Command livekit-agent-facilitator demonstrates an OmniAgent-powered Facilitator
// with voice capabilities via LiveKit.
//
// The agent uses the Facilitator role for omnichannel collaboration:
//   - Facilitates meetings with structured agendas
//   - Tracks action items, decisions, and questions
//   - Generates meeting notes and summaries
//
// Usage:
//
//	export LIVEKIT_URL="wss://your-project.livekit.cloud"
//	export LIVEKIT_API_KEY="your-api-key"
//	export LIVEKIT_API_SECRET="your-api-secret"
//	export OPENAI_API_KEY="your-key"      # for LLM and TTS
//	export STT_PROVIDER="deepgram"
//	export DEEPGRAM_API_KEY="your-key"
//	export TTS_PROVIDER="openai"
//
//	# Optional: Enable avatar (static image in video tile)
//	export AGENT_AVATAR="true"            # Use default OmniAgent icon
//	export AGENT_AVATAR="/path/to/avatar.h264"  # Use custom pre-encoded avatar
//
//	# Optional: Realtime mode (native voice-to-voice, ~100-300ms latency)
//	export VOICE_MODE="realtime"          # "pipeline" (default) or "realtime"
//	export REALTIME_PROVIDER="openai"     # "openai", "gemini", or "deepgram"
//
//	go run -tags opus ./cmd/livekit-agent-facilitator
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omni-livekit/room"
	omniagent "github.com/plexusone/omniagent/agent"
	"github.com/plexusone/omnimeet-core/participant"
	"github.com/plexusone/omnimeet-core/track"
	facilitator "github.com/plexusone/omnirole-facilitator"
	"github.com/plexusone/omnivoice"
	"github.com/plexusone/omnivoice-core/stt"
	"github.com/plexusone/omnivoice-core/tts"
	_ "github.com/plexusone/omnivoice/providers/all" // Import all omnivoice providers
)

func main() {
	// Load config from environment
	serverURL := os.Getenv("LIVEKIT_URL")
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")

	if serverURL == "" || apiKey == "" || apiSecret == "" {
		log.Fatal("Required: LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET")
	}

	// Get voice mode configuration
	voiceMode := getEnvOrDefault("VOICE_MODE", "pipeline") // "pipeline" or "realtime"
	realtimeProvider := getEnvOrDefault("REALTIME_PROVIDER", "openai")

	// Get provider configuration
	sttProviderName := getEnvOrDefault("STT_PROVIDER", "deepgram")
	ttsProviderName := getEnvOrDefault("TTS_PROVIDER", "openai")
	llmProvider := getEnvOrDefault("LLM_PROVIDER", "openai")
	llmModel := getEnvOrDefault("LLM_MODEL", "gpt-4o")

	// Role configuration
	confluenceSpace := getEnvOrDefault("CONFLUENCE_SPACE", "MEETINGS")
	ahaProduct := getEnvOrDefault("AHA_PRODUCT", "")

	// Avatar configuration
	// AGENT_AVATAR: "true" or "1" to enable avatar (uses default OmniAgent icon)
	// AGENT_AVATAR: path to .h264 file for custom pre-encoded avatar
	avatarConfig := os.Getenv("AGENT_AVATAR")

	// Resolve API keys
	sttAPIKey := resolveAPIKey(sttProviderName)
	ttsAPIKey := resolveAPIKey(ttsProviderName)
	llmAPIKey := resolveAPIKey(llmProvider)

	if sttAPIKey == "" {
		log.Fatalf("No API key for STT provider '%s'. Set %s_API_KEY", sttProviderName, strings.ToUpper(sttProviderName))
	}
	if ttsAPIKey == "" {
		log.Fatalf("No API key for TTS provider '%s'. Set %s_API_KEY", ttsProviderName, strings.ToUpper(ttsProviderName))
	}
	if llmAPIKey == "" {
		log.Fatalf("No API key for LLM provider '%s'. Set %s_API_KEY", llmProvider, strings.ToUpper(llmProvider))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Create providers based on voice mode
	var sttProv stt.Provider
	var ttsProv tts.Provider
	var realtimeVA *RealtimeVoiceAgent

	if voiceMode == "realtime" {
		// Realtime mode: use native voice-to-voice provider
		realtimeAPIKey := resolveAPIKey(realtimeProvider)
		if realtimeAPIKey == "" {
			log.Fatalf("No API key for realtime provider '%s'. Set %s_API_KEY", realtimeProvider, strings.ToUpper(realtimeProvider))
		}
		fmt.Printf("Initializing realtime provider: %s\n", realtimeProvider)
	} else {
		// Pipeline mode: use STT + LLM + TTS
		fmt.Printf("Initializing STT provider: %s\n", sttProviderName)
		var err error
		sttProv, err = omnivoice.GetSTTProvider(sttProviderName, omnivoice.WithAPIKey(sttAPIKey))
		if err != nil {
			log.Fatalf("Failed to create STT provider: %v", err)
		}

		fmt.Printf("Initializing TTS provider: %s\n", ttsProviderName)
		ttsProv, err = omnivoice.GetTTSProvider(ttsProviderName, omnivoice.WithAPIKey(ttsAPIKey))
		if err != nil {
			log.Fatalf("Failed to create TTS provider: %v", err)
		}
	}

	// Create Meeting PM role
	pmRole := facilitator.New(facilitator.Config{
		DefaultConfluenceSpace: confluenceSpace,
		DefaultAhaProduct:      ahaProduct,
		EnableTranscription:    true,
		EnableActionTracking:   true,
	})

	// Get the role's system prompt
	// Note: For a voice-only demo, we use the system prompt directly
	// without requiring the full skill integration (meeting, google, confluence).
	systemPrompt, err := pmRole.SystemPrompt(ctx)
	if err != nil {
		log.Fatalf("Failed to get system prompt: %v", err)
	}

	// Add voice-specific instructions to the system prompt
	voicePrompt := systemPrompt + `

## Voice Interaction Guidelines

You are participating in a voice meeting via LiveKit. Keep these guidelines in mind:

- Keep responses brief and conversational (1-3 sentences when possible)
- Speak naturally as if in a real meeting
- Acknowledge what participants say before responding
- When tracking action items or decisions, confirm them verbally
- If you need clarification, ask follow-up questions
`

	// Create OmniAgent with the Meeting PM system prompt
	fmt.Printf("Initializing OmniAgent with Meeting PM role: %s/%s\n", llmProvider, llmModel)
	agent, err := omniagent.New(omniagent.Config{
		Provider:     llmProvider,
		Model:        llmModel,
		APIKey:       llmAPIKey,
		SystemPrompt: voicePrompt,
	})
	if err != nil {
		log.Fatalf("Failed to create OmniAgent: %v", err)
	}
	defer func() { _ = agent.Close() }()

	// Register web search tool (optional enhancement)
	searchTool, err := omniagent.NewSearchTool()
	if err != nil {
		log.Printf("Warning: Web search unavailable (set SERPER_API_KEY): %v", err)
	} else {
		agent.RegisterTool(searchTool)
		fmt.Println("Web search enabled (Serper)")
	}

	// Create room client
	roomClient, err := room.NewClient(room.Config{
		APIKey:    apiKey,
		APISecret: apiSecret,
		URL:       serverURL,
	})
	if err != nil {
		log.Fatalf("Failed to create room client: %v", err)
	}

	// Create a unique room name
	roomName := fmt.Sprintf("meeting-%d", time.Now().Unix())

	// Create the room
	fmt.Printf("Creating room: %s\n", roomName)
	if _, err := roomClient.CreateRoom(ctx, roomName); err != nil {
		log.Fatalf("Failed to create room: %v", err)
	}

	// Generate token for human participant
	humanToken, err := roomClient.GenerateClientToken(roomName, "human-user", "You")
	if err != nil {
		log.Fatalf("Failed to generate human token: %v", err)
	}

	// Build LiveKit Meet URL
	meetURL := buildMeetURL(serverURL, humanToken)

	// Start meeting session
	meetingID := roomName
	pmRole.StartSession(meetingID, "Team Standup")

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Meeting Program Manager (Voice Agent)")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Printf("Meeting:  %s\n", roomName)
	fmt.Printf("Mode:     %s\n", voiceMode)
	if voiceMode == "realtime" {
		fmt.Printf("Provider: %s (native voice-to-voice)\n", realtimeProvider)
	} else {
		fmt.Printf("STT:      %s\n", sttProviderName)
		fmt.Printf("TTS:      %s\n", ttsProviderName)
		fmt.Printf("LLM:      %s/%s\n", llmProvider, llmModel)
	}
	fmt.Printf("Role:     %s\n", pmRole.Name())
	fmt.Println()
	fmt.Println("Join the meeting to interact with the Meeting PM:")
	fmt.Println()
	fmt.Printf("  %s\n", meetURL)
	fmt.Println()
	fmt.Println("The Meeting PM will:")
	fmt.Println("  - Track action items and decisions")
	fmt.Println("  - Take notes during the meeting")
	fmt.Println("  - Generate meeting summaries")
	fmt.Println()
	fmt.Println("Starting agent...")

	// Select voice based on provider
	var va *VoiceAgent
	ttsVoice := getEnvOrDefault("TTS_VOICE", "")

	if voiceMode == "realtime" {
		// Realtime mode: create RealtimeVoiceAgent
		if ttsVoice == "" {
			switch realtimeProvider {
			case "openai":
				ttsVoice = "alloy"
			case "deepgram":
				ttsVoice = "aura-2-thalia-en"
			case "gemini":
				ttsVoice = "Puck"
			default:
				ttsVoice = "default"
			}
		}
		fmt.Printf("Voice:    %s\n", ttsVoice)

		realtimeAPIKey := resolveAPIKey(realtimeProvider)
		var err error
		realtimeVA, err = NewRealtimeVoiceAgent(RealtimeConfig{
			ProviderName: realtimeProvider,
			APIKey:       realtimeAPIKey,
			Voice:        ttsVoice,
			Instructions: voicePrompt,
			SampleRate:   48000,
		})
		if err != nil {
			log.Fatalf("Failed to create realtime voice agent: %v", err)
		}
		defer realtimeVA.Close()
	} else {
		// Pipeline mode: create VoiceAgent with STT + LLM + TTS
		if ttsVoice == "" {
			switch ttsProviderName {
			case "openai":
				ttsVoice = "alloy"
			case "deepgram":
				ttsVoice = "aura-asteria-en"
			case "elevenlabs":
				ttsVoice = "Rachel"
			default:
				ttsVoice = "default"
			}
		}
		fmt.Printf("TTS Voice: %s\n", ttsVoice)

		va = &VoiceAgent{
			sttProvider: sttProv,
			ttsProvider: ttsProv,
			ttsVoice:    ttsVoice,
			omniAgent:   agent,
			sessionID:   meetingID,
			sampleRate:  48000,
		}
	}

	// Build agent options
	agentOpts := livekitagent.Options{
		APIKey:        apiKey,
		APISecret:     apiSecret,
		ServerURL:     serverURL,
		Identity:      "meeting-pm",
		Name:          "Meeting PM",
		AutoSubscribe: true,
		Audio: livekitagent.AudioConfig{
			SampleRate: 48000,
			Channels:   1,
			TrackName:  "agent-audio",
		},
	}

	// Enable avatar if configured
	if avatarConfig != "" {
		agentOpts.MediaMode = livekitagent.AudioWithImage
		// If it's a path to a file, use it as custom avatar
		if avatarConfig != "true" && avatarConfig != "1" {
			agentOpts.Image.H264Path = avatarConfig
			fmt.Printf("Using custom avatar: %s\n", avatarConfig)
		} else {
			fmt.Println("Using default OmniAgent avatar")
		}
	}

	// Create LiveKit agent
	lkAgent, err := livekitagent.New(agentOpts)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Set up event handlers BEFORE joining (but greeting waits for audio ready)
	audioReady := make(chan struct{})
	greetedParticipants := make(map[string]bool)
	var greetedMu sync.Mutex

	lkAgent.OnParticipantJoined(func(p participant.Participant) {
		fmt.Printf("[+] %s joined (ID: %s)\n", p.Name, p.ID)

		// Simple test: speak 3 seconds after join (pipeline mode only)
		if voiceMode != "realtime" {
			go func() {
				<-audioReady // Wait for audio to be ready
				fmt.Println("[TEST] Waiting 3 seconds before test speech...")
				time.Sleep(3 * time.Second)
				fmt.Println("[TEST] Speaking test message...")
				if err := va.speak(ctx, lkAgent, "Testing one two three. Can you hear me?"); err != nil {
					log.Printf("[TEST] Error speaking: %v", err)
				} else {
					fmt.Println("[TEST] Test speech completed")
				}
			}()
		}
	})

	// Subscribe to audio when tracks are published (not when participant joins)
	lkAgent.OnTrackPublished(func(p participant.Participant, t track.Track) {
		fmt.Printf("[TRACK] %s published %s track: %s\n", p.Name, t.Kind, t.ID)

		if t.Kind != "audio" {
			return
		}

		go func() {
			// Wait for audio output to be ready
			<-audioReady

			// Subscribe to this participant's audio
			audioCh, err := lkAgent.SubscribeToAudio(ctx, p.ID)
			if err != nil {
				log.Printf("Warning: Could not subscribe to %s audio: %v", p.Name, err)
				return
			}
			fmt.Printf("[+] Subscribed to %s audio\n", p.Name)

			if voiceMode == "realtime" {
				// Realtime mode: start the realtime voice agent
				if err := realtimeVA.Start(ctx, lkAgent, audioCh); err != nil {
					log.Printf("Error starting realtime voice agent: %v", err)
				}
				// Greeting is handled by the realtime provider configuration
			} else {
				// Pipeline mode: use traditional STT + LLM + TTS
				go va.processAudio(ctx, lkAgent, audioCh)

				// Greet the participant (only once)
				greetedMu.Lock()
				alreadyGreeted := greetedParticipants[p.ID]
				if !alreadyGreeted {
					greetedParticipants[p.ID] = true
				}
				greetedMu.Unlock()

				if !alreadyGreeted {
					greeting := fmt.Sprintf("Hello %s! I'm the Meeting Program Manager. I'll be tracking action items, decisions, and key discussion points during our meeting. Feel free to ask me to note anything important.", p.Name)
					if err := va.speak(ctx, lkAgent, greeting); err != nil {
						log.Printf("Error greeting: %v", err)
					}
				}
			}
		}()
	})

	lkAgent.OnParticipantLeft(func(p participant.Participant) {
		fmt.Printf("[-] %s left\n", p.Name)
	})

	// Join the room
	if err := lkAgent.Join(ctx, roomName); err != nil {
		log.Fatalf("Failed to join room: %v", err)
	}
	fmt.Println("Meeting PM joined room!")

	// Start audio track for TTS output
	audioWriter, err := lkAgent.StartAudio(ctx)
	if err != nil {
		log.Fatalf("Failed to start audio: %v", err)
	}
	if voiceMode == "realtime" {
		realtimeVA.SetAudioWriter(audioWriter)
	} else {
		va.audioWriter = audioWriter
	}
	fmt.Println("Audio output ready")
	close(audioReady) // Signal that audio is ready

	// Note: Audio subscription happens per-participant in OnParticipantJoined
	fmt.Println("Audio subscription will activate when participants join...")

	fmt.Println()
	fmt.Println("Ready! Waiting for participants... (Ctrl+C to exit)")
	fmt.Println()

	// Wait for shutdown
	<-ctx.Done()

	// End meeting session and get summary
	session := pmRole.EndSession()
	if session != nil {
		fmt.Println("\n===========================================")
		fmt.Println("  Meeting Summary")
		fmt.Println("===========================================")
		fmt.Printf("Meeting: %s\n", session.MeetingName)
		fmt.Printf("Duration: %s\n", session.Duration())
		fmt.Printf("Action Items: %d\n", len(session.Actions))
		fmt.Printf("Decisions: %d\n", len(session.Decisions))
		fmt.Println()
	}

	// Cleanup
	if err := lkAgent.Leave(ctx); err != nil {
		log.Printf("Error leaving room: %v", err)
	}
	if err := roomClient.DeleteRoom(context.Background(), roomName); err != nil {
		log.Printf("Error deleting room: %v", err)
	}

	fmt.Println("Meeting ended. Goodbye!")
}

// VoiceAgent handles the voice processing pipeline with OmniAgent
type VoiceAgent struct {
	sttProvider stt.Provider
	ttsProvider tts.Provider
	ttsVoice    string
	omniAgent   *omniagent.Agent
	audioWriter livekitagent.AudioWriter
	sessionID   string
	speaking    bool
	speakingMu  sync.Mutex // protects speaking flag
	speakLock   sync.Mutex // serializes speak calls
	sampleRate  int
}

// processAudio handles incoming audio from participants
func (va *VoiceAgent) processAudio(ctx context.Context, ag *livekitagent.Agent, audioCh <-chan livekitagent.AudioFrame) {
	var audioBuffer []byte
	var lastSpeechTime time.Time
	silenceThreshold := 800 * time.Millisecond // Time after last speech to trigger processing
	frameCount := 0
	skippedWhileSpeaking := 0
	speechFrames := 0

	// Energy threshold for VAD (voice activity detection)
	// Frames below this RMS energy are considered silence/noise
	const energyThreshold = 500

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Heartbeat to confirm processAudio is still running
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-heartbeat.C:
			va.speakingMu.Lock()
			speaking := va.speaking
			va.speakingMu.Unlock()
			fmt.Printf("[HEARTBEAT] processAudio alive: frames=%d, speech=%d, buffer=%d bytes, speaking=%v, skipped=%d\n",
				frameCount, speechFrames, len(audioBuffer), speaking, skippedWhileSpeaking)

		case frame, ok := <-audioCh:
			if !ok {
				fmt.Println("[DEBUG] Audio channel closed")
				return
			}

			frameCount++
			if frameCount == 1 {
				fmt.Println("[DEBUG] Receiving audio frames...")
			}

			// Skip if we're currently speaking (avoid echo)
			va.speakingMu.Lock()
			speaking := va.speaking
			va.speakingMu.Unlock()
			if speaking {
				skippedWhileSpeaking++
				continue
			}

			// Simple VAD: calculate RMS energy of the frame
			energy := calculateRMSEnergy(frame.Data)
			isSpeech := energy > energyThreshold

			if isSpeech {
				speechFrames++
				audioBuffer = append(audioBuffer, frame.Data...)
				lastSpeechTime = time.Now()
				if speechFrames == 1 {
					fmt.Printf("[VAD] Speech started (energy=%d)\n", energy)
				}
			}

			if frameCount%100 == 0 {
				fmt.Printf("[DEBUG] Received %d frames, speech=%d, buffer=%d bytes, energy=%d\n",
					frameCount, speechFrames, len(audioBuffer), energy)
			}

		case <-ticker.C:
			// Check for silence (end of speech) - only if we have speech and enough silence has passed
			if len(audioBuffer) > 0 && !lastSpeechTime.IsZero() && time.Since(lastSpeechTime) > silenceThreshold {
				fmt.Printf("[VAD] Silence detected after speech, processing %d bytes (%d speech frames)\n", len(audioBuffer), speechFrames)
				audioCopy := make([]byte, len(audioBuffer))
				copy(audioCopy, audioBuffer)
				audioBuffer = nil
				speechFrames = 0
				lastSpeechTime = time.Time{} // Reset to zero value

				go func(audio []byte) {
					if err := va.processUtterance(ctx, ag, audio); err != nil {
						log.Printf("Error processing: %v", err)
					}
				}(audioCopy)
			}
		}
	}
}

// calculateRMSEnergy calculates the root mean square energy of PCM16 audio data.
// Returns a value indicating the "loudness" of the audio frame.
func calculateRMSEnergy(data []byte) int {
	if len(data) < 2 {
		return 0
	}

	var sumSquares int64
	numSamples := len(data) / 2

	for i := 0; i < numSamples; i++ {
		// Read little-endian int16
		sample := int16(binary.LittleEndian.Uint16(data[i*2:])) //nolint:gosec // G115: PCM audio uses full uint16→int16 range
		sumSquares += int64(sample) * int64(sample)
	}

	// RMS = sqrt(sum of squares / n)
	meanSquare := sumSquares / int64(numSamples)
	rms := int(math.Sqrt(float64(meanSquare)))
	return rms
}

// processUtterance handles a complete utterance: STT → OmniAgent → TTS
func (va *VoiceAgent) processUtterance(ctx context.Context, ag *livekitagent.Agent, audio []byte) error {
	// Minimum audio length check
	if len(audio) < 4800 {
		return nil
	}

	// 1. Speech-to-Text
	fmt.Print("[STT] Transcribing... ")
	text, err := va.transcribe(ctx, audio)
	if err != nil {
		return fmt.Errorf("STT: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		fmt.Println("(empty)")
		return nil
	}
	fmt.Printf("\"%s\"\n", text)

	// 2. Process with OmniAgent (Meeting PM role handles context)
	fmt.Print("[Meeting PM] Processing... ")
	response, err := va.omniAgent.Process(ctx, va.sessionID, text)
	if err != nil {
		return fmt.Errorf("OmniAgent: %w", err)
	}
	fmt.Printf("\"%s\"\n", response)

	// 3. Text-to-Speech
	fmt.Print("[TTS] Speaking... ")
	if err := va.speak(ctx, ag, response); err != nil {
		return fmt.Errorf("TTS: %w", err)
	}
	fmt.Println("done")

	return nil
}

// transcribe converts audio to text
func (va *VoiceAgent) transcribe(ctx context.Context, audio []byte) (string, error) {
	wavData := pcmToWav(audio, va.sampleRate, 1)

	result, err := va.sttProvider.Transcribe(ctx, wavData, stt.TranscriptionConfig{
		Language:   "en",
		SampleRate: va.sampleRate,
		Encoding:   "linear16",
	})
	if err != nil {
		return "", err
	}

	return result.Text, nil
}

// speak converts text to speech and sends to LiveKit
func (va *VoiceAgent) speak(ctx context.Context, _ *livekitagent.Agent, text string) error {
	// Serialize speak calls to prevent interleaved audio
	va.speakLock.Lock()
	defer va.speakLock.Unlock()

	// Set speaking flag (checked by processAudio to avoid echo)
	va.speakingMu.Lock()
	va.speaking = true
	va.speakingMu.Unlock()
	fmt.Println("[SPEAK] speaking=true (starting)")
	defer func() {
		va.speakingMu.Lock()
		va.speaking = false
		va.speakingMu.Unlock()
		fmt.Println("[SPEAK] speaking=false (finished)")
	}()

	fmt.Printf("[SPEAK] Starting TTS for: %q\n", text)

	// Use provider-appropriate voice
	voiceID := va.ttsVoice
	if voiceID == "" {
		voiceID = "aura-asteria-en" // Deepgram default
	}

	// Request PCM from TTS - Deepgram Opus returns Ogg container which doesn't work with WebRTC
	// Use linear16 (PCM) at 48kHz to match WebRTC's expected format
	fmt.Printf("[SPEAK] Calling TTS provider with voice: %s (linear16 format)\n", voiceID)
	result, err := va.ttsProvider.Synthesize(ctx, text, tts.SynthesisConfig{
		VoiceID:      voiceID,
		SampleRate:   48000,
		OutputFormat: "linear16",
	})
	if err != nil {
		return fmt.Errorf("TTS synthesis: %w", err)
	}
	fmt.Printf("[SPEAK] TTS returned %d bytes of PCM at %dHz\n", len(result.Audio), result.SampleRate)

	audioData := result.Audio

	// If TTS returned different sample rate, resample
	if result.SampleRate != 0 && result.SampleRate != 48000 {
		if result.SampleRate == 24000 {
			audioData = resample24to48(audioData)
			fmt.Printf("[SPEAK] Resampled to %d bytes at 48kHz\n", len(audioData))
		} else {
			return fmt.Errorf("unsupported sample rate: %d", result.SampleRate)
		}
	}

	// Write PCM to LiveKit - the agent's audio writer will encode to Opus
	// Write in 20ms frames (1920 bytes = 960 samples * 2 bytes at 48kHz mono)
	frameSize := 1920
	frameDuration := 20 * time.Millisecond
	frameCount := 0

	for i := 0; i < len(audioData); i += frameSize {
		end := i + frameSize
		var frame []byte
		if end > len(audioData) {
			// Pad last frame with silence (zeros)
			frame = make([]byte, frameSize)
			copy(frame, audioData[i:])
		} else {
			frame = audioData[i:end]
		}

		if _, err := va.audioWriter.Write(frame); err != nil {
			return fmt.Errorf("write frame %d: %w", frameCount, err)
		}
		frameCount++

		// Simple fixed-duration sleep for consistent pacing
		time.Sleep(frameDuration)
	}
	fmt.Printf("[SPEAK] Wrote %d PCM frames to LiveKit\n", frameCount)

	return nil
}

// Helper functions

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func resolveAPIKey(provider string) string {
	key := os.Getenv(strings.ToUpper(provider) + "_API_KEY")
	if key != "" {
		return key
	}
	switch provider {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "deepgram":
		return os.Getenv("DEEPGRAM_API_KEY")
	case "elevenlabs":
		return os.Getenv("ELEVENLABS_API_KEY")
	}
	return ""
}

func buildMeetURL(serverURL, token string) string {
	u, _ := url.Parse("https://meet.livekit.io/custom")
	q := u.Query()
	q.Set("liveKitUrl", serverURL)
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}

func pcmToWav(pcm []byte, sampleRate, channels int) []byte {
	var buf bytes.Buffer
	w := func(data any) { _ = binary.Write(&buf, binary.LittleEndian, data) }

	buf.WriteString("RIFF")
	w(uint32(36 + len(pcm))) //nolint:gosec // G115: PCM length bounded by audio frame size
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	w(uint32(16))
	w(uint16(1))
	w(uint16(channels))   //nolint:gosec // G115: channels is 1 or 2
	w(uint32(sampleRate)) //nolint:gosec // G115: sample rate bounded (8000-48000)
	w(uint32(sampleRate * channels * 2))
	w(uint16(channels * 2)) //nolint:gosec // G115: channels is 1 or 2
	w(uint16(16))

	buf.WriteString("data")
	w(uint32(len(pcm))) //nolint:gosec // G115: PCM length bounded by audio frame size
	buf.Write(pcm)

	return buf.Bytes()
}

func resample24to48(audio []byte) []byte {
	if len(audio) < 2 {
		return audio
	}

	numSamples := len(audio) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(audio[i*2:])) //nolint:gosec // G115: PCM audio uses full uint16→int16 range
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
		binary.LittleEndian.PutUint16(result[i*2:], uint16(s)) //nolint:gosec // G115: int16→uint16 for binary encoding
	}
	return result
}
