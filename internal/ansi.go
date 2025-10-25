package ghprcomments

import "strings"

const (
	ansiReset           = "\u001b[0m"
	ansiDim             = "\u001b[2m"
	ansiFaint           = "\u001b[90m"
	ansiBrightCyan      = "\u001b[96m"
	ansiYellow          = "\u001b[33m"
	ansiMagenta         = "\u001b[35m"
	ansiBlue            = "\u001b[94m"
	ansiGreen           = "\u001b[92m"
	ansiUnderline       = "\u001b[4m"
	oscHyperlinkPrefix  = "\u001b]8;;"
	oscHyperlinkClosure = "\u001b]8;;\u0007"
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
