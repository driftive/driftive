package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateBytesNoOpWhenShorter(t *testing.T) {
	if got := TruncateBytes("hello", 64); got != "hello" {
		t.Errorf("TruncateBytes() = %q, want %q", got, "hello")
	}
}

func TestTruncateBytesExactBoundary(t *testing.T) {
	if got := TruncateBytes("hello", 5); got != "hello" {
		t.Errorf("TruncateBytes() = %q, want %q", got, "hello")
	}
}

// TestTruncateBytesCutsOnRuneBoundary covers the terraform diagnostic frame characters
// (╷ │ ╵), which are 3 bytes each: a naive byte slice splits one and yields invalid UTF-8,
// which the GitHub API rejects.
func TestTruncateBytesCutsOnRuneBoundary(t *testing.T) {
	input := strings.Repeat("╷", 10)

	for _, max := range []int{1, 2, 3, 4, 5, 7, 28, 29} {
		got := TruncateBytes(input, max)
		if !utf8.ValidString(got) {
			t.Errorf("TruncateBytes(max=%d) produced invalid UTF-8: %q", max, got)
		}
		if len(got) > max {
			t.Errorf("TruncateBytes(max=%d) returned %d bytes", max, len(got))
		}
	}
}

func TestTruncateBytesKeepsWholeRunesThatFit(t *testing.T) {
	if got := TruncateBytes("╷╷╷", 6); got != "╷╷" {
		t.Errorf("TruncateBytes() = %q, want %q", got, "╷╷")
	}
}

func TestTruncateBytesZeroAndNegativeMax(t *testing.T) {
	if got := TruncateBytes("hello", 0); got != "" {
		t.Errorf("TruncateBytes(max=0) = %q, want empty", got)
	}
	if got := TruncateBytes("hello", -1); got != "" {
		t.Errorf("TruncateBytes(max=-1) = %q, want empty", got)
	}
}

func TestTruncateBytesEmptyInput(t *testing.T) {
	if got := TruncateBytes("", 10); got != "" {
		t.Errorf("TruncateBytes() = %q, want empty", got)
	}
}
