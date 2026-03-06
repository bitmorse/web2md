package cmd

import (
	"testing"
)

func TestRootCmdAcceptsURL(t *testing.T) {
	// The root command must accept a URL as a positional argument
	// instead of treating it as an unknown subcommand.
	rootCmd.SetArgs([]string{"https://example.com"})
	err := rootCmd.Execute()

	// We expect it to proceed (and fail on network/db, not on "unknown command")
	if err != nil && err.Error() == `unknown command "https://example.com" for "web2md"` {
		t.Fatal("root command rejected URL as unknown subcommand")
	}
}

func TestRootCmdRejectsNonURL(t *testing.T) {
	rootCmd.SetArgs([]string{"not-a-url"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-URL argument")
	}
	if err.Error() != "URL must start with http:// or https://" {
		t.Fatalf("unexpected error: %v", err)
	}
}
