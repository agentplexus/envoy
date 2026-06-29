// Package strutil provides string utility functions.
package strutil

import (
	"unicode/utf16"
	"unicode/utf8"
)

// TruncateUTF16 truncates a string to the specified number of UTF-16 code units.
// This is important for APIs like Telegram, Discord, and Slack that count
// characters in UTF-16 code units rather than bytes or runes.
//
// The function ensures the string is truncated at a valid UTF-16 boundary,
// avoiding splitting surrogate pairs (emojis, CJK characters, etc.).
func TruncateUTF16(s string, maxCodeUnits int) string {
	if maxCodeUnits <= 0 {
		return ""
	}

	// Fast path: if string is ASCII-only and short enough, return as-is
	if len(s) <= maxCodeUnits && isASCII(s) {
		return s
	}

	codeUnits := 0
	lastValid := 0

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8, skip byte
			i++
			continue
		}

		// Calculate UTF-16 code units for this rune
		units := 1
		if r > 0xFFFF {
			units = 2 // Surrogate pair
		}

		if codeUnits+units > maxCodeUnits {
			break
		}

		codeUnits += units
		i += size
		lastValid = i
	}

	return s[:lastValid]
}

// TruncateUTF16WithEllipsis truncates a string and adds an ellipsis if truncated.
// The ellipsis counts as 1 UTF-16 code unit.
func TruncateUTF16WithEllipsis(s string, maxCodeUnits int) string {
	if maxCodeUnits <= 0 {
		return ""
	}

	// Check if truncation is needed
	if UTF16Len(s) <= maxCodeUnits {
		return s
	}

	// Reserve space for ellipsis
	truncated := TruncateUTF16(s, maxCodeUnits-1)
	return truncated + "…"
}

// UTF16Len returns the number of UTF-16 code units in a string.
// This is useful for checking message lengths against platform limits.
func UTF16Len(s string) int {
	codeUnits := 0
	for _, r := range s {
		if r > 0xFFFF {
			codeUnits += 2 // Surrogate pair
		} else {
			codeUnits++
		}
	}
	return codeUnits
}

// SplitUTF16 splits a string into chunks of at most maxCodeUnits UTF-16 code units.
// Useful for splitting long messages for APIs with character limits.
func SplitUTF16(s string, maxCodeUnits int) []string {
	if maxCodeUnits <= 0 {
		return nil
	}

	if UTF16Len(s) <= maxCodeUnits {
		return []string{s}
	}

	var chunks []string
	remaining := s

	for len(remaining) > 0 {
		chunk := TruncateUTF16(remaining, maxCodeUnits)
		if chunk == "" {
			// Edge case: first character is too wide (shouldn't happen with valid input)
			break
		}
		chunks = append(chunks, chunk)
		remaining = remaining[len(chunk):]
	}

	return chunks
}

// isASCII checks if a string contains only ASCII characters.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// EncodeUTF16 encodes a string to UTF-16.
// Returns the UTF-16 code units.
func EncodeUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// DecodeUTF16 decodes UTF-16 code units to a string.
func DecodeUTF16(u []uint16) string {
	return string(utf16.Decode(u))
}
