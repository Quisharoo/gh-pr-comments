package ghprcomments

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MarshalJSON encodes the output as either nested or flat JSON.
func MarshalJSON(out Output, flat bool) ([]byte, error) {
	if flat {
		return json.MarshalIndent(out.Comments, "", "  ")
	}
	return json.MarshalIndent(out, "", "  ")
}

// RenderMarkdown emits a human-readable review summary.
func RenderMarkdown(out Output) string {
	var b strings.Builder

	title := out.PR.Title
	if title == "" {
		title = fmt.Sprintf("PR #%d", out.PR.Number)
	}

	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- Repo: %s\n", safeMarkdownValue(out.PR.Repo))
	fmt.Fprintf(&b, "- Number: #%d\n", out.PR.Number)
	if out.PR.HeadRef != "" {
		fmt.Fprintf(&b, "- Head: %s\n", safeMarkdownValue(out.PR.HeadRef))
	}
	if !out.PR.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "- Updated: %s\n", out.PR.UpdatedAt.Format(time.RFC3339))
	}
	b.WriteString("\n")

	for _, c := range out.Comments {
		heading := formatCommentType(c.Type)
		fmt.Fprintf(&b, "## %s by %s (%s)\n", heading, safeMarkdownValue(c.Author), c.CreatedAt.Format(time.RFC3339))
		if c.Path != "" {
			fmt.Fprintf(&b, "- Path: %s\n", safeMarkdownValue(c.Path))
		}
		if c.Line != nil {
			fmt.Fprintf(&b, "- Line: %d\n", *c.Line)
		}
		if c.State != "" {
			fmt.Fprintf(&b, "- State: %s\n", safeMarkdownValue(c.State))
		}
		if c.Permalink != "" {
			fmt.Fprintf(&b, "- Link: %s\n", c.Permalink)
		}
		b.WriteString("\n")
		b.WriteString(blockQuote(c.BodyText))
		b.WriteString("\n\n")
	}

	return strings.TrimSpace(b.String()) + "\n"
}

func blockQuote(body string) string {
	if body == "" {
		return "> (empty)"
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = "> " + strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func safeMarkdownValue(value string) string {
	if value == "" {
		return "(unknown)"
	}
	return value
}

func formatCommentType(kind string) string {
	if kind == "" {
		return "Comment"
	}
	parts := strings.Split(kind, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
