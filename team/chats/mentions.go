package chats

import "strings"

// mentionsAgent reports whether content @-mentions the given agent slug — the
// group-chat trigger for an agent turn (RMI-113). A mention is an "@" followed
// by exactly the slug, where the "@" starts a token (preceded by the string
// start or a non-slug character) and the slug is not merely a prefix of a
// longer token. Matching is case-insensitive: slugs are stored citext
// (case-insensitive unique), so "@Slug" mentions "slug".
//
// Examples (slug "helper"): "@helper hi" and "hey @helper!" match; "@helperbot"
// and "email@helper" do not.
func mentionsAgent(content, slug string) bool {
	if slug == "" {
		return false
	}
	content = strings.ToLower(content)
	slug = strings.ToLower(slug)
	for i := 0; i < len(content); i++ {
		if content[i] != '@' {
			continue
		}
		// The "@" must begin a token: the preceding char cannot be part of a
		// slug (so "email@helper" is not a mention).
		if i > 0 && isSlugChar(content[i-1]) {
			continue
		}
		j := i + 1
		for j < len(content) && isSlugChar(content[j]) {
			j++
		}
		if content[i+1:j] == slug {
			return true
		}
	}
	return false
}

// isSlugChar reports whether b is a character an agent slug may contain
// ("^[a-z0-9][a-z0-9_-]{2,31}$"). Used to find token boundaries around a
// mention so a slug matches only as a whole token.
func isSlugChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b == '_'
}
