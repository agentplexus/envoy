package channels

import "testing"

func TestChannelTypes(t *testing.T) {
	types := []ChannelType{
		ChannelTypeDM,
		ChannelTypeGroup,
		ChannelTypeChannel,
		ChannelTypeThread,
	}

	seen := make(map[ChannelType]bool)
	for _, ct := range types {
		if seen[ct] {
			t.Errorf("Duplicate channel type: %s", ct)
		}
		seen[ct] = true
		if ct == "" {
			t.Error("Channel type should not be empty")
		}
	}
}

func TestMediaTypes(t *testing.T) {
	types := []MediaType{
		MediaTypeImage,
		MediaTypeVideo,
		MediaTypeAudio,
		MediaTypeDocument,
		MediaTypeSticker,
		MediaTypeVoice,
	}

	seen := make(map[MediaType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("Duplicate media type: %s", mt)
		}
		seen[mt] = true
	}
}

func TestMessageFormats(t *testing.T) {
	formats := []MessageFormat{
		MessageFormatPlain,
		MessageFormatMarkdown,
		MessageFormatHTML,
	}

	seen := make(map[MessageFormat]bool)
	for _, f := range formats {
		if seen[f] {
			t.Errorf("Duplicate format: %s", f)
		}
		seen[f] = true
	}
}

func TestEventTypes(t *testing.T) {
	types := []EventType{
		EventTypeMessageEdited,
		EventTypeMessageDeleted,
		EventTypeReaction,
		EventTypeTyping,
		EventTypePresence,
		EventTypeMemberJoined,
		EventTypeMemberLeft,
		EventTypeChannelCreated,
		EventTypeChannelDeleted,
	}

	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("Duplicate event type: %s", et)
		}
		seen[et] = true
	}
}
