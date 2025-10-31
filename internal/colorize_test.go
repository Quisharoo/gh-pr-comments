package ghprcomments

import (
	"bytes"
	"os"
	"path/filepath"
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
	payload := []byte("{\n  \"pr\": {\n    \"repo\": \"Quisharoo/gh-pr-comments\",\n    \"number\": 42,\n    \"url\": \"https://github.com/org/repo/pull/42\",\n    \"head_ref\": \"feature\",\n    \"base_ref\": \"main\",\n    \"updated_at\": \"2025-10-24T12:00:00Z\"\n  },\n  \"comments\": [\n    {\n      \"author\": \"octocat\",\n      \"comments\": [\n        {\n          \"type\": \"review_comment\",\n          \"created_at\": \"2025-10-23T12:34:56Z\",\n          \"body_text\": \"use `fmt` please\",\n          \"permalink\": \"https://example.test/path\"\n        }\n      ]\n    }\n  ]\n}\n")

	coloured := string(ColouriseJSONComments(true, payload))

	// Test that keys are styled (dimStyle renders with faint)
	typeKey := "\"" + dimStyle.Render("type") + "\":"
	if !strings.Contains(coloured, typeKey) {
		t.Fatalf("expected dim key styling for type, missing %q", typeKey)
	}

	// Test that type value is green
	typeValue := "\"" + greenStyle.Render("review_comment") + "\""
	if !strings.Contains(coloured, typeValue) {
		t.Fatalf("expected coloured type value, missing %q in %q", typeValue, coloured)
	}

	// Test that author key is styled
	authorKey := "\"" + dimStyle.Render("author") + "\":"
	if !strings.Contains(coloured, authorKey) {
		t.Fatalf("expected dim key styling for author, missing %q", authorKey)
	}

	// Test that author value is bright cyan
	authorValue := "\"" + brightCyanStyle.Render("octocat") + "\""
	if !strings.Contains(coloured, authorValue) {
		t.Fatalf("expected coloured author value, missing %q", authorValue)
	}

	// Test that timestamp key is styled
	createdKey := "\"" + dimStyle.Render("created_at") + "\":"
	if !strings.Contains(coloured, createdKey) {
		t.Fatalf("expected dim key styling for created_at, missing %q", createdKey)
	}

	// Test that timestamp value is faint
	createdValue := "\"" + faintStyle.Render("2025-10-23T12:34:56Z") + "\""
	if !strings.Contains(coloured, createdValue) {
		t.Fatalf("expected coloured created_at value, missing %q", createdValue)
	}

	// Test inline code highlighting (yellow style applied to `fmt`)
	inlineCode := yellowStyle.Render("`fmt`")
	if !strings.Contains(coloured, inlineCode) {
		t.Fatalf("expected inline code segment to be highlighted, missing %q", inlineCode)
	}

	// Test permalink key
	permalinkKey := "\"" + dimStyle.Render("permalink") + "\":"
	if !strings.Contains(coloured, permalinkKey) {
		t.Fatalf("expected dim key styling for permalink, missing %q", permalinkKey)
	}

	// Test permalink value (link style + hyperlink)
	permalinkValue := "\"" + applyHyperlink(true, "https://example.test/path", linkStyle.Render("https://example.test/path")) + "\""
	if !strings.Contains(coloured, permalinkValue) {
		t.Fatalf("expected coloured permalink value, missing %q", permalinkValue)
	}

	// Test PR URL key
	prURLKey := "\"" + dimStyle.Render("url") + "\":"
	if !strings.Contains(coloured, prURLKey) {
		t.Fatalf("expected dim key styling for url, missing %q", prURLKey)
	}

	// Test PR URL value (link style + hyperlink)
	prURLValue := "\"" + applyHyperlink(true, "https://github.com/org/repo/pull/42", linkStyle.Render("https://github.com/org/repo/pull/42")) + "\""
	if !strings.Contains(coloured, prURLValue) {
		t.Fatalf("expected coloured PR url value, missing %q", prURLValue)
	}
}

// TestColouriseJSONGolden is a golden file test to detect visual regressions
// when refactoring ANSI code to lipgloss
func TestColouriseJSONGolden(t *testing.T) {
	// Read golden input
	inputPath := filepath.Join("testdata", "golden", "colorize_sample.json")
	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("failed to read golden input: %v", err)
	}

	// Colorize the JSON
	output := ColouriseJSONComments(true, input)

	// Golden output path
	goldenPath := filepath.Join("testdata", "golden", "colorize_sample.golden")

	// Update mode: set UPDATE_GOLDEN=1 to regenerate golden files
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, output, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Log("Golden file updated")
		return
	}

	// Read expected golden output
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}

	// Compare byte-for-byte
	if !bytes.Equal(output, expected) {
		t.Errorf("colorize output differs from golden file\nGot length: %d\nExpected length: %d", len(output), len(expected))
		t.Errorf("Run with UPDATE_GOLDEN=1 to update golden file if this is intentional")
	}
}
