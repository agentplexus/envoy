package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"

	"github.com/plexusone/omniagent/cmd/livekit-agent-panel/panel"
)

// RecordingManager handles panel recording via LiveKit Egress.
type RecordingManager struct {
	egress   *lksdk.EgressClient
	roomName string
	settings *panel.RecordingSettings

	egressID  string
	startedAt time.Time

	mu sync.Mutex
}

// NewRecordingManager creates a new recording manager.
func NewRecordingManager(serverURL, apiKey, apiSecret, roomName string, settings *panel.RecordingSettings) *RecordingManager {
	return &RecordingManager{
		egress:   lksdk.NewEgressClient(serverURL, apiKey, apiSecret),
		roomName: roomName,
		settings: settings,
	}
}

// Start begins recording the panel.
func (m *RecordingManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.settings == nil || !m.settings.Enabled {
		return nil // Recording not enabled
	}

	if m.egressID != "" {
		return fmt.Errorf("recording already started")
	}

	// Determine layout
	layout := "speaker"
	if m.settings.Layout != "" {
		layout = m.settings.Layout
	}

	// Determine format (MP4 is the primary supported format)
	fileType := livekit.EncodedFileType_MP4
	if m.settings.Format == "ogg" {
		fileType = livekit.EncodedFileType_OGG
	}

	// Build file output
	var output livekit.EncodedFileOutput
	output.FileType = fileType

	if m.settings.FilePath != "" {
		output.Filepath = m.settings.FilePath
	} else if m.settings.S3Bucket != "" {
		output.Output = &livekit.EncodedFileOutput_S3{
			S3: &livekit.S3Upload{
				Bucket: m.settings.S3Bucket,
				Region: m.settings.S3Region,
			},
		}
	} else {
		// Default to local file with timestamp
		output.Filepath = fmt.Sprintf("panel-recording-%s-%d.mp4", m.roomName, time.Now().Unix())
	}

	req := &livekit.RoomCompositeEgressRequest{
		RoomName: m.roomName,
		Layout:   layout,
		Output: &livekit.RoomCompositeEgressRequest_File{
			File: &output,
		},
	}

	log.Printf("[Recording] Starting room composite egress for %s...", m.roomName)

	info, err := m.egress.StartRoomCompositeEgress(ctx, req)
	if err != nil {
		return fmt.Errorf("start egress: %w", err)
	}

	m.egressID = info.EgressId
	m.startedAt = time.Now()

	log.Printf("[Recording] Started egress %s", m.egressID)
	return nil
}

// Stop ends the recording.
func (m *RecordingManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.egressID == "" {
		return nil // Not recording
	}

	log.Printf("[Recording] Stopping egress %s...", m.egressID)

	_, err := m.egress.StopEgress(ctx, &livekit.StopEgressRequest{
		EgressId: m.egressID,
	})
	if err != nil {
		return fmt.Errorf("stop egress: %w", err)
	}

	duration := time.Since(m.startedAt)
	log.Printf("[Recording] Stopped. Duration: %s", duration.Round(time.Second))

	m.egressID = ""
	return nil
}

// IsRecording returns true if recording is active.
func (m *RecordingManager) IsRecording() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.egressID != ""
}

// EgressID returns the current egress ID.
func (m *RecordingManager) EgressID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.egressID
}

// Duration returns the recording duration so far.
func (m *RecordingManager) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.egressID == "" {
		return 0
	}
	return time.Since(m.startedAt)
}
