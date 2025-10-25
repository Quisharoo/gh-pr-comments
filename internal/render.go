package ghprcomments

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// MarshalJSON encodes the output as either nested or flat JSON.
func MarshalJSON(out Output, flat bool) ([]byte, error) {
	if flat {
		return json.MarshalIndent(flattenCommentGroups(out.Comments), "", "  ")
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
	if out.PR.URL != "" {
		fmt.Fprintf(&b, "- URL: %s\n", out.PR.URL)
	}
	if out.PR.HeadRef != "" || out.PR.BaseRef != "" {
		switch {
		case out.PR.HeadRef != "" && out.PR.BaseRef != "":
			fmt.Fprintf(&b, "- Branch: %s → %s\n", safeMarkdownValue(out.PR.HeadRef), safeMarkdownValue(out.PR.BaseRef))
		case out.PR.HeadRef != "":
			fmt.Fprintf(&b, "- Branch: %s\n", safeMarkdownValue(out.PR.HeadRef))
		case out.PR.BaseRef != "":
			fmt.Fprintf(&b, "- Branch: %s\n", safeMarkdownValue(out.PR.BaseRef))
		}
	}
	if !out.PR.UpdatedAt.IsZero() {
		fmt.Fprintf(&b, "- Updated: %s\n", out.PR.UpdatedAt.Format(time.RFC3339))
	}
	b.WriteString("\n")

	for _, group := range out.Comments {
		fmt.Fprintf(&b, "## %s\n\n", safeMarkdownValue(group.Author))
		for _, c := range group.Comments {
			heading := formatCommentType(c.Type)
			timestamp := "(unknown time)"
			if !c.CreatedAt.IsZero() {
				timestamp = c.CreatedAt.Format(time.RFC3339)
			}
			fmt.Fprintf(&b, "### %s — %s\n", heading, timestamp)
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

func flattenCommentGroups(groups []AuthorComments) []Comment {
	total := 0
	for _, group := range groups {
		total += len(group.Comments)
	}
	flat := make([]Comment, 0, total)
	for _, group := range groups {
		flat = append(flat, group.Comments...)
	}
	sort.SliceStable(flat, func(i, j int) bool {
		if flat[i].CreatedAt.Equal(flat[j].CreatedAt) {
			return flat[i].ID > flat[j].ID
		}
		return flat[i].CreatedAt.After(flat[j].CreatedAt)
	})
	return flat
}
