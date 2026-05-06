package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_DefaultPanelRepositoryUsesCPAManager(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("remote-management:\n  secret-key: \"\"\n"), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if got := loaded.RemoteManagement.PanelGitHubRepository; got != "https://github.com/seakee/CPA-Manager" {
		t.Fatalf("default panel repository = %q", got)
	}
}

func TestLoadConfig_LegacyDefaultPanelRepositoryMigratesToCPAManager(t *testing.T) {
	legacyValues := []string{
		"https://github.com/router-for-me/Cli-Proxy-API-Management-Center",
		"https://github.com/router-for-me/Cli-Proxy-API-Management-Center/",
		"https://github.com/router-for-me/Cli-Proxy-API-Management-Center.git",
		"https://api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center/releases/latest",
	}
	for _, value := range legacyValues {
		value := value
		t.Run(value, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			data := []byte("remote-management:\n  panel-github-repository: \"" + value + "\"\n")
			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			loaded, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if got := loaded.RemoteManagement.PanelGitHubRepository; got != "https://github.com/seakee/CPA-Manager" {
				t.Fatalf("migrated panel repository = %q", got)
			}
		})
	}
}

func TestSanitizeGeminiKeys_TrimsLabel(t *testing.T) {
	cfg := &Config{
		GeminiKey: []GeminiKey{{Label: "  Primary Gemini  ", APIKey: "gemini-key", BaseURL: "https://gemini.example.com"}},
	}

	cfg.SanitizeGeminiKeys()

	if got := cfg.GeminiKey[0].Label; got != "Primary Gemini" {
		t.Fatalf("gemini label = %q, want %q", got, "Primary Gemini")
	}
}

func TestSanitizeClaudeKeys_TrimsLabel(t *testing.T) {
	cfg := &Config{
		ClaudeKey: []ClaudeKey{{Label: "  Main Claude  ", APIKey: "claude-key", BaseURL: "https://claude.example.com"}},
	}

	cfg.SanitizeClaudeKeys()

	if got := cfg.ClaudeKey[0].Label; got != "Main Claude" {
		t.Fatalf("claude label = %q, want %q", got, "Main Claude")
	}
}

func TestSanitizeCodexKeys_TrimsLabel(t *testing.T) {
	cfg := &Config{
		CodexKey: []CodexKey{{Label: "  Main Codex  ", APIKey: "codex-key", BaseURL: "https://codex.example.com"}},
	}

	cfg.SanitizeCodexKeys()

	if got := cfg.CodexKey[0].Label; got != "Main Codex" {
		t.Fatalf("codex label = %q, want %q", got, "Main Codex")
	}
}

func TestSanitizeVertexCompatKeys_TrimsLabel(t *testing.T) {
	cfg := &Config{
		VertexCompatAPIKey: []VertexCompatKey{{Label: "  Main Vertex  ", APIKey: "vertex-key", BaseURL: "https://vertex.example.com"}},
	}

	cfg.SanitizeVertexCompatKeys()

	if got := cfg.VertexCompatAPIKey[0].Label; got != "Main Vertex" {
		t.Fatalf("vertex label = %q, want %q", got, "Main Vertex")
	}
}

func TestSaveConfigPreserveComments_PersistsGeminiLabelOnExistingSequenceItem(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	original := strings.TrimSpace(`
# top-level comment
port: 8317

gemini-api-key:
  # keep this provider comment
  - api-key: gemini-key
    base-url: https://gemini.example.com
    priority: 9
`) + "\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := &Config{
		Port: 8317,
		GeminiKey: []GeminiKey{{
			Label:    "Team A",
			APIKey:   "gemini-key",
			BaseURL:  "https://gemini.example.com",
			Priority: 9,
		}},
	}

	if err := SaveConfigPreserveComments(configPath, cfg); err != nil {
		t.Fatalf("SaveConfigPreserveComments() error = %v", err)
	}

	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read persisted config: %v", err)
	}
	persistedText := string(persisted)
	if !strings.Contains(persistedText, "# keep this provider comment") {
		t.Fatalf("expected provider comment to be preserved, got:\n%s", persistedText)
	}
	if !strings.Contains(persistedText, "label: Team A") {
		t.Fatalf("expected persisted yaml to contain label, got:\n%s", persistedText)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := len(loaded.GeminiKey); got != 1 {
		t.Fatalf("loaded gemini keys len = %d, want 1", got)
	}
	if got := loaded.GeminiKey[0].Label; got != "Team A" {
		t.Fatalf("loaded gemini label = %q, want %q", got, "Team A")
	}
}

func TestSaveConfigPreserveComments_PersistsLabelToMatchingDuplicateAPIKeyEntry(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	original := strings.TrimSpace(`
gemini-api-key:
  # first entry comment
  - api-key: shared-key
    base-url: https://a.example.com
  # second entry comment
  - api-key: shared-key
    base-url: https://b.example.com
`) + "\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg := &Config{
		GeminiKey: []GeminiKey{
			{APIKey: "shared-key", BaseURL: "https://a.example.com"},
			{Label: "Team B", APIKey: "shared-key", BaseURL: "https://b.example.com"},
		},
	}

	if err := SaveConfigPreserveComments(configPath, cfg); err != nil {
		t.Fatalf("SaveConfigPreserveComments() error = %v", err)
	}

	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read persisted config: %v", err)
	}
	persistedText := string(persisted)
	if !strings.Contains(persistedText, "# first entry comment") || !strings.Contains(persistedText, "# second entry comment") {
		t.Fatalf("expected sequence item comments to be preserved, got:\n%s", persistedText)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := len(loaded.GeminiKey); got != 2 {
		t.Fatalf("loaded gemini keys len = %d, want 2", got)
	}
	if got := loaded.GeminiKey[0].Label; got != "" {
		t.Fatalf("loaded first gemini label = %q, want empty", got)
	}
	if got := loaded.GeminiKey[1].Label; got != "Team B" {
		t.Fatalf("loaded second gemini label = %q, want %q", got, "Team B")
	}
}
