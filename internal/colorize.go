package ghprcomments

import (
	"regexp"
	"strings"
)

var (
	jsonTypePattern      = regexp.MustCompile(`("type":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonAuthorPattern    = regexp.MustCompile(`("author":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonRepoPattern      = regexp.MustCompile(`("repo":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonCreatedAtPattern = regexp.MustCompile(`("created_at":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonUpdatedAtPattern = regexp.MustCompile(`("updated_at":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonHeadRefPattern   = regexp.MustCompile(`("head_ref":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonBaseRefPattern   = regexp.MustCompile(`("base_ref":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonBodyTextPattern  = regexp.MustCompile(`("body_text":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonPermalinkPattern = regexp.MustCompile(`("permalink":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonPRURLPattern     = regexp.MustCompile(`("url":\s*)"((?:[^"\\]|\\.)*)"`)
	jsonPRNumberPattern  = regexp.MustCompile(`("number":\s*)(\d+)`)
)

// ColouriseJSONComments applies ANSI styling to comment-focused JSON payloads.
func ColouriseJSONComments(enabled bool, payload []byte) []byte {
	if !enabled || len(payload) == 0 {
		return payload
	}

	text := string(payload)

	text = colouriseJSONNumber(text, jsonPRNumberPattern, func(value string) string {
		return applyStyle(true, ansiYellow, value)
	})

	text = colouriseJSONValue(text, jsonRepoPattern, func(value string) string {
		return applyStyle(true, ansiBrightCyan, value)
	})

	text = colouriseJSONValue(text, jsonTypePattern, func(value string) string {
		return applyStyle(true, ansiGreen, value)
	})

	text = colouriseJSONValue(text, jsonAuthorPattern, func(value string) string {
		return applyStyle(true, ansiBrightCyan, value)
	})

	text = colouriseJSONValue(text, jsonCreatedAtPattern, func(value string) string {
		return applyStyle(true, ansiFaint, value)
	})

	text = colouriseJSONValue(text, jsonUpdatedAtPattern, func(value string) string {
		return applyStyle(true, ansiFaint, value)
	})

	text = colouriseJSONValue(text, jsonHeadRefPattern, func(value string) string {
		return applyStyle(true, ansiMagenta, value)
	})

	text = colouriseJSONValue(text, jsonBaseRefPattern, func(value string) string {
		return applyStyle(true, ansiMagenta, value)
	})

	text = colouriseJSONValue(text, jsonBodyTextPattern, func(value string) string {
		return highlightInlineCode(value)
	})

	text = colouriseJSONValue(text, jsonPermalinkPattern, func(value string) string {
		styled := applyStyles(true, value, ansiUnderline, ansiBlue)
		return applyHyperlink(true, value, styled)
	})

	text = colouriseJSONValue(text, jsonPRURLPattern, func(value string) string {
		styled := applyStyles(true, value, ansiUnderline, ansiBlue)
		return applyHyperlink(true, value, styled)
	})

	text = colouriseJSONKeys(text, func(key string) string {
		return applyStyle(true, ansiDim, key)
	})

	return []byte(text)
}

func colouriseJSONKeys(text string, transform func(string) string) string {
	var b strings.Builder
	var current strings.Builder
	inString := false
	escape := false

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if inString {
			if escape {
				escape = false
				current.WriteByte(ch)
				continue
			}
			if ch == '\\' {
				escape = true
				current.WriteByte(ch)
				continue
			}
			if ch == '"' {
				inString = false
				isKey := false
				for j := i + 1; j < len(text); j++ {
					c := text[j]
					switch c {
					case ' ', '\t', '\n', '\r':
						continue
					case ':':
						isKey = true
					}
					break
				}
				if isKey {
					b.WriteString(transform(current.String()))
				} else {
					b.WriteString(current.String())
				}
				b.WriteByte('"')
				current.Reset()
				continue
			}
			current.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = true
			escape = false
			b.WriteByte('"')
			current.Reset()
			continue
		}

		b.WriteByte(ch)
	}

	if inString {
		b.WriteString(current.String())
	}

	return b.String()
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

func colouriseJSONNumber(text string, pattern *regexp.Regexp, transform func(string) string) string {
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := pattern.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		prefix := sub[1]
		value := sub[2]
		styled := transform(value)
		return prefix + styled
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
	b.Grow(len(value) + 16)
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
