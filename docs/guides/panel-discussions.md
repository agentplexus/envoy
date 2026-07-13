# Panel Discussions

Run multi-agent AI panel discussions where 2-4 AI panelists discuss a topic with a human or AI moderator.

## Overview

The `livekit-agent-panel` command creates a LiveKit room where:

- A **moderator** (human or AI) guides the discussion
- **2-4 AI panelists** take turns responding
- Each panelist has a unique **voice**, **personality**, and optional **HeyGen avatar**
- Panelists **build on each other's points** using a shared transcript
- **Slides** can be displayed automatically based on keywords
- **Recording** captures the full session to local storage or S3

## Quick Start

```bash
# Required
export LIVEKIT_URL="wss://your-livekit-server"
export LIVEKIT_API_KEY="your-api-key"
export LIVEKIT_API_SECRET="your-api-secret"
export ANTHROPIC_API_KEY="your-anthropic-key"  # For LLM
export DEEPGRAM_API_KEY="your-deepgram-key"    # For STT
export OPENAI_API_KEY="your-openai-key"        # For TTS

# Panel configuration
export PANEL_SIZE=3
export PANEL_TOPIC="The Future of AI Agents"

# Run
go run -tags opus ./cmd/livekit-agent-panel
```

Join the meeting URL printed in the console to moderate the discussion.

## Configuration

### Panel Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PANEL_SIZE` | `3` | Number of panelists (2-4) |
| `PANEL_TOPIC` | `"AI and Technology"` | Discussion topic |

### Default Panelists

If no custom panelists are configured, these defaults are used:

| Name | Personality | Voice |
|------|-------------|-------|
| Alex | Optimistic tech enthusiast, cites benefits | `alloy` |
| Jordan | Pragmatic skeptic, asks hard questions | `echo` |
| Morgan | Academic expert, provides depth/context | `onyx` |
| Casey | Creative thinker, offers novel perspectives | `nova` |

### Custom Panelists

Override default panelists with environment variables:

```bash
export PANELIST_1_NAME="Dr. Sarah"
export PANELIST_1_VOICE="nova"
export PANELIST_1_PERSONALITY="A physician specializing in AI diagnostics"
export PANELIST_1_AVATAR_ID="heygen-avatar-id"  # Optional HeyGen avatar

export PANELIST_2_NAME="Marcus"
export PANELIST_2_VOICE="echo"
export PANELIST_2_PERSONALITY="A patient advocate concerned about AI bias"
export PANELIST_2_AVATAR_ID="heygen-avatar-id"
```

## HeyGen Avatar Integration

Panel agents support HeyGen avatars for realistic AI panelists with lip-synchronized speech.

### Configuration

```bash
# Global HeyGen settings
export HEYGEN_API_KEY="your-api-key"
export HEYGEN_SANDBOX=true  # Use sandbox for testing
export HEYGEN_VIDEO_QUALITY=high

# Per-participant avatar IDs
export MODERATOR_AVATAR_ID="avatar-id-1"
export PANELIST_1_AVATAR_ID="avatar-id-2"
export PANELIST_2_AVATAR_ID="avatar-id-3"
```

### Features

- Audio routing to avatar at 24kHz for lip-sync
- Automatic fallback to LiveKit when avatar not configured
- Per-panelist avatar configuration via environment variables

## JSON Schedule Format

Define panel discussions using JSON-based schedules for reproducible, automated panels.

### Schedule File

```json
{
  "title": "AI Panel Discussion",
  "moderator": {
    "name": "Alex",
    "personality": "Professional moderator",
    "avatarId": "heygen-avatar-id"
  },
  "panelists": [
    {
      "name": "Sarah",
      "personality": "Technical expert",
      "expertise": ["AI", "ML"],
      "avatarId": "heygen-avatar-id"
    }
  ],
  "segments": [
    {
      "type": "moderator_intro",
      "duration": "1m"
    },
    {
      "type": "panelist_backgrounds",
      "duration": "2m"
    },
    {
      "type": "discussion_round",
      "topic": "Future of AI",
      "duration": "5m",
      "responseOrder": "relevance"
    },
    {
      "type": "open_discussion",
      "duration": "3m"
    },
    {
      "type": "closing",
      "duration": "1m"
    }
  ],
  "settings": {
    "recording": {
      "enabled": true,
      "format": "mp4",
      "layout": "speaker"
    }
  }
}
```

### Running with Schedule

```bash
livekit-agent-panel --schedule ./panel-schedule.json
```

### Segment Types

| Type | Description |
|------|-------------|
| `moderator_intro` | Opening remarks by moderator |
| `panelist_backgrounds` | Brief introductions of panelists |
| `discussion_round` | Focused discussion on a topic |
| `open_discussion` | Free-form conversation |
| `closing` | Closing remarks and summary |

### Response Ordering

| Order | Description |
|-------|-------------|
| `relevance` | Most relevant panelist speaks first |
| `rotation` | Round-robin rotation |
| `random` | Random order each round |
| `fixed` | Fixed order as defined in schedule |

## JSON Output Format

Panel discussions generate structured output for post-processing.

### Output Format

```json
{
  "metadata": {
    "sessionId": "panel-123",
    "title": "AI Panel Discussion",
    "startTime": "2026-07-13T10:00:00Z",
    "endTime": "2026-07-13T10:30:00Z",
    "statistics": {
      "totalEntries": 45,
      "totalWords": 3500
    }
  },
  "participants": {
    "moderator": { "name": "Alex", "entries": 12, "words": 450 },
    "panelists": [
      { "name": "Sarah", "entries": 15, "words": 1200 }
    ]
  },
  "segments": [
    {
      "type": "discussion_round",
      "topic": "Future of AI",
      "entries": [
        {
          "speaker": "Sarah",
          "text": "...",
          "timestamp": "2026-07-13T10:05:00Z",
          "wordCount": 85
        }
      ]
    }
  ]
}
```

## Slide Sharing

Display slides during panel discussions with automatic keyword triggers.

### Slide Configuration

```json
{
  "slides": [
    {
      "id": "intro",
      "title": "Introduction",
      "imagePath": "/slides/intro.png",
      "keywords": ["welcome", "introduction"]
    },
    {
      "id": "stats",
      "title": "Key Statistics",
      "imagePath": "/slides/stats.png",
      "keywords": ["statistics", "numbers", "data"]
    }
  ]
}
```

### Features

- Load slides from local files or URLs
- Keyword-based automatic slide display
- Per-segment slide configuration
- Manual `ShowSlide`/`HideSlide` control

## Recording

Record panel sessions using LiveKit Egress with local or S3 output.

### Environment Variables

```bash
export PANEL_RECORD=true
export PANEL_RECORD_FORMAT=mp4        # mp4 or ogg
export PANEL_RECORD_LAYOUT=speaker    # speaker or grid
export PANEL_RECORD_PATH=/recordings  # Local path
export PANEL_RECORD_S3_BUCKET=my-bucket
export PANEL_RECORD_S3_REGION=us-east-1
```

### Features

- Speaker or grid layout options
- Duration tracking and status queries
- Automatic start/stop with panel lifecycle

## How It Works

### Flow

1. Load schedule (JSON or environment variables)
2. Panelists join with avatars (if configured)
3. Execute segments in order:
   - Moderator intro
   - Panelist backgrounds
   - Discussion rounds
   - Open discussion
   - Closing
4. Generate output JSON with statistics
5. Save recording to configured destination

### Turn-Taking

- All panelists respond to each moderator question
- Speaking order determined by segment configuration
- ~1-2 second pause between speakers for natural pacing
- Panelists are instructed to build on previous points, not repeat

### Shared Transcript

Each panelist sees the full conversation history:

```
You are Morgan, a panelist in a discussion about "AI in Healthcare".

Your personality: Academic expert, provides depth and context...

Current discussion transcript:
[Moderator]: What are the biggest challenges for AI in healthcare?
[Alex]: The potential is enormous - AI can analyze medical images...
[Jordan]: But we need to address the trust gap. Doctors are...

The moderator just said: "What about patient privacy?"

Respond as Morgan:
```

## Architecture

```
cmd/livekit-agent-panel/
├── main.go           # Entry point, LiveKit setup
├── panelist.go       # Panelist definition (voice, personality, avatar)
├── coordinator.go    # Turn-taking orchestration
├── moderator.go      # Human/AI speech handling
├── transcript.go     # Shared conversation context
├── speaker.go        # TTS output management
├── llm.go            # LLM response generation
├── schedule.go       # JSON schedule loading
├── output.go         # JSON output generation
├── slides.go         # Slide manager
└── recording.go      # Recording manager
```

## Tips

- **Keep responses short**: Panelists are instructed to give 2-4 sentences
- **Vary the topic**: Different topics bring out different panelist dynamics
- **Custom personalities**: Create domain-specific experts for focused discussions
- **Moderator guidance**: Ask follow-up questions to steer the conversation
- **Use schedules**: JSON schedules enable reproducible, automated panels
- **Enable recording**: Capture sessions for review and distribution

## Related

- [Voice Integration Guide](voice.md)
- [LiveKit Agents in README](https://github.com/plexusone/omniagent#livekit-voice-agents)
- [omni-livekit Documentation](https://github.com/plexusone/omni-livekit)
- [v0.15.0 Release Notes](../releases/v0.15.0.md)
