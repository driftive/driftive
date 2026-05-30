package config

import "testing"

func TestResolvedVersion(t *testing.T) {
	t.Run("compile-time value wins", func(t *testing.T) {
		got := resolvedVersion("v1.2.3")
		if got != "v1.2.3" {
			t.Fatalf("want v1.2.3, got %q", got)
		}
	})

	t.Run("any non-empty non-dev value is passed through", func(t *testing.T) {
		got := resolvedVersion("v0.0.0-anything")
		if got != "v0.0.0-anything" {
			t.Fatalf("want v0.0.0-anything, got %q", got)
		}
	})

	// For empty/"dev" inputs the fallback resolves from runtime/debug.ReadBuildInfo,
	// which depends on how the test binary was built. We only assert the contract:
	// the function never returns "".
	t.Run("empty input falls back to a non-empty value", func(t *testing.T) {
		if got := resolvedVersion(""); got == "" {
			t.Fatal("resolvedVersion(\"\") returned empty string")
		}
	})

	t.Run("dev input falls back to a non-empty value", func(t *testing.T) {
		if got := resolvedVersion("dev"); got == "" {
			t.Fatal("resolvedVersion(\"dev\") returned empty string")
		}
	})
}
