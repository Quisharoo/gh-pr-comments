package ghprcomments

import "strings"

// OSC-8 hyperlink escape codes for terminal hyperlink support
// These are kept as raw constants since lipgloss doesn't directly support OSC-8 hyperlinks
const (
	oscHyperlinkPrefix  = "\u001b]8;;"
	oscHyperlinkClosure = "\u001b]8;;\u0007"
)

// applyHyperlink wraps text in OSC-8 hyperlink sequences for terminal support.
// This is kept separate from lipgloss styles since OSC-8 is a terminal-specific feature.
func applyHyperlink(enabled bool, url, text string) string {
	if !enabled || url == "" || text == "" {
		return text
	}
	var b strings.Builder
	b.Grow(len(oscHyperlinkPrefix) + len(url) + len(text) + len(oscHyperlinkClosure) + 1)
	b.WriteString(oscHyperlinkPrefix)
	b.WriteString(url)
	b.WriteByte('\a')
	b.WriteString(text)
	b.WriteString(oscHyperlinkClosure)
	return b.String()
}
