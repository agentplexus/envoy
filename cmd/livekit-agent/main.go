// Command livekit-agent demonstrates an OmniAgent-powered voice assistant
// with web search capabilities via LiveKit.
//
// The agent uses OmniAgent for LLM + web search (Serper) and responds via voice.
//
// Usage:
//
//	export LIVEKIT_URL="wss://your-project.livekit.cloud"
//	export LIVEKIT_API_KEY="your-api-key"
//	export LIVEKIT_API_SECRET="your-api-secret"
//	export ANTHROPIC_API_KEY="your-key"  # or OPENAI_API_KEY
//	export SERPER_API_KEY="your-key"     # for web search
//	export STT_PROVIDER="deepgram"
//	export DEEPGRAM_API_KEY="your-key"
//	export TTS_PROVIDER="openai"
//	export OPENAI_API_KEY="your-key"
//
//	# Optional: Enable avatar (static image in video tile)
//	export AGENT_AVATAR="true"            # Use default OmniAgent icon
//	export AGENT_AVATAR="/path/to/avatar.h264"  # Use custom pre-encoded avatar
//
//	go run -tags opus ./cmd/livekit-agent
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
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
	"github.com/plexusone/omnivoice"
	"github.com/plexusone/omnivoice-core/stt"
	"github.com/plexusone/omnivoice-core/tts"

	// Import all omnivoice providers
	_ "github.com/plexusone/omnivoice/providers/all"
)

func main() {
	// Load config from environment
	serverURL := os.Getenv("LIVEKIT_URL")
	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")

	if serverURL == "" || apiKey == "" || apiSecret == "" {
		log.Fatal("Required: LIVEKIT_URL, LIVEKIT_API_KEY, LIVEKIT_API_SECRET")
	}

	// Get provider configuration
	sttProviderName := getEnvOrDefault("STT_PROVIDER", "deepgram")
	ttsProviderName := getEnvOrDefault("TTS_PROVIDER", "openai")
	llmProvider := getEnvOrDefault("LLM_PROVIDER", "anthropic")
	llmModel := getEnvOrDefault("LLM_MODEL", "claude-sonnet-4-20250514")

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

	// Create STT provider via omnivoice registry
	fmt.Printf("Initializing STT provider: %s\n", sttProviderName)
	sttProv, err := omnivoice.GetSTTProvider(sttProviderName, omnivoice.WithAPIKey(sttAPIKey))
	if err != nil {
		log.Fatalf("Failed to create STT provider: %v", err)
	}

	// Create TTS provider via omnivoice registry
	fmt.Printf("Initializing TTS provider: %s\n", ttsProviderName)
	ttsProv, err := omnivoice.GetTTSProvider(ttsProviderName, omnivoice.WithAPIKey(ttsAPIKey))
	if err != nil {
		log.Fatalf("Failed to create TTS provider: %v", err)
	}

	// Create OmniAgent with web search enabled
	fmt.Printf("Initializing OmniAgent: %s/%s\n", llmProvider, llmModel)
	omniAgent, err := omniagent.New(omniagent.Config{
		Provider: llmProvider,
		Model:    llmModel,
		APIKey:   llmAPIKey,
		SystemPrompt: `You are a helpful voice assistant with web search capabilities.

When answering questions:
- Use the web_search tool to find current information, news, or facts you're unsure about
- Keep responses brief and conversational (1-3 sentences)
- Speak naturally as if having a phone conversation
- If you search the web, briefly mention what you found

Examples of when to search:
- Current events, news, weather
- Recent releases, updates, or changes
- Specific facts you're uncertain about
- Real-time data like stock prices or sports scores`,
	})
	if err != nil {
		log.Fatalf("Failed to create OmniAgent: %v", err)
	}
	defer func() { _ = omniAgent.Close() }()

	// Register web search tool
	searchTool, err := omniagent.NewSearchTool()
	if err != nil {
		log.Printf("Warning: Web search unavailable (set SERPER_API_KEY): %v", err)
	} else {
		omniAgent.RegisterTool(searchTool)
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
	roomName := fmt.Sprintf("omniagent-%d", time.Now().Unix())

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

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  OmniAgent Voice Assistant")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Printf("Room:     %s\n", roomName)
	fmt.Printf("STT:      %s\n", sttProviderName)
	fmt.Printf("TTS:      %s\n", ttsProviderName)
	fmt.Printf("LLM:      %s/%s\n", llmProvider, llmModel)
	fmt.Printf("Search:   %v\n", searchTool != nil)
	fmt.Println()
	fmt.Println("Join the meeting to talk to the AI agent:")
	fmt.Println()
	fmt.Printf("  %s\n", meetURL)
	fmt.Println()
	fmt.Println("Try asking:")
	fmt.Println("  - \"What's the latest news about AI?\"")
	fmt.Println("  - \"What's the weather like in San Francisco?\"")
	fmt.Println("  - \"Tell me about yourself\"")
	fmt.Println()
	fmt.Println("Starting agent...")

	// Create voice agent wrapper
	va := &VoiceAgent{
		sttProvider: sttProv,
		ttsProvider: ttsProv,
		omniAgent:   omniAgent,
		sessionID:   fmt.Sprintf("voice-%d", time.Now().UnixNano()),
		sampleRate:  48000,
	}

	// Build agent options
	agentOpts := livekitagent.Options{
		APIKey:        apiKey,
		APISecret:     apiSecret,
		ServerURL:     serverURL,
		Identity:      "omniagent",
		Name:          "AI Assistant",
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

	// Set up event handlers
	lkAgent.OnParticipantJoined(func(p participant.Participant) {
		fmt.Printf("[+] %s joined\n", p.Name)
		go func() {
			greeting := fmt.Sprintf("Hello %s! I'm your AI assistant with web search capabilities. Ask me anything - I can look up current information if needed.", p.Name)
			if err := va.speak(ctx, lkAgent, greeting); err != nil {
				log.Printf("Error greeting: %v", err)
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
	fmt.Println("Agent joined room!")

	// Start audio track for TTS output
	audioWriter, err := lkAgent.StartAudio(ctx)
	if err != nil {
		log.Fatalf("Failed to start audio: %v", err)
	}
	va.audioWriter = audioWriter

	// Subscribe to audio from participants
	audioCh, err := lkAgent.SubscribeToAllAudio(ctx)
	if err != nil {
		log.Printf("Warning: Could not subscribe to audio: %v", err)
	} else {
		go va.processAudio(ctx, lkAgent, audioCh)
	}

	fmt.Println()
	fmt.Println("Ready! Waiting for participants... (Ctrl+C to exit)")
	fmt.Println()

	// Wait for shutdown
	<-ctx.Done()

	// Cleanup
	if err := lkAgent.Leave(ctx); err != nil {
		log.Printf("Error leaving room: %v", err)
	}
	if err := roomClient.DeleteRoom(context.Background(), roomName); err != nil {
		log.Printf("Error deleting room: %v", err)
	}

	fmt.Println("Goodbye!")
}

// VoiceAgent handles the voice processing pipeline with OmniAgent
type VoiceAgent struct {
	sttProvider stt.Provider
	ttsProvider tts.Provider
	omniAgent   *omniagent.Agent
	audioWriter livekitagent.AudioWriter
	sessionID   string
	speaking    bool
	speakingMu  sync.Mutex
	sampleRate  int
}

// processAudio handles incoming audio from participants
func (va *VoiceAgent) processAudio(ctx context.Context, ag *livekitagent.Agent, audioCh <-chan livekitagent.AudioFrame) {
	var audioBuffer []byte
	var lastAudioTime time.Time
	silenceThreshold := 500 * time.Millisecond

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case frame, ok := <-audioCh:
			if !ok {
				return
			}

			// Skip if we're currently speaking (avoid echo)
			va.speakingMu.Lock()
			speaking := va.speaking
			va.speakingMu.Unlock()
			if speaking {
				continue
			}

			audioBuffer = append(audioBuffer, frame.Data...)
			lastAudioTime = time.Now()

		case <-ticker.C:
			// Check for silence (end of speech)
			if len(audioBuffer) > 0 && time.Since(lastAudioTime) > silenceThreshold {
				audioCopy := make([]byte, len(audioBuffer))
				copy(audioCopy, audioBuffer)
				audioBuffer = nil

				go func(audio []byte) {
					if err := va.processUtterance(ctx, ag, audio); err != nil {
						log.Printf("Error processing: %v", err)
					}
				}(audioCopy)
			}
		}
	}
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

	// 2. Process with OmniAgent (LLM + optional web search)
	fmt.Print("[OmniAgent] Processing... ")
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
	va.speakingMu.Lock()
	va.speaking = true
	va.speakingMu.Unlock()
	defer func() {
		va.speakingMu.Lock()
		va.speaking = false
		va.speakingMu.Unlock()
	}()

	result, err := va.ttsProvider.Synthesize(ctx, text, tts.SynthesisConfig{
		VoiceID:      "alloy",
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

	// Write to LiveKit in 20ms frames
	frameSize := 1920
	for i := 0; i < len(audioData); i += frameSize {
		end := i + frameSize
		if end > len(audioData) {
			end = len(audioData)
		}
		frame := audioData[i:end]

		if _, err := va.audioWriter.Write(frame); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}

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
	w(uint16(channels))                  //nolint:gosec // G115: channels is 1 or 2
	w(uint32(sampleRate))                //nolint:gosec // G115: sample rate bounded (8000-48000)
	w(uint32(sampleRate * channels * 2)) //nolint:gosec // G115: byte rate bounded by sample rate and channels
	w(uint16(channels * 2))              //nolint:gosec // G115: channels is 1 or 2
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
