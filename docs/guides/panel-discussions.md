# Panel Discussions

Run multi-agent AI panel discussions where 2-4 AI panelists discuss a topic with a human moderator.

## Overview

The `livekit-agent-panel` command creates a LiveKit room where:

- A **human moderator** asks questions via voice
- **2-4 AI panelists** take turns responding
- Each panelist has a unique **voice** and **personality**
- Panelists **build on each other's points** using a shared transcript

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

export PANELIST_2_NAME="Marcus"
export PANELIST_2_VOICE="echo"
export PANELIST_2_PERSONALITY="A patient advocate concerned about AI bias"
```

## How It Works

### Flow

1. Human joins room as moderator
2. Panelists introduce themselves (brief round-robin)
3. Moderator speaks → STT transcribes the question
4. Coordinator selects speaking order (rotates each round)
5. Each panelist:
   - Sees full conversation transcript
   - Generates response based on personality
   - Speaks via TTS with their unique voice
6. Wait for next moderator question
7. Repeat

### Turn-Taking

- All panelists respond to each moderator question
- Speaking order rotates to vary who speaks first
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
├── panelist.go       # Panelist definition (voice, personality)
├── coordinator.go    # Turn-taking orchestration
├── moderator.go      # Human speech handling
├── transcript.go     # Shared conversation context
├── speaker.go        # TTS output management
└── llm.go            # LLM response generation
```

## Tips

- **Keep responses short**: Panelists are instructed to give 2-4 sentences
- **Vary the topic**: Different topics bring out different panelist dynamics
- **Custom personalities**: Create domain-specific experts for focused discussions
- **Moderator guidance**: Ask follow-up questions to steer the conversation

## Limitations (V1)

- No avatar support (audio-only)
- No realtime voice mode (uses STT → LLM → TTS pipeline)
- No audience/chat participation

These features are planned for V2.

## Related

- [Voice Integration Guide](voice.md)
- [LiveKit Agents in README](https://github.com/plexusone/omniagent#livekit-voice-agents)
- [omni-livekit Documentation](https://github.com/plexusone/omni-livekit)
