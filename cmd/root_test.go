package cmd

import (
	"os"
	"path/filepath"
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

func TestVersionCmd(t *testing.T) {
	SetVersionInfo("v1.2.3", "abc123", "2026-01-01")
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if buildVersion != "v1.2.3" || buildCommit != "abc123" || buildDate != "2026-01-01" {
		t.Fatal("SetVersionInfo did not set values correctly")
	}
}

func TestRootCmdAcceptsURLFile(t *testing.T) {
	// Create a temp file with URLs
	dir := t.TempDir()
	urlFile := filepath.Join(dir, "urls.txt")
	os.WriteFile(urlFile, []byte("https://example.com\nhttps://example.org\n"), 0644)

	// --url-file should be accepted as a flag
	rootCmd.SetArgs([]string{"--url-file", urlFile})
	err := rootCmd.Execute()
	// Should proceed to crawl (not reject as missing URL)
	if err != nil && err.Error() == `unknown flag: --url-file` {
		t.Fatal("root command does not accept --url-file flag")
	}
}

func TestParseURLFile(t *testing.T) {
	dir := t.TempDir()
	urlFile := filepath.Join(dir, "urls.txt")
	os.WriteFile(urlFile, []byte("https://example.com\n\n# comment\nhttps://example.org\n  \n"), 0644)

	urls, err := parseURLFile(urlFile)
	if err != nil {
		t.Fatalf("parseURLFile() error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("parseURLFile() returned %d urls, want 2", len(urls))
	}
	if urls[0] != "https://example.com" || urls[1] != "https://example.org" {
		t.Errorf("parseURLFile() = %v, want [https://example.com https://example.org]", urls)
	}
}

func TestYesFlagExists(t *testing.T) {
	// The -y flag should exist on the root command
	f := rootCmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("-y/--yes flag not found on root command")
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
