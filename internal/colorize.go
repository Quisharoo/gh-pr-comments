package ghprcomments

import (
	"regexp"
	"strings"
)

var (
	jsonTypePattern      = regexp.MustCompile(`("type":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonAuthorPattern    = regexp.MustCompile(`("author":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonCreatedAtPattern = regexp.MustCompile(`("created_at":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonBodyTextPattern  = regexp.MustCompile(`("body_text":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonPermalinkPattern = regexp.MustCompile(`("permalink":\s*)"((?:[^"\\]|\\.)*)"`)
)

// ColouriseJSONComments applies ANSI styling to comment-focused JSON payloads.
func ColouriseJSONComments(enabled bool, payload []byte) []byte {
	if !enabled || len(payload) == 0 {
		return payload
	}

	text := string(payload)

	text = colouriseJSONValue(text, jsonTypePattern, func(value string) string {
		return applyStyle(true, ansiDim, value)
	})

	text = colouriseJSONValue(text, jsonAuthorPattern, func(value string) string {
		return applyStyle(true, ansiCyan, value)
	})

	text = colouriseJSONValue(text, jsonCreatedAtPattern, func(value string) string {
		return applyStyle(true, ansiDim, value)
	})

	text = colouriseJSONValue(text, jsonBodyTextPattern, func(value string) string {
		return highlightInlineCode(value)
	})

	text = colouriseJSONValue(text, jsonPermalinkPattern, func(value string) string {
		return applyStyles(true, value, ansiUnderline, ansiMagenta)
	})

	return []byte(text)
}

func colouriseJSONValue(text string, pattern *regexp.Regexp, transform func(string) string) string {
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := pattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		prefix := sub[1]
		value := sub[2]
		styled := transform(value)
		return prefix + `"` + styled + `"`
	})
}

func highlightInlineCode(value string) string {
	if value == "" {
		return value
	}

	needsColour := false
	for i := 0; i < len(value); i++ {
		if value[i] == '`' {
			needsColour = true
			break
		}
	}
	if !needsColour {
		return value
	}

	var b strings.Builder
	b.Grow(len(value) + 16) // slight headroom for escape sequences
	inCode := false

	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '`' {
			if inCode {
				b.WriteByte('`')
				b.WriteString(ansiReset)
				inCode = false
			} else {
				b.WriteString(ansiYellow)
				b.WriteByte('`')
				inCode = true
			}
			continue
		}
		b.WriteByte(ch)
	}

	if inCode {
		b.WriteString(ansiReset)
	}

	return b.String()
}
