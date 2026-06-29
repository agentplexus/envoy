package strutil

import (
	"testing"
)

func TestTruncateUTF16(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		maxCodeUnits int
		want         string
	}{
		{
			name:         "empty string",
			input:        "",
			maxCodeUnits: 10,
			want:         "",
		},
		{
			name:         "ASCII under limit",
			input:        "hello",
			maxCodeUnits: 10,
			want:         "hello",
		},
		{
			name:         "ASCII at limit",
			input:        "hello",
			maxCodeUnits: 5,
			want:         "hello",
		},
		{
			name:         "ASCII over limit",
			input:        "hello world",
			maxCodeUnits: 5,
			want:         "hello",
		},
		{
			name:         "emoji (surrogate pair)",
			input:        "hi 👋",
			maxCodeUnits: 5,
			want:         "hi 👋", // 👋 is 2 UTF-16 code units
		},
		{
			name:         "emoji truncated",
			input:        "hi 👋 there",
			maxCodeUnits: 5,
			want:         "hi 👋",
		},
		{
			name:         "emoji at boundary",
			input:        "hi 👋",
			maxCodeUnits: 4,
			want:         "hi ", // Can't fit emoji (2 units) in remaining 1 slot
		},
		{
			name:         "multiple emojis",
			input:        "🎉🎊🎈",
			maxCodeUnits: 4,
			want:         "🎉🎊",
		},
		{
			name:         "Chinese characters",
			input:        "你好世界",
			maxCodeUnits: 3,
			want:         "你好世", // Each Chinese char is 1 UTF-16 code unit (in BMP)
		},
		{
			name:         "mixed content",
			input:        "Hello 世界 👋",
			maxCodeUnits: 10,
			want:         "Hello 世界 ", // 5 + 1 + 2 + 1 = 9, + emoji would be 11, so stop at space
		},
		{
			name:         "zero limit",
			input:        "hello",
			maxCodeUnits: 0,
			want:         "",
		},
		{
			name:         "negative limit",
			input:        "hello",
			maxCodeUnits: -1,
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateUTF16(tt.input, tt.maxCodeUnits)
			if got != tt.want {
				t.Errorf("TruncateUTF16(%q, %d) = %q, want %q",
					tt.input, tt.maxCodeUnits, got, tt.want)
			}
		})
	}
}

func TestTruncateUTF16WithEllipsis(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		maxCodeUnits int
		want         string
	}{
		{
			name:         "no truncation needed",
			input:        "hello",
			maxCodeUnits: 10,
			want:         "hello",
		},
		{
			name:         "truncation with ellipsis",
			input:        "hello world",
			maxCodeUnits: 6,
			want:         "hello…",
		},
		{
			name:         "emoji preserved",
			input:        "hi 👋 there",
			maxCodeUnits: 6,
			want:         "hi 👋…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateUTF16WithEllipsis(tt.input, tt.maxCodeUnits)
			if got != tt.want {
				t.Errorf("TruncateUTF16WithEllipsis(%q, %d) = %q, want %q",
					tt.input, tt.maxCodeUnits, got, tt.want)
			}
		})
	}
}

func TestUTF16Len(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"ASCII", "hello", 5},
		{"emoji", "👋", 2},    // Surrogate pair
		{"Chinese", "你好", 2}, // BMP characters
		{"mixed", "hi 👋", 5}, // 2 + 1 + 2 = 5
		{"flags", "🇺🇸", 4},   // Two regional indicators (4 UTF-16 code units)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UTF16Len(tt.input)
			if got != tt.want {
				t.Errorf("UTF16Len(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitUTF16(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		maxCodeUnits int
		want         []string
	}{
		{
			name:         "no split needed",
			input:        "hello",
			maxCodeUnits: 10,
			want:         []string{"hello"},
		},
		{
			name:         "simple split",
			input:        "hello world",
			maxCodeUnits: 6,
			want:         []string{"hello ", "world"},
		},
		{
			name:         "emoji boundary",
			input:        "a👋b👋c",
			maxCodeUnits: 4,
			want:         []string{"a👋b", "👋c"},
		},
		{
			name:         "zero limit",
			input:        "hello",
			maxCodeUnits: 0,
			want:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitUTF16(tt.input, tt.maxCodeUnits)
			if len(got) != len(tt.want) {
				t.Errorf("SplitUTF16(%q, %d) = %v, want %v",
					tt.input, tt.maxCodeUnits, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitUTF16(%q, %d)[%d] = %q, want %q",
						tt.input, tt.maxCodeUnits, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestEncodeDecodeUTF16(t *testing.T) {
	tests := []string{
		"hello",
		"你好",
		"👋🎉",
		"Hello 世界 👋",
	}

	for _, s := range tests {
		encoded := EncodeUTF16(s)
		decoded := DecodeUTF16(encoded)
		if decoded != s {
			t.Errorf("roundtrip failed: %q -> %v -> %q", s, encoded, decoded)
		}
	}
}

func BenchmarkTruncateUTF16(b *testing.B) {
	input := "Hello 世界! This is a test message with some emojis 👋🎉🎊 and more text."
	for i := 0; i < b.N; i++ {
		TruncateUTF16(input, 30)
	}
}

func BenchmarkUTF16Len(b *testing.B) {
	input := "Hello 世界! This is a test message with some emojis 👋🎉🎊 and more text."
	for i := 0; i < b.N; i++ {
		UTF16Len(input)
	}
}
