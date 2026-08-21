package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Saluev/kc-review-bot/common"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "reviewers.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestLoadReviewersArray(t *testing.T) {
	path := writeTemp(t, `[
		{"username": "spidy", "discordId": "<@111>", "labels": ["backend"]},
		{"username": "groot", "discordId": "@groot"}
	]`)

	got := loadReviewers(path)

	want := map[string]common.Reviewer{
		"spidy": {Username: "spidy", DiscordID: "<@111>", Labels: []string{"backend"}},
		"groot": {Username: "groot", DiscordID: "@groot"},
	}
	require.Equal(t, want, got)
}

func TestLoadReviewersLegacyMap(t *testing.T) {
	path := writeTemp(t, `{
		"hulk51": "@hulk",
		"tonystark": "@iron_man"
	}`)

	got := loadReviewers(path)

	want := map[string]common.Reviewer{
		"hulk51":    {Username: "hulk51", DiscordID: "@hulk"},
		"tonystark": {Username: "tonystark", DiscordID: "@iron_man"},
	}
	require.Equal(t, want, got)
}

func TestLoadReviewersEmailSlackIDMap(t *testing.T) {
	path := writeTemp(t, `{
		"s.medina": {"email": "s.medina@example.com", "slack_id": "U0BN1HW8KQB"},
		"a.arciniegas": {"email": "a.arciniegas@example.com", "slack_id": "<@U0BND6987AR>"},
		"x.yz": {"email": "x.yz@example.com"}
	}`)

	got := loadReviewers(path)

	want := map[string]common.Reviewer{
		"s.medina":     {Username: "s.medina", DiscordID: "<@U0BN1HW8KQB>"},
		"a.arciniegas": {Username: "a.arciniegas", DiscordID: "<@U0BND6987AR>"},
		"x.yz":         {Username: "x.yz", DiscordID: "x.yz@example.com"},
	}
	require.Equal(t, want, got)
}

func TestLoadReviewersRootFile(t *testing.T) {
	got := loadReviewers("reviewers.json")

	require.Contains(t, got, "s.medina")
	require.Equal(t, "<@U0BN1HW8KQB>", got["s.medina"].DiscordID)
}
