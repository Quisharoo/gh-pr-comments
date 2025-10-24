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
	payload := []byte("[\n  {\n    \"type\": \"review_comment\",\n    \"author\": \"octocat\",\n    \"created_at\": \"2025-10-23T12:34:56Z\",\n    \"body_text\": \"use `fmt` please\",\n    \"permalink\": \"https://example.test/path\"\n  }\n]\n")

	coloured := string(ColouriseJSONComments(true, payload))

	typeFragment := "\"type\": \"" + applyStyle(true, ansiDim, "review_comment") + "\""
	if !strings.Contains(coloured, typeFragment) {
		t.Fatalf("expected coloured type value, missing %q in %q", typeFragment, coloured)
	}

	authorFragment := "\"author\": \"" + applyStyle(true, ansiCyan, "octocat") + "\""
	if !strings.Contains(coloured, authorFragment) {
		t.Fatalf("expected coloured author value, missing %q", authorFragment)
	}

	createdFragment := "\"created_at\": \"" + applyStyle(true, ansiDim, "2025-10-23T12:34:56Z") + "\""
	if !strings.Contains(coloured, createdFragment) {
		t.Fatalf("expected coloured created_at value, missing %q", createdFragment)
	}

	inlineCode := ansiYellow + "`fmt`" + ansiReset
	if !strings.Contains(coloured, inlineCode) {
		t.Fatalf("expected inline code segment to be highlighted, missing %q", inlineCode)
	}

	permalinkFragment := "\"permalink\": \"" + applyStyles(true, "https://example.test/path", ansiUnderline, ansiMagenta) + "\""
	if !strings.Contains(coloured, permalinkFragment) {
		t.Fatalf("expected coloured permalink value, missing %q", permalinkFragment)
	}
}
