package main

import (
	"slices"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	testCases := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "gh extension invocation",
			input: []string{"prcomments", "--flat"},
			want:  []string{"--flat"},
		},
		{
			name:  "binary name injected",
			input: []string{"gh-prcomments", "--text"},
			want:  []string{"--text"},
		},
		{
			name:  "legacy binary name injected",
			input: []string{"gh-pr-comments", "--text"},
			want:  []string{"--text"},
		},
		{
			name:  "no injected command",
			input: []string{"--save"},
			want:  []string{"--save"},
		},
		{
			name:  "non matching leading token left intact",
			input: []string{"some-arg", "--flat"},
			want:  []string{"some-arg", "--flat"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeArgs(tc.input)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("normalizeArgs(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
