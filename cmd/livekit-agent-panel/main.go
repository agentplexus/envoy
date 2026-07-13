// Command livekit-agent-panel runs a multi-agent panel discussion in LiveKit.
//
// Supports two modes:
//   - Human mode (default): Human moderator asks questions via voice
//   - Auto mode: AI moderator runs the entire discussion automatically
//
// Usage:
//
//	export LIVEKIT_URL="wss://your-project.livekit.cloud"
//	export LIVEKIT_API_KEY="your-api-key"
//	export LIVEKIT_API_SECRET="your-api-secret"
//	export LLM_PROVIDER="anthropic"        # or "openai"
//	export LLM_MODEL="claude-sonnet-4-20250514"
//	export ANTHROPIC_API_KEY="your-key"    # or OPENAI_API_KEY
//	export STT_PROVIDER="deepgram"         # for human mode
//	export DEEPGRAM_API_KEY="your-key"
//	export TTS_PROVIDER="openai"
//	export OPENAI_API_KEY="your-key"
//
//	# Panel configuration
//	export PANEL_TOPIC="The future of AI agents"
//	export PANEL_SIZE=3  # 2-4 panelists
//	export PANEL_MODE=human  # "human" or "auto"
//
//	# Auto mode configuration (optional)
//	export PANEL_QUESTIONS="What are the benefits?,What are the risks?,What's next?"
//	export PANEL_ROUNDS=5  # If no questions provided
//	export MODERATOR_NAME="Sam"
//	export MODERATOR_VOICE="shimmer"
//	export MODERATOR_PERSONALITY="Engaging moderator who asks probing questions"
//
//	# Optional: Override default panelists
//	export PANELIST_1_NAME="Dr. Sarah"
//	export PANELIST_1_VOICE="nova"
//	export PANELIST_1_PERSONALITY="A physician specializing in AI diagnostics"
//
//	# HeyGen Avatar Configuration (optional)
//	export HEYGEN_API_KEY="your-key"
//	export HEYGEN_SANDBOX=false           # Set to "true" for sandbox mode
//	export HEYGEN_VIDEO_QUALITY="high"    # "high", "medium", or "low"
//	export MODERATOR_AVATAR_ID="avatar-id"
//	export PANELIST_1_AVATAR_ID="avatar-id"
//	export PANELIST_2_AVATAR_ID="avatar-id"
//
//	go run -tags opus ./cmd/livekit-agent-panel
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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	livekitagent "github.com/plexusone/omni-livekit/agent"
	"github.com/plexusone/omni-livekit/room"
	"github.com/plexusone/omniagent/cmd/livekit-agent-panel/panel"
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

	// Provider configuration
	ttsProviderName := getEnvOrDefault("TTS_PROVIDER", "openai")
	llmProvider := getEnvOrDefault("LLM_PROVIDER", "anthropic")
	llmModel := getEnvOrDefault("LLM_MODEL", "claude-sonnet-4-20250514")

	// Panel configuration
	panelMode := getEnvOrDefault("PANEL_MODE", "human")
	panelTopic := getEnvOrDefault("PANEL_TOPIC", "The future of artificial intelligence")
	panelSize := getEnvIntOrDefault("PANEL_SIZE", 3)
	if panelSize < 2 {
		panelSize = 2
	}
	if panelSize > 4 {
		panelSize = 4
	}

	// Resolve API keys
	ttsAPIKey := resolveAPIKey(ttsProviderName)
	llmKey := resolveAPIKey(llmProvider)

	if ttsAPIKey == "" {
		log.Fatalf("No API key for TTS provider '%s'. Set %s_API_KEY", ttsProviderName, strings.ToUpper(ttsProviderName))
	}
	if llmKey == "" {
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

	// Create TTS provider
	fmt.Printf("Initializing TTS provider: %s\n", ttsProviderName)
	ttsProv, err := omnivoice.GetTTSProvider(ttsProviderName, omnivoice.WithAPIKey(ttsAPIKey))
	if err != nil {
		log.Fatalf("Failed to create TTS provider: %v", err)
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
	roomName := fmt.Sprintf("panel-%d", time.Now().Unix())

	// Create the room
	fmt.Printf("Creating room: %s\n", roomName)
	if _, err := roomClient.CreateRoom(ctx, roomName); err != nil {
		log.Fatalf("Failed to create room: %v", err)
	}

	// Create shared LLM client
	llmClient := NewLLMClient(llmProvider, llmKey, llmModel)

	// Create panelists
	panelists := createPanelists(panelSize, ttsProv, llmClient)

	// Create shared transcript
	transcript := NewTranscript(panelTopic)

	// Create coordinator
	coordinator := NewCoordinator(panelists, transcript, panelTopic, llmClient)

	// Run in selected mode
	switch panelMode {
	case "auto":
		runAutoMode(ctx, serverURL, apiKey, apiSecret, roomName, roomClient, coordinator, panelists, ttsProv, llmClient, llmProvider, llmModel)
	default:
		runHumanMode(ctx, serverURL, apiKey, apiSecret, roomName, roomClient, coordinator, panelists, llmProvider, llmModel)
	}

	// Cleanup
	if err := roomClient.DeleteRoom(context.Background(), roomName); err != nil {
		log.Printf("Error deleting room: %v", err)
	}

	fmt.Println("Goodbye!")
}

// runHumanMode runs the panel with a human moderator.
func runHumanMode(
	ctx context.Context,
	serverURL, apiKey, apiSecret, roomName string,
	roomClient *room.Client,
	coordinator *Coordinator,
	panelists []*Panelist,
	llmProvider, llmModel string,
) {
	// STT provider needed for human mode
	sttProviderName := getEnvOrDefault("STT_PROVIDER", "deepgram")
	sttAPIKey := resolveAPIKey(sttProviderName)
	if sttAPIKey == "" {
		log.Fatalf("No API key for STT provider '%s'. Set %s_API_KEY", sttProviderName, strings.ToUpper(sttProviderName))
	}

	fmt.Printf("Initializing STT provider: %s\n", sttProviderName)
	sttProv, err := omnivoice.GetSTTProvider(sttProviderName, omnivoice.WithAPIKey(sttAPIKey))
	if err != nil {
		log.Fatalf("Failed to create STT provider: %v", err)
	}

	// Generate token for human moderator
	humanToken, err := roomClient.GenerateClientToken(roomName, "moderator", "Moderator")
	if err != nil {
		log.Fatalf("Failed to generate moderator token: %v", err)
	}
	meetURL := buildMeetURL(serverURL, humanToken)

	// Print info
	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Panel Discussion (Human Moderator)")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Printf("Topic:      %s\n", coordinator.Topic)
	fmt.Printf("Room:       %s\n", roomName)
	fmt.Printf("Panelists:  %s\n", strings.Join(coordinator.PanelistNames(), ", "))
	fmt.Printf("STT:        %s\n", sttProviderName)
	fmt.Printf("LLM:        %s/%s\n", llmProvider, llmModel)
	fmt.Println()
	fmt.Println("Join as Moderator:")
	fmt.Println()
	fmt.Printf("  %s\n", meetURL)
	fmt.Println()
	fmt.Println("Starting panelists...")

	// Create and join panelist agents
	var lkAgents []*livekitagent.Agent
	for i, panelist := range panelists {
		ag := createPanelistAgent(ctx, serverURL, apiKey, apiSecret, roomName, panelist, i)
		lkAgents = append(lkAgents, ag)
	}

	// Create STT listener agent
	listenerAgent := createListenerAgent(ctx, serverURL, apiKey, apiSecret, roomName)
	lkAgents = append(lkAgents, listenerAgent)

	// Subscribe to moderator audio
	audioCh, err := listenerAgent.SubscribeToAllAudio(ctx)
	if err != nil {
		log.Printf("Warning: Could not subscribe to audio: %v", err)
	}

	// Create audio processor
	processor := &AudioProcessor{
		sttProvider: sttProv,
		coordinator: coordinator,
		sampleRate:  48000,
	}

	// Wait for moderator to join
	fmt.Println()
	fmt.Println("Waiting for moderator to join...")
	fmt.Println()

	moderatorJoined := make(chan struct{}, 1)
	var moderatorOnce sync.Once

	listenerAgent.OnParticipantJoined(func(p participant.Participant) {
		if p.Name == "Moderator" {
			moderatorOnce.Do(func() {
				fmt.Printf("[+] Moderator joined\n")
				close(moderatorJoined)
			})
		}
	})

	select {
	case <-moderatorJoined:
		time.Sleep(500 * time.Millisecond)
		if err := coordinator.RunIntroductions(ctx); err != nil {
			log.Printf("Error during introductions: %v", err)
		}
	case <-ctx.Done():
		cleanupAgents(lkAgents)
		return
	case <-time.After(5 * time.Minute):
		fmt.Println("Timeout waiting for moderator. Proceeding anyway...")
		if err := coordinator.RunIntroductions(ctx); err != nil {
			log.Printf("Error during introductions: %v", err)
		}
	}

	// Process incoming audio
	if audioCh != nil {
		go processor.ProcessAudio(ctx, audioCh)
	}

	fmt.Println()
	fmt.Println("Ready! Speak a question as the moderator. (Ctrl+C to exit)")
	fmt.Println()

	<-ctx.Done()
	cleanupAgents(lkAgents)
}

// runAutoMode runs the panel with an AI moderator.
func runAutoMode(
	ctx context.Context,
	serverURL, apiKey, apiSecret, roomName string,
	roomClient *room.Client,
	coordinator *Coordinator,
	panelists []*Panelist,
	ttsProv tts.Provider,
	llmClient LLMClient,
	llmProvider, llmModel string,
) {
	// Auto mode configuration
	panelQuestions := os.Getenv("PANEL_QUESTIONS")
	panelRounds := getEnvIntOrDefault("PANEL_ROUNDS", 5)

	// Parse questions
	var questions []string
	if panelQuestions != "" {
		questions = strings.Split(panelQuestions, ",")
		for i := range questions {
			questions[i] = strings.TrimSpace(questions[i])
		}
	}

	// Moderator configuration
	modDefaults := DefaultModeratorConfig()
	moderator := NewModerator(ModeratorConfig{
		Name:        getEnvOrDefault("MODERATOR_NAME", modDefaults.Name),
		Personality: getEnvOrDefault("MODERATOR_PERSONALITY", modDefaults.Personality),
		Voice:       getEnvOrDefault("MODERATOR_VOICE", modDefaults.Voice),
		AvatarID:    os.Getenv("MODERATOR_AVATAR_ID"),
		TTSProvider: ttsProv,
		LLMClient:   llmClient,
		Questions:   questions,
		NumRounds:   panelRounds,
	})

	// Generate token for observer (optional viewing)
	observerToken, err := roomClient.GenerateClientToken(roomName, "observer", "Observer")
	if err != nil {
		log.Printf("Warning: Could not generate observer token: %v", err)
	}
	meetURL := buildMeetURL(serverURL, observerToken)

	// Print info
	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("  Panel Discussion (Auto Mode)")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Printf("Topic:      %s\n", coordinator.Topic)
	fmt.Printf("Room:       %s\n", roomName)
	fmt.Printf("Moderator:  %s\n", moderator.Name)
	fmt.Printf("Panelists:  %s\n", strings.Join(coordinator.PanelistNames(), ", "))
	fmt.Printf("Rounds:     %d\n", moderator.TotalRounds())
	fmt.Printf("LLM:        %s/%s\n", llmProvider, llmModel)
	fmt.Println()
	fmt.Println("Watch the discussion (optional):")
	fmt.Println()
	fmt.Printf("  %s\n", meetURL)
	fmt.Println()
	fmt.Println("Starting panel...")

	// Create and join panelist agents
	var lkAgents []*livekitagent.Agent
	for i, panelist := range panelists {
		ag := createPanelistAgent(ctx, serverURL, apiKey, apiSecret, roomName, panelist, i)
		lkAgents = append(lkAgents, ag)
	}

	// Create moderator agent
	modAgent := createModeratorAgent(ctx, serverURL, apiKey, apiSecret, roomName, moderator)
	lkAgents = append(lkAgents, modAgent)

	// Give agents time to stabilize
	time.Sleep(time.Second)

	// Start recording if enabled
	var recorder *RecordingManager
	if os.Getenv("PANEL_RECORD") == "true" {
		recordingSettings := &panel.RecordingSettings{
			Enabled:  true,
			Format:   getEnvOrDefault("PANEL_RECORD_FORMAT", "mp4"),
			Layout:   getEnvOrDefault("PANEL_RECORD_LAYOUT", "speaker"),
			FilePath: os.Getenv("PANEL_RECORD_PATH"),
			S3Bucket: os.Getenv("PANEL_RECORD_S3_BUCKET"),
			S3Region: os.Getenv("PANEL_RECORD_S3_REGION"),
		}
		recorder = NewRecordingManager(serverURL, apiKey, apiSecret, roomName, recordingSettings)
		if err := recorder.Start(ctx); err != nil {
			log.Printf("Warning: Could not start recording: %v", err)
		} else {
			fmt.Printf("Recording:  ENABLED (egress: %s)\n", recorder.EgressID())
		}
	}

	// Run the automated panel discussion
	if err := coordinator.RunAutoPanel(ctx, moderator); err != nil {
		if ctx.Err() == nil {
			log.Printf("Error running auto panel: %v", err)
		}
	}

	// Stop recording
	if recorder != nil && recorder.IsRecording() {
		if err := recorder.Stop(context.Background()); err != nil {
			log.Printf("Warning: Could not stop recording: %v", err)
		}
	}

	cleanupAgents(lkAgents)
}

// createPanelistAgent creates and joins a LiveKit agent for a panelist.
func createPanelistAgent(ctx context.Context, serverURL, apiKey, apiSecret, roomName string, panelist *Panelist, idx int) *livekitagent.Agent {
	agentOpts := livekitagent.Options{
		APIKey:        apiKey,
		APISecret:     apiSecret,
		ServerURL:     serverURL,
		Identity:      fmt.Sprintf("panelist-%d", idx+1),
		Name:          panelist.Name,
		AutoSubscribe: false,
		Audio: livekitagent.AudioConfig{
			SampleRate: 48000,
			Channels:   1,
			TrackName:  fmt.Sprintf("%s-audio", strings.ToLower(panelist.Name)),
		},
	}

	// Configure HeyGen avatar if avatar ID is provided
	if panelist.AvatarID != "" {
		heygenKey := os.Getenv("HEYGEN_API_KEY")
		if heygenKey == "" {
			log.Fatalf("HEYGEN_API_KEY required for panelist %s avatar", panelist.Name)
		}
		agentOpts.Avatar = &livekitagent.AvatarConfig{
			Provider: "heygen",
			HeyGen: &livekitagent.HeyGenAvatarConfig{
				APIKey:       heygenKey,
				AvatarID:     panelist.AvatarID,
				Sandbox:      os.Getenv("HEYGEN_SANDBOX") == "true",
				VideoQuality: getEnvOrDefault("HEYGEN_VIDEO_QUALITY", "high"),
			},
		}
	}

	ag, err := livekitagent.New(agentOpts)
	if err != nil {
		log.Fatalf("Failed to create agent for %s: %v", panelist.Name, err)
	}

	if err := ag.Join(ctx, roomName); err != nil {
		log.Fatalf("Failed to join room for %s: %v", panelist.Name, err)
	}

	if panelist.AvatarID != "" {
		fmt.Printf("  [+] %s joined (avatar: %s)\n", panelist.Name, panelist.AvatarID)
	} else {
		fmt.Printf("  [+] %s joined\n", panelist.Name)
	}

	audioWriter, err := ag.StartAudio(ctx)
	if err != nil {
		log.Fatalf("Failed to start audio for %s: %v", panelist.Name, err)
	}
	panelist.SetAgent(ag)
	panelist.SetAudioWriter(audioWriter)

	return ag
}

// createModeratorAgent creates and joins a LiveKit agent for the AI moderator.
func createModeratorAgent(ctx context.Context, serverURL, apiKey, apiSecret, roomName string, moderator *Moderator) *livekitagent.Agent {
	agentOpts := livekitagent.Options{
		APIKey:        apiKey,
		APISecret:     apiSecret,
		ServerURL:     serverURL,
		Identity:      "moderator",
		Name:          moderator.Name,
		AutoSubscribe: false,
		Audio: livekitagent.AudioConfig{
			SampleRate: 48000,
			Channels:   1,
			TrackName:  "moderator-audio",
		},
	}

	// Configure HeyGen avatar if avatar ID is provided
	if moderator.AvatarID != "" {
		heygenKey := os.Getenv("HEYGEN_API_KEY")
		if heygenKey == "" {
			log.Fatal("HEYGEN_API_KEY required for moderator avatar")
		}
		agentOpts.Avatar = &livekitagent.AvatarConfig{
			Provider: "heygen",
			HeyGen: &livekitagent.HeyGenAvatarConfig{
				APIKey:       heygenKey,
				AvatarID:     moderator.AvatarID,
				Sandbox:      os.Getenv("HEYGEN_SANDBOX") == "true",
				VideoQuality: getEnvOrDefault("HEYGEN_VIDEO_QUALITY", "high"),
			},
		}
	}

	ag, err := livekitagent.New(agentOpts)
	if err != nil {
		log.Fatalf("Failed to create moderator agent: %v", err)
	}

	if err := ag.Join(ctx, roomName); err != nil {
		log.Fatalf("Failed to join room for moderator: %v", err)
	}

	if moderator.AvatarID != "" {
		fmt.Printf("  [+] %s (moderator) joined (avatar: %s)\n", moderator.Name, moderator.AvatarID)
	} else {
		fmt.Printf("  [+] %s (moderator) joined\n", moderator.Name)
	}

	audioWriter, err := ag.StartAudio(ctx)
	if err != nil {
		log.Fatalf("Failed to start audio for moderator: %v", err)
	}
	moderator.SetAgent(ag)
	moderator.SetAudioWriter(audioWriter)

	return ag
}

// createListenerAgent creates a STT listener agent for human mode.
func createListenerAgent(ctx context.Context, serverURL, apiKey, apiSecret, roomName string) *livekitagent.Agent {
	listenerOpts := livekitagent.Options{
		APIKey:        apiKey,
		APISecret:     apiSecret,
		ServerURL:     serverURL,
		Identity:      "stt-listener",
		Name:          "STT Listener",
		AutoSubscribe: true,
		Audio: livekitagent.AudioConfig{
			SampleRate: 48000,
			Channels:   1,
			TrackName:  "listener-audio",
		},
	}

	listenerAgent, err := livekitagent.New(listenerOpts)
	if err != nil {
		log.Fatalf("Failed to create listener agent: %v", err)
	}

	if err := listenerAgent.Join(ctx, roomName); err != nil {
		log.Fatalf("Failed to join room for listener: %v", err)
	}
	fmt.Println("  [+] STT Listener joined")

	return listenerAgent
}

// cleanupAgents leaves the room for all agents.
func cleanupAgents(agents []*livekitagent.Agent) {
	for _, ag := range agents {
		if err := ag.Leave(context.Background()); err != nil {
			log.Printf("Error leaving room: %v", err)
		}
	}
}

// AudioProcessor handles incoming audio and triggers discussion rounds.
type AudioProcessor struct {
	sttProvider  stt.Provider
	coordinator  *Coordinator
	sampleRate   int
	processing   bool
	processingMu sync.Mutex
}

// ProcessAudio processes incoming audio frames from the moderator.
func (p *AudioProcessor) ProcessAudio(ctx context.Context, audioCh <-chan livekitagent.AudioFrame) {
	var audioBuffer []byte
	var lastAudioTime time.Time
	silenceThreshold := 800 * time.Millisecond

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

			p.processingMu.Lock()
			processing := p.processing
			p.processingMu.Unlock()
			if processing {
				continue
			}

			audioBuffer = append(audioBuffer, frame.Data...)
			lastAudioTime = time.Now()

		case <-ticker.C:
			if len(audioBuffer) > 0 && time.Since(lastAudioTime) > silenceThreshold {
				audioCopy := make([]byte, len(audioBuffer))
				copy(audioCopy, audioBuffer)
				audioBuffer = nil

				go func(audio []byte) {
					p.processingMu.Lock()
					p.processing = true
					p.processingMu.Unlock()

					defer func() {
						p.processingMu.Lock()
						p.processing = false
						p.processingMu.Unlock()
					}()

					if err := p.processUtterance(ctx, audio); err != nil {
						log.Printf("Error processing: %v", err)
					}
				}(audioCopy)
			}
		}
	}
}

// processUtterance transcribes audio and triggers a discussion round.
func (p *AudioProcessor) processUtterance(ctx context.Context, audio []byte) error {
	if len(audio) < 24000 {
		return nil
	}

	wavData := pcmToWav(audio, p.sampleRate, 1)
	result, err := p.sttProvider.Transcribe(ctx, wavData, stt.TranscriptionConfig{
		Language:   "en",
		SampleRate: p.sampleRate,
		Encoding:   "linear16",
	})
	if err != nil {
		return fmt.Errorf("STT: %w", err)
	}

	text := strings.TrimSpace(result.Text)
	if text == "" {
		return nil
	}

	return p.coordinator.OnModeratorSpeech(ctx, text)
}

// createPanelists creates panelists from configuration.
func createPanelists(count int, ttsProv tts.Provider, llmClient LLMClient) []*Panelist {
	defaults := DefaultPanelists()
	panelists := make([]*Panelist, 0, count)

	for i := 0; i < count; i++ {
		idx := i + 1

		name := os.Getenv(fmt.Sprintf("PANELIST_%d_NAME", idx))
		voice := os.Getenv(fmt.Sprintf("PANELIST_%d_VOICE", idx))
		personality := os.Getenv(fmt.Sprintf("PANELIST_%d_PERSONALITY", idx))
		avatarID := os.Getenv(fmt.Sprintf("PANELIST_%d_AVATAR_ID", idx))

		if i < len(defaults) {
			if name == "" {
				name = defaults[i].Name
			}
			if voice == "" {
				voice = defaults[i].Voice
			}
			if personality == "" {
				personality = defaults[i].Personality
			}
		}

		if name == "" {
			name = fmt.Sprintf("Panelist %d", idx)
		}
		if voice == "" {
			voice = "alloy"
		}
		if personality == "" {
			personality = "A thoughtful participant who shares interesting perspectives."
		}

		panelists = append(panelists, NewPanelist(PanelistConfig{
			Name:        name,
			Personality: personality,
			Voice:       voice,
			AvatarID:    avatarID,
			TTSProvider: ttsProv,
			LLMClient:   llmClient,
		}))
	}

	return panelists
}

// Helper functions

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
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
