package usage

import (
	"context"
	"os"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestSQLitePlugin_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := NewSQLitePersistencePlugin(dir, time.Hour, 90)
	if err != nil {
		t.Fatalf("new plugin: %v", err)
	}
	defer p.Stop()

	stats := NewRequestStatistics()
	p.stats = stats

	// Inject two records via Record()
	stats.Record(context.Background(), coreusage.Record{
		Provider: "claude",
		Model:    "claude-3-5-sonnet",
		Detail:   coreusage.Detail{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
		Latency:  200 * time.Millisecond,
	})
	stats.Record(context.Background(), coreusage.Record{
		Provider: "claude",
		Model:    "claude-3-5-sonnet",
		Failed:   true,
		Latency:  50 * time.Millisecond,
	})

	// Flush to SQLite
	p.dirty.Store(true)
	p.flush()

	// Load into a fresh store and verify counts
	p2, err := NewSQLitePersistencePlugin(dir, time.Hour, 90)
	if err != nil {
		t.Fatalf("new plugin 2: %v", err)
	}
	defer p2.Stop()

	stats2 := NewRequestStatistics()
	p2.stats = stats2
	p2.LoadAndMerge()

	snap := stats2.Snapshot()
	if snap.TotalRequests != 2 {
		t.Errorf("want 2 total requests, got %d", snap.TotalRequests)
	}
	if snap.FailureCount != 1 {
		t.Errorf("want 1 failure, got %d", snap.FailureCount)
	}
	if snap.TotalTokens != 150 {
		t.Errorf("want 150 total tokens, got %d", snap.TotalTokens)
	}
}

func TestSQLitePlugin_Prune(t *testing.T) {
	dir := t.TempDir()
	p, err := NewSQLitePersistencePlugin(dir, time.Hour, 1) // 1-day retention
	if err != nil {
		t.Fatalf("new plugin: %v", err)
	}
	defer p.Stop()

	stats := NewRequestStatistics()
	p.stats = stats

	stats.Record(context.Background(), coreusage.Record{
		Provider: "gemini",
		Model:    "gemini-2.0-flash",
		Detail:   coreusage.Detail{TotalTokens: 10},
	})

	p.dirty.Store(true)
	p.flush()

	// Manually insert an old row (3 days ago)
	old := time.Now().UTC().AddDate(0, 0, -3).UnixMilli()
	_, _ = p.db.Exec(`INSERT INTO request_details
		(api_key, model, ts, latency_ms, total_tok) VALUES (?,?,?,?,?)`,
		"gemini", "gemini-2.0-flash", old, 10, 5)

	rowsBefore := 0
	_ = p.db.QueryRow("SELECT COUNT(*) FROM request_details").Scan(&rowsBefore)

	// Flush triggers prune (maxAgeDays=1, old row is 3 days old)
	p.dirty.Store(true)
	p.flush()

	rowsAfter := 0
	_ = p.db.QueryRow("SELECT COUNT(*) FROM request_details").Scan(&rowsAfter)

	if rowsAfter >= rowsBefore {
		t.Errorf("prune should have reduced rows: before=%d after=%d", rowsBefore, rowsAfter)
	}
}

func TestSQLitePlugin_HandleUsage_SetsDirty(t *testing.T) {
	dir := t.TempDir()
	p, err := NewSQLitePersistencePlugin(dir, time.Hour, 0)
	if err != nil {
		t.Fatalf("new plugin: %v", err)
	}
	defer p.Stop()

	if p.dirty.Load() {
		t.Fatal("should not be dirty before HandleUsage")
	}
	p.HandleUsage(context.Background(), coreusage.Record{})
	if !p.dirty.Load() {
		t.Fatal("should be dirty after HandleUsage")
	}
}

func TestSQLitePlugin_FallbackToJSON(t *testing.T) {
	// Passing a non-writable path should cause InitPersistenceDB to fall back
	// gracefully without panicking.
	dir := t.TempDir()
	roDir := dir + "/readonly"
	if err := os.MkdirAll(roDir, 0o000); err != nil {
		t.Skip("cannot create unwritable dir")
	}
	defer os.Chmod(roDir, 0o700) //nolint:errcheck

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("InitPersistenceDB panicked: %v", r)
		}
	}()
	// Reset singletons for isolated test
	prev := defaultSQLitePlugin
	prevJSON := defaultPersistencePlugin
	defaultSQLitePlugin = nil
	defaultPersistencePlugin = nil
	defer func() {
		if defaultSQLitePlugin != nil {
			defaultSQLitePlugin.Stop()
		}
		if defaultPersistencePlugin != nil {
			defaultPersistencePlugin.Stop()
		}
		defaultSQLitePlugin = prev
		defaultPersistencePlugin = prevJSON
	}()

	// Use a writable temp dir for fallback
	InitPersistenceDB(dir, time.Hour, 30)
}
