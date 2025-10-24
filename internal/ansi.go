package ghprcomments

import "strings"

const (
	ansiReset     = "\u001b[0m"
	ansiDim       = "\u001b[2m"
	ansiCyan      = "\u001b[36m"
	ansiYellow    = "\u001b[33m"
	ansiMagenta   = "\u001b[35m"
	ansiUnderline = "\u001b[4m"
)

func applyStyle(enabled bool, code, text string) string {
	if !enabled || text == "" {
		return text
	}
	var b strings.Builder
	b.Grow(len(code) + len(text) + len(ansiReset))
	b.WriteString(code)
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}

func applyStyles(enabled bool, text string, codes ...string) string {
	if !enabled || text == "" {
		return text
	}
	var b strings.Builder
	for _, code := range codes {
		b.WriteString(code)
	}
	b.WriteString(text)
	b.WriteString(ansiReset)
	return b.String()
}
