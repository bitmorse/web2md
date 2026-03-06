package cmd

import (
	"strings"
	"testing"
)

func TestHighlightKeywords(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		want     string
	}{
		{
			text:     "the bearing is worn",
			keywords: []string{"bearing"},
			want:     "the BEARING is worn",
		},
		{
			text:     "starch residue found",
			keywords: []string{"starch", "residue"},
			want:     "STARCH RESIDUE found",
		},
		{
			text:     "no match here",
			keywords: []string{"bearing"},
			want:     "no match here",
		},
		{
			text:     "",
			keywords: []string{"word"},
			want:     "",
		},
	}

	for _, tt := range tests {
		got := highlightKeywords(tt.text, tt.keywords)
		if got != tt.want {
			t.Errorf("highlightKeywords(%q, %v) = %q, want %q", tt.text, tt.keywords, got, tt.want)
		}
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		text  string
		width int
		lines int
	}{
		{"short text", 50, 1},
		{"", 50, 1},
		{"word " + strings.Repeat("x", 60), 50, 2},
	}

	for _, tt := range tests {
		got := wrapText(tt.text, tt.width)
		if len(got) != tt.lines {
			t.Errorf("wrapText(%q, %d) returned %d lines, want %d", tt.text, tt.width, len(got), tt.lines)
		}
	}
}

func TestPrintBoxLine_truncates(t *testing.T) {
	// Just verify it doesn't panic with long input
	longText := strings.Repeat("a", boxWidth+100)
	// printBoxLine prints to stdout; just call it to check no panic
	printBoxLine(longText)
}
