package ghprcomments

import (
	"bytes"
	"strings"
	"testing"
)

func TestColouriseJSONCommentsDisabled(t *testing.T) {
	input := []byte("{\n  \"type\": \"issue\"\n}\n")
	if got := ColouriseJSONComments(false, input); !bytes.Equal(got, input) {
		t.Fatalf("expected payload to remain unchanged when disabled")
	}
}

func TestColouriseJSONCommentsAppliesStyles(t *testing.T) {
	payload := []byte("{\n  \"pr\": {\n    \"repo\": \"Quish-Labs/gh-pr-comments\",\n    \"number\": 42,\n    \"url\": \"https://github.com/org/repo/pull/42\",\n    \"head_ref\": \"feature\",\n    \"base_ref\": \"main\",\n    \"updated_at\": \"2025-10-24T12:00:00Z\"\n  },\n  \"comments\": [\n    {\n      \"author\": \"octocat\",\n      \"comments\": [\n        {\n          \"type\": \"review_comment\",\n          \"created_at\": \"2025-10-23T12:34:56Z\",\n          \"body_text\": \"use `fmt` please\",\n          \"permalink\": \"https://example.test/path\"\n        }\n      ]\n    }\n  ]\n}\n")

	coloured := string(ColouriseJSONComments(true, payload))

	typeKey := "\"" + applyStyle(true, ansiDim, "type") + "\":"
	if !strings.Contains(coloured, typeKey) {
		t.Fatalf("expected dim key styling for type, missing %q", typeKey)
	}
	typeValue := "\"" + applyStyle(true, ansiGreen, "review_comment") + "\""
	if !strings.Contains(coloured, typeValue) {
		t.Fatalf("expected coloured type value, missing %q in %q", typeValue, coloured)
	}

	authorKey := "\"" + applyStyle(true, ansiDim, "author") + "\":"
	if !strings.Contains(coloured, authorKey) {
		t.Fatalf("expected dim key styling for author, missing %q", authorKey)
	}
	authorValue := "\"" + applyStyle(true, ansiBrightCyan, "octocat") + "\""
	if !strings.Contains(coloured, authorValue) {
		t.Fatalf("expected coloured author value, missing %q", authorValue)
	}

	createdKey := "\"" + applyStyle(true, ansiDim, "created_at") + "\":"
	if !strings.Contains(coloured, createdKey) {
		t.Fatalf("expected dim key styling for created_at, missing %q", createdKey)
	}
	createdValue := "\"" + applyStyle(true, ansiFaint, "2025-10-23T12:34:56Z") + "\""
	if !strings.Contains(coloured, createdValue) {
		t.Fatalf("expected coloured created_at value, missing %q", createdValue)
	}

	inlineCode := ansiYellow + "`fmt`" + ansiReset
	if !strings.Contains(coloured, inlineCode) {
		t.Fatalf("expected inline code segment to be highlighted, missing %q", inlineCode)
	}

	permalinkKey := "\"" + applyStyle(true, ansiDim, "permalink") + "\":"
	if !strings.Contains(coloured, permalinkKey) {
		t.Fatalf("expected dim key styling for permalink, missing %q", permalinkKey)
	}
	permalinkValue := "\"" + applyHyperlink(true, "https://example.test/path", applyStyles(true, "https://example.test/path", ansiUnderline, ansiBlue)) + "\""
	if !strings.Contains(coloured, permalinkValue) {
		t.Fatalf("expected coloured permalink value, missing %q", permalinkValue)
	}

	prURLKey := "\"" + applyStyle(true, ansiDim, "url") + "\":"
	if !strings.Contains(coloured, prURLKey) {
		t.Fatalf("expected dim key styling for url, missing %q", prURLKey)
	}
	prURLValue := "\"" + applyHyperlink(true, "https://github.com/org/repo/pull/42", applyStyles(true, "https://github.com/org/repo/pull/42", ansiUnderline, ansiBlue)) + "\""
	if !strings.Contains(coloured, prURLValue) {
		t.Fatalf("expected coloured PR url value, missing %q", prURLValue)
	}
}
