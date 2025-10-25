package ghprcomments

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFlattenCommentGroupsOrdersByCreatedAtDesc(t *testing.T) {
	earlier := time.Date(2025, time.October, 24, 10, 0, 0, 0, time.UTC)
	later := earlier.Add(2 * time.Hour)

	groups := []AuthorComments{
		{
			Author: "alice",
			Comments: []Comment{
				{Type: "issue", Author: "alice", CreatedAt: earlier, ID: 1},
			},
		},
		{
			Author: "bob",
			Comments: []Comment{
				{Type: "review_comment", Author: "bob", CreatedAt: later, ID: 2},
			},
		},
	}

	flat := flattenCommentGroups(groups)

	if len(flat) != 2 {
		t.Fatalf("expected 2 flattened comments, got %d", len(flat))
	}
	if flat[0].Author != "bob" {
		t.Fatalf("expected most recent comment first, got %q", flat[0].Author)
	}
	if flat[1].Author != "alice" {
		t.Fatalf("expected older comment second, got %q", flat[1].Author)
	}
}

func TestMarshalJSONFlatProducesArrayOfComments(t *testing.T) {
	out := Output{
		PR: PullRequestMetadata{
			Repo:   "owner/repo",
			Number: 7,
		},
		CommentCount: 1,
		Comments: []AuthorComments{
			{
				Author: "octocat",
				Comments: []Comment{
					{Type: "issue", Author: "octocat", CreatedAt: time.Now()},
				},
			},
		},
	}

	payload, err := MarshalJSON(out, true)
	if err != nil {
		t.Fatalf("marshal flat: %v", err)
	}

	if !strings.HasPrefix(string(payload), "[") {
		t.Fatalf("expected flat JSON to be an array, got %q", string(payload))
	}

	var decoded []map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal flat payload: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 comment in flat payload, got %d", len(decoded))
	}
	if decoded[0]["author"].(string) != "octocat" {
		t.Fatalf("unexpected author in flat payload: %#v", decoded[0]["author"])
	}
}

func TestMarshalJSONIncludesCommentCount(t *testing.T) {
	out := Output{
		PR:           PullRequestMetadata{Repo: "owner/repo", Number: 9},
		CommentCount: 2,
		Comments: []AuthorComments{
			{Author: "octocat", Comments: []Comment{{Type: "issue", Author: "octocat", CreatedAt: time.Now()}}},
			{Author: "hubot", Comments: []Comment{{Type: "review_comment", Author: "hubot", CreatedAt: time.Now()}}},
		},
	}

	payload, err := MarshalJSON(out, false)
	if err != nil {
		t.Fatalf("marshal nested: %v", err)
	}

	if !strings.Contains(string(payload), "\"comment_count\": 2") {
		t.Fatalf("expected payload to include comment_count, got %q", string(payload))
	}
}
