package management

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestPatchGeminiKey_ByIndexUpdatesOnlyMatchingLabel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			GeminiKey: []config.GeminiKey{
				{Label: "Primary", APIKey: "shared-key", BaseURL: "https://a.example.com"},
				{Label: "Secondary", APIKey: "shared-key", BaseURL: "https://b.example.com"},
			},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/gemini-api-key", strings.NewReader(`{"index":1,"value":{"label":"  Team B  "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchGeminiKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.GeminiKey[0].Label; got != "Primary" {
		t.Fatalf("gemini key[0] label = %q, want %q", got, "Primary")
	}
	if got := h.cfg.GeminiKey[1].Label; got != "Team B" {
		t.Fatalf("gemini key[1] label = %q, want %q", got, "Team B")
	}
}

func TestPatchGeminiKey_PersistsLabelToConfigFile(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	original := strings.TrimSpace(`
port: 8317

gemini-api-key:
  - api-key: gemini-key
    base-url: https://gemini.example.com
    priority: 9
`) + "\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	h := &Handler{
		cfg: &config.Config{
			Port:      8317,
			GeminiKey: []config.GeminiKey{{APIKey: "gemini-key", BaseURL: "https://gemini.example.com", Priority: 9}},
		},
		configFilePath: configPath,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/gemini-api-key", strings.NewReader(`{"index":0,"value":{"label":"  Team Persist Check  "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchGeminiKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	persisted, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	persistedText := string(persisted)
	if !strings.Contains(persistedText, "label: Team Persist Check") {
		t.Fatalf("expected persisted yaml to contain label, got:\n%s", persistedText)
	}

	loaded, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := len(loaded.GeminiKey); got != 1 {
		t.Fatalf("loaded gemini keys len = %d, want 1", got)
	}
	if got := loaded.GeminiKey[0].Label; got != "Team Persist Check" {
		t.Fatalf("loaded gemini label = %q, want %q", got, "Team Persist Check")
	}
}

func TestPatchClaudeKey_TrimsLabel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{{APIKey: "claude-key", BaseURL: "https://claude.example.com"}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/claude-api-key", strings.NewReader(`{"index":0,"value":{"label":"  Main Claude  "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchClaudeKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.ClaudeKey[0].Label; got != "Main Claude" {
		t.Fatalf("claude key label = %q, want %q", got, "Main Claude")
	}
}

func TestPatchCodexKey_BlankLabelClearsWithoutRemovingEntry(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			CodexKey: []config.CodexKey{{Label: "Old Name", APIKey: "codex-key", BaseURL: "https://codex.example.com"}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/codex-api-key", strings.NewReader(`{"index":0,"value":{"label":"   "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchCodexKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := len(h.cfg.CodexKey); got != 1 {
		t.Fatalf("codex keys len = %d, want 1", got)
	}
	if got := h.cfg.CodexKey[0].Label; got != "" {
		t.Fatalf("codex key label = %q, want empty string", got)
	}
}

func TestPatchVertexCompatKey_TrimsLabel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			VertexCompatAPIKey: []config.VertexCompatKey{{APIKey: "vertex-key", BaseURL: "https://vertex.example.com"}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/vertex-api-key", strings.NewReader(`{"index":0,"value":{"label":"  Vertex Main  "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchVertexCompatKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.VertexCompatAPIKey[0].Label; got != "Vertex Main" {
		t.Fatalf("vertex key label = %q, want %q", got, "Vertex Main")
	}
}
