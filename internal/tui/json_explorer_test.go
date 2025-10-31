package tui

import (
	"reflect"
	"testing"
)

// Tests for wrapString using muesli/reflow
// Note: muesli/reflow provides better whitespace handling and doesn't hard-break words,
// which is appropriate for JSON value display (minimum width is 20 in practice)
func TestWrapString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			width:    10,
			expected: []string{""},
		},
		{
			name:     "string shorter than width",
			input:    "hello",
			width:    10,
			expected: []string{"hello"},
		},
		{
			name:     "string exactly width",
			input:    "hello",
			width:    5,
			expected: []string{"hello"},
		},
		{
			name:     "simple word wrap",
			input:    "hello world",
			width:    7,
			expected: []string{"hello", "world"},
		},
		{
			name:     "multiple lines",
			input:    "the quick brown fox jumps over the lazy dog",
			width:    15,
			expected: []string{"the quick brown", "fox jumps over", "the lazy dog"},
		},
		{
			name:     "long word doesn't hard break",
			input:    "supercalifragilisticexpialidocious",
			width:    10,
			expected: []string{"supercalifragilisticexpialidocious"}, // reflow doesn't break words - better for URLs/hashes
		},
		{
			name:     "word exactly at boundary",
			input:    "hello world test",
			width:    11,
			expected: []string{"hello world", "test"},
		},
		{
			name:     "multiple spaces normalized",
			input:    "hello    world",
			width:    8,
			expected: []string{"hello", "world"}, // reflow normalizes whitespace - cleaner output
		},
		{
			name:     "tabs handled properly",
			input:    "hello\tworld test",
			width:    10,
			expected: []string{"hello", "world test"}, // reflow treats tabs as word boundaries
		},
		{
			name:     "newline breaks properly",
			input:    "hello\nworld",
			width:    10,
			expected: []string{"hello", "world"}, // reflow respects newlines as word boundaries
		},
		{
			name:     "zero width",
			input:    "hello world",
			width:    0,
			expected: []string{"hello world"},
		},
		{
			name:     "negative width",
			input:    "hello world",
			width:    -1,
			expected: []string{"hello world"},
		},
		{
			name:     "very narrow width - doesn't hard break",
			input:    "hello",
			width:    1,
			expected: []string{"hello"}, // reflow preserves words - fine since min width is 20 in practice
		},
		{
			name:     "narrow width preserves words",
			input:    "hello world",
			width:    2,
			expected: []string{"hello", "world"}, // reflow preserves words even at narrow widths
		},
		{
			name:     "preserves leading spaces",
			input:    "  hello world",
			width:    20,
			expected: []string{"  hello world"}, // leading spaces preserved
		},
		{
			name:     "preserves trailing spaces",
			input:    "hello world  ",
			width:    20,
			expected: []string{"hello world  "}, // trailing spaces preserved
		},
		{
			name:     "preserves both leading and trailing spaces",
			input:    "  hello  ",
			width:    20,
			expected: []string{"  hello  "}, // both preserved
		},
		{
			name:     "preserves trailing spaces with wrapping",
			input:    "line with trailing  ",
			width:    10,
			expected: []string{"line with", "trailing  "}, // trailing spaces on last line
		},
		{
			name:     "preserves leading spaces with wrapping",
			input:    "  long line that wraps",
			width:    10,
			expected: []string{"  long", "line that", "wraps"}, // leading spaces on first line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapString(tt.input, tt.width)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("wrapString(%q, %d) = %#v, want %#v", tt.input, tt.width, result, tt.expected)
			}
		})
	}
}

// TestPhysicalLineTracking tests that multi-line entries are tracked correctly
func TestPhysicalLineTracking(t *testing.T) {
	// Create a simple JSON with a long string that will wrap
	jsonData := []byte(`{
		"short": "value",
		"long": "this is a very long string that should wrap across multiple lines when rendered in a narrow terminal"
	}`)

	model, err := NewJSONExplorerModel(jsonData)
	if err != nil {
		t.Fatalf("NewJSONExplorerModel failed: %v", err)
	}

	// Set narrow width to force wrapping
	model.width = 40

	// Render tree to compute physical offsets
	model.renderTree()

	// Verify that nodes have physical line info
	if len(model.flatNodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(model.flatNodes))
	}

	// Check that physical offsets are computed
	for i, node := range model.flatNodes {
		if node.PhysicalLines == 0 {
			t.Errorf("node %d (%q) has PhysicalLines=0, expected > 0", i, node.Key)
		}
	}

	// Verify offsets are cumulative
	if len(model.flatNodes) > 1 {
		for i := 1; i < len(model.flatNodes); i++ {
			prevNode := model.flatNodes[i-1]
			currNode := model.flatNodes[i]

			expectedOffset := prevNode.PhysicalOffset + prevNode.PhysicalLines
			if currNode.PhysicalOffset != expectedOffset {
				t.Errorf("node %d offset = %d, want %d (prev offset %d + prev lines %d)",
					i, currNode.PhysicalOffset, expectedOffset,
					prevNode.PhysicalOffset, prevNode.PhysicalLines)
			}
		}
	}
}

// TestBuildTree tests the JSON tree building logic
func TestBuildTree(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    interface{}
		depth    int
		wantType string
		wantKids int
	}{
		{
			name:     "string value",
			key:      "name",
			value:    "test",
			depth:    0,
			wantType: "string",
			wantKids: 0,
		},
		{
			name:     "number value",
			key:      "count",
			value:    42.0,
			depth:    0,
			wantType: "number",
			wantKids: 0,
		},
		{
			name:     "bool value",
			key:      "active",
			value:    true,
			depth:    0,
			wantType: "bool",
			wantKids: 0,
		},
		{
			name:     "null value",
			key:      "empty",
			value:    nil,
			depth:    0,
			wantType: "null",
			wantKids: 0,
		},
		{
			name:  "object with children",
			key:   "user",
			value: map[string]interface{}{"name": "alice", "age": 30.0},
			depth: 0,
			wantType: "object",
			wantKids: 2,
		},
		{
			name:     "array with elements",
			key:      "items",
			value:    []interface{}{"a", "b", "c"},
			depth:    0,
			wantType: "array",
			wantKids: 3,
		},
		{
			name:     "depth 0 auto-expands",
			key:      "root",
			value:    map[string]interface{}{"child": "value"},
			depth:    0,
			wantType: "object",
			wantKids: 1,
		},
		{
			name:     "depth 1 auto-expands",
			key:      "level1",
			value:    map[string]interface{}{"child": "value"},
			depth:    1,
			wantType: "object",
			wantKids: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := buildTree(tt.key, tt.value, nil, tt.depth)

			if node.Key != tt.key {
				t.Errorf("key = %q, want %q", node.Key, tt.key)
			}
			if node.Type != tt.wantType {
				t.Errorf("type = %q, want %q", node.Type, tt.wantType)
			}
			if node.Depth != tt.depth {
				t.Errorf("depth = %d, want %d", node.Depth, tt.depth)
			}
			if len(node.Children) != tt.wantKids {
				t.Errorf("children count = %d, want %d", len(node.Children), tt.wantKids)
			}

			// Check auto-expand behavior (depth < 2 should expand)
			if tt.depth < 2 && !node.Expanded {
				t.Errorf("expected node at depth %d to be auto-expanded", tt.depth)
			}
		})
	}
}

// TestFlattenTree tests converting tree to flat list for navigation
func TestFlattenTree(t *testing.T) {
	// Build a simple tree: root { a, b { c } }
	root := &JSONNode{
		Key:      "root",
		Type:     "object",
		Expanded: true,
		Children: []*JSONNode{
			{Key: "a", Type: "string", Value: "value_a", Expanded: false, Children: nil},
			{
				Key:      "b",
				Type:     "object",
				Expanded: true,
				Children: []*JSONNode{
					{Key: "c", Type: "string", Value: "value_c", Expanded: false, Children: nil},
				},
			},
		},
	}

	flat := flattenTree(root)

	// Should have: root, a, b, c (all expanded)
	if len(flat) != 4 {
		t.Fatalf("expected 4 flattened nodes, got %d", len(flat))
	}

	// Check order
	expectedKeys := []string{"root", "a", "b", "c"}
	for i, expected := range expectedKeys {
		if flat[i].Key != expected {
			t.Errorf("flat[%d].Key = %q, want %q", i, flat[i].Key, expected)
		}
		if flat[i].Index != i {
			t.Errorf("flat[%d].Index = %d, want %d", i, flat[i].Index, i)
		}
		if flat[i].LineNumber != i+1 {
			t.Errorf("flat[%d].LineNumber = %d, want %d", i, flat[i].LineNumber, i+1)
		}
	}
}

// TestFlattenTreeCollapsed tests that collapsed nodes hide children
func TestFlattenTreeCollapsed(t *testing.T) {
	root := &JSONNode{
		Key:      "root",
		Type:     "object",
		Expanded: true,
		Children: []*JSONNode{
			{Key: "a", Type: "string", Value: "value_a", Expanded: false, Children: nil},
			{
				Key:      "b",
				Type:     "object",
				Expanded: false, // Collapsed
				Children: []*JSONNode{
					{Key: "c", Type: "string", Value: "hidden", Expanded: false, Children: nil},
				},
			},
		},
	}

	flat := flattenTree(root)

	// Should have: root, a, b (c is hidden because b is collapsed)
	if len(flat) != 3 {
		t.Fatalf("expected 3 flattened nodes (c hidden), got %d", len(flat))
	}

	expectedKeys := []string{"root", "a", "b"}
	for i, expected := range expectedKeys {
		if flat[i].Key != expected {
			t.Errorf("flat[%d].Key = %q, want %q", i, flat[i].Key, expected)
		}
	}
}

// TestExtractURL tests URL extraction from JSON nodes
func TestExtractURL(t *testing.T) {
	model := JSONExplorerModel{}

	tests := []struct {
		name     string
		node     *JSONNode
		expected string
	}{
		{
			name:     "nil node returns empty",
			node:     nil,
			expected: "",
		},
		{
			name: "string with URL",
			node: &JSONNode{
				Key:   "permalink",
				Type:  "string",
				Value: "https://github.com/owner/repo/pull/123",
			},
			expected: "https://github.com/owner/repo/pull/123",
		},
		{
			name: "URL-like key",
			node: &JSONNode{
				Key:   "api_url",
				Type:  "string",
				Value: "https://api.github.com/repos/owner/repo",
			},
			expected: "https://api.github.com/repos/owner/repo",
		},
		{
			name: "key with 'link'",
			node: &JSONNode{
				Key:   "html_link",
				Type:  "string",
				Value: "http://example.com/page",
			},
			expected: "http://example.com/page",
		},
		{
			name: "key with 'href'",
			node: &JSONNode{
				Key:   "href",
				Type:  "string",
				Value: "https://example.org/path",
			},
			expected: "https://example.org/path",
		},
		{
			name: "non-URL string",
			node: &JSONNode{
				Key:   "title",
				Type:  "string",
				Value: "Just a title",
			},
			expected: "",
		},
		{
			name: "number value",
			node: &JSONNode{
				Key:   "count",
				Type:  "number",
				Value: 42,
			},
			expected: "",
		},
		{
			name: "URL in non-string type",
			node: &JSONNode{
				Key:   "data",
				Type:  "object",
				Value: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.extractURL(tt.node)
			if result != tt.expected {
				t.Errorf("extractURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestApplySearch tests search filtering logic
func TestApplySearch(t *testing.T) {
	nodes := []*JSONNode{
		{Key: "name", Type: "string", Value: "Alice"},
		{Key: "age", Type: "number", Value: 30},
		{Key: "email", Type: "string", Value: "alice@example.com"},
		{Key: "admin", Type: "bool", Value: true},
	}

	model := &JSONExplorerModel{
		flatNodes: nodes,
	}

	tests := []struct {
		query       string
		expectMatch []int // indices that should match
	}{
		{
			query:       "",
			expectMatch: []int{}, // Empty query matches nothing
		},
		{
			query:       "alice",
			expectMatch: []int{0, 2}, // "name" value and "email" value
		},
		{
			query:       "age",
			expectMatch: []int{1}, // "age" key
		},
		{
			query:       "30",
			expectMatch: []int{1}, // "age" value
		},
		{
			query:       "admin",
			expectMatch: []int{3}, // "admin" key
		},
		{
			query:       "xyz",
			expectMatch: []int{}, // No matches
		},
	}

	for _, tt := range tests {
		t.Run("query_"+tt.query, func(t *testing.T) {
			model.searchQuery = tt.query
			model.applySearch()

			matchCount := 0
			for i, node := range nodes {
				if node.Matches {
					matchCount++
					found := false
					for _, expectedIdx := range tt.expectMatch {
						if i == expectedIdx {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("node %d (%q) matched but shouldn't have", i, node.Key)
					}
				}
			}

			if matchCount != len(tt.expectMatch) {
				t.Errorf("matched %d nodes, expected %d", matchCount, len(tt.expectMatch))
			}
		})
	}
}

// TestExpandCollapseAll tests expand/collapse recursion
func TestExpandCollapseAll(t *testing.T) {
	root := &JSONNode{
		Key:      "root",
		Expanded: false,
		Children: []*JSONNode{
			{
				Key:      "child1",
				Expanded: false,
				Children: []*JSONNode{
					{Key: "grandchild", Expanded: false, Children: nil},
				},
			},
			{Key: "child2", Expanded: false, Children: nil},
		},
	}

	// Test expandAll
	expandAll(root)
	if !root.Expanded {
		t.Error("root should be expanded")
	}
	if !root.Children[0].Expanded {
		t.Error("child1 should be expanded")
	}
	if !root.Children[0].Children[0].Expanded {
		t.Error("grandchild should be expanded")
	}
	if !root.Children[1].Expanded {
		t.Error("child2 should be expanded")
	}

	// Test collapseAll
	collapseAll(root)
	if root.Expanded {
		t.Error("root should be collapsed")
	}
	if root.Children[0].Expanded {
		t.Error("child1 should be collapsed")
	}
	if root.Children[0].Children[0].Expanded {
		t.Error("grandchild should be collapsed")
	}
	if root.Children[1].Expanded {
		t.Error("child2 should be collapsed")
	}
}
