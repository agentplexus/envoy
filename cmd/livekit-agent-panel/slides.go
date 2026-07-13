package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/plexusone/omniagent/cmd/livekit-agent-panel/panel"

	// Import image formats
	_ "image/gif"
)

// SlideManager manages slide sharing for a panel.
type SlideManager struct {
	room        *lksdk.Room
	slides      map[string]*LoadedSlide // slideID -> slide
	panelistMap map[string][]string     // panelistName -> slideIDs

	currentSlide   string // currently displayed slide ID
	currentSharer  string // name of current sharer
	screenTrack    *lksdk.LocalSampleTrack
	screenTrackPub *lksdk.LocalTrackPublication
	frameWriter    *slideFrameWriter
	httpClient     *http.Client

	mu sync.Mutex
}

// LoadedSlide contains a slide ready for display.
type LoadedSlide struct {
	ID        string
	Title     string
	ImageData []byte
	Width     int
	Height    int
	Keywords  []string
}

// slideFrameWriter writes image frames to a video track.
type slideFrameWriter struct {
	track     *lksdk.LocalSampleTrack
	frameRate int
	frameDur  time.Duration
	current   []byte

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewSlideManager creates a new slide manager.
func NewSlideManager(room *lksdk.Room) *SlideManager {
	return &SlideManager{
		room:        room,
		slides:      make(map[string]*LoadedSlide),
		panelistMap: make(map[string][]string),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// LoadSlides loads slides for all panelists from a schedule.
func (m *SlideManager) LoadSlides(ctx context.Context, panelists []panel.PanelistSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range panelists {
		for _, slide := range p.Slides {
			loaded, err := m.loadSlide(ctx, slide)
			if err != nil {
				return fmt.Errorf("load slide %s for %s: %w", slide.ID, p.Name, err)
			}
			m.slides[slide.ID] = loaded
			m.panelistMap[p.Name] = append(m.panelistMap[p.Name], slide.ID)
		}
	}

	return nil
}

// loadSlide loads a single slide from path or URL.
func (m *SlideManager) loadSlide(ctx context.Context, slide panel.Slide) (*LoadedSlide, error) {
	var data []byte
	var err error

	if slide.ImagePath != "" {
		data, err = os.ReadFile(slide.ImagePath)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
	} else if slide.ImageURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", slide.ImageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		resp, err := m.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch URL: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no image_path or image_url specified")
	}

	// Decode image to get dimensions
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Re-encode as JPEG for consistent format
	var buf bytes.Buffer
	if format == "png" {
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	} else {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
	}

	return &LoadedSlide{
		ID:        slide.ID,
		Title:     slide.Title,
		ImageData: buf.Bytes(),
		Width:     width,
		Height:    height,
		Keywords:  slide.Keywords,
	}, nil
}

// StartScreenShare initializes the screen share track.
func (m *SlideManager) StartScreenShare(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.screenTrack != nil {
		return nil // Already started
	}

	// Create video track for screen sharing
	track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeVP8,
		ClockRate: 90000,
	})
	if err != nil {
		return fmt.Errorf("create track: %w", err)
	}

	m.screenTrack = track

	// Create frame writer
	fwCtx, cancel := context.WithCancel(ctx)
	m.frameWriter = &slideFrameWriter{
		track:     track,
		frameRate: 5, // Low frame rate for static slides
		frameDur:  time.Second / 5,
		ctx:       fwCtx,
		cancel:    cancel,
	}

	// Start frame loop (but don't publish yet)
	go m.frameWriter.loop()

	return nil
}

// ShowSlide displays a slide and publishes the screen share track.
func (m *SlideManager) ShowSlide(_ context.Context, slideID, sharerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	slide, ok := m.slides[slideID]
	if !ok {
		return fmt.Errorf("slide %q not found", slideID)
	}

	// Update current slide
	m.currentSlide = slideID
	m.currentSharer = sharerName
	m.frameWriter.setImage(slide.ImageData)

	// Publish track if not already published
	if m.screenTrackPub == nil {
		pub, err := m.room.LocalParticipant.PublishTrack(m.screenTrack, &lksdk.TrackPublicationOptions{
			Name:   "slides",
			Source: livekit.TrackSource_SCREEN_SHARE,
		})
		if err != nil {
			return fmt.Errorf("publish track: %w", err)
		}
		m.screenTrackPub = pub
	}

	return nil
}

// HideSlide stops showing the current slide.
func (m *SlideManager) HideSlide(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.screenTrackPub != nil {
		if err := m.room.LocalParticipant.UnpublishTrack(m.screenTrackPub.SID()); err != nil {
			return fmt.Errorf("unpublish track: %w", err)
		}
		m.screenTrackPub = nil
	}

	m.currentSlide = ""
	m.currentSharer = ""
	m.frameWriter.setImage(nil)

	return nil
}

// FindSlideByKeyword finds a slide matching keywords in the given text.
func (m *SlideManager) FindSlideByKeyword(panelistName, text string) *LoadedSlide {
	m.mu.Lock()
	defer m.mu.Unlock()

	slideIDs, ok := m.panelistMap[panelistName]
	if !ok {
		return nil
	}

	textLower := strings.ToLower(text)
	for _, id := range slideIDs {
		slide := m.slides[id]
		for _, kw := range slide.Keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				return slide
			}
		}
	}

	return nil
}

// GetPanelistSlides returns all slides for a panelist.
func (m *SlideManager) GetPanelistSlides(panelistName string) []*LoadedSlide {
	m.mu.Lock()
	defer m.mu.Unlock()

	var slides []*LoadedSlide
	for _, id := range m.panelistMap[panelistName] {
		if slide, ok := m.slides[id]; ok {
			slides = append(slides, slide)
		}
	}
	return slides
}

// CurrentSlide returns the currently displayed slide ID.
func (m *SlideManager) CurrentSlide() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentSlide
}

// Close cleans up resources.
func (m *SlideManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.frameWriter != nil {
		m.frameWriter.cancel()
	}

	return nil
}

// slideFrameWriter methods

func (w *slideFrameWriter) setImage(data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.current = data
}

func (w *slideFrameWriter) loop() {
	ticker := time.NewTicker(w.frameDur)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			data := w.current
			w.mu.Unlock()

			if data != nil {
				sample := media.Sample{
					Data:     data,
					Duration: w.frameDur,
				}
				if err := w.track.WriteSample(sample, nil); err != nil {
					// Log but continue
					continue
				}
			}
		}
	}
}
