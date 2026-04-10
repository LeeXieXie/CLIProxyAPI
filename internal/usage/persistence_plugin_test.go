package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

// newTestPlugin builds a persistence plugin backed by a fresh RequestStatistics
// instance writing to a temp directory, so tests are fully isolated.
func newTestPlugin(t *testing.T, interval time.Duration) (*PersistencePlugin, *RequestStatistics, string) {
	t.Helper()
	dir := t.TempDir()
	stats := NewRequestStatistics()
	p := &PersistencePlugin{
		stats:    stats,
		filePath: filepath.Join(dir, persistenceFileName),
		interval: interval,
		stopCh:   make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	return p, stats, dir
}

func TestPersistencePlugin_HandleUsage_SetsDirty(t *testing.T) {
	p, _, _ := newTestPlugin(t, time.Minute)
	if p.dirty.Load() {
		t.Fatal("dirty should be false before any record")
	}
	p.HandleUsage(context.Background(), coreusage.Record{})
	if !p.dirty.Load() {
		t.Fatal("dirty should be true after HandleUsage")
	}
}

func TestPersistencePlugin_FlushWritesFile(t *testing.T) {
	p, stats, dir := newTestPlugin(t, time.Minute)

	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "key1",
		Model:       "model-a",
		RequestedAt: ts,
		Detail:      coreusage.Detail{InputTokens: 5, OutputTokens: 10, TotalTokens: 15},
	})
	p.dirty.Store(true)
	p.flush()

	path := filepath.Join(dir, persistenceFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist after flush: %v", err)
	}

	var payload persistencePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("cannot unmarshal persisted file: %v", err)
	}
	if payload.Version != persistenceVersion {
		t.Errorf("version = %d, want %d", payload.Version, persistenceVersion)
	}
	apiSnap, ok := payload.Snapshot.APIs["key1"]
	if !ok {
		t.Fatal("api key1 missing from persisted snapshot")
	}
	modelSnap, ok := apiSnap.Models["model-a"]
	if !ok {
		t.Fatal("model-a missing from persisted snapshot")
	}
	if modelSnap.TotalTokens != 15 {
		t.Errorf("total_tokens = %d, want 15", modelSnap.TotalTokens)
	}
}

func TestPersistencePlugin_FlushSkipsWhenNotDirty(t *testing.T) {
	p, _, dir := newTestPlugin(t, time.Minute)
	p.dirty.Store(false)
	p.flush()

	path := filepath.Join(dir, persistenceFileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not be created when not dirty")
	}
}

func TestPersistencePlugin_LoadAndMerge(t *testing.T) {
	p, stats, _ := newTestPlugin(t, time.Minute)

	// Write a snapshot file manually.
	ts := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)
	payload := persistencePayload{
		Version: persistenceVersion,
		SavedAt: time.Now().UTC(),
		Snapshot: StatisticsSnapshot{
			TotalRequests: 1,
			SuccessCount:  1,
			TotalTokens:   42,
			APIs: map[string]APISnapshot{
				"saved-key": {
					TotalRequests: 1,
					TotalTokens:   42,
					Models: map[string]ModelSnapshot{
						"saved-model": {
							TotalRequests: 1,
							TotalTokens:   42,
							Details: []RequestDetail{{
								Timestamp: ts,
								LatencyMs: 200,
								Source:    "tester",
								AuthIndex: "0",
								Tokens:    TokenStats{InputTokens: 12, OutputTokens: 30, TotalTokens: 42},
							}},
						},
					},
				},
			},
			RequestsByDay:  map[string]int64{"2025-06-01": 1},
			RequestsByHour: map[string]int64{"08": 1},
			TokensByDay:    map[string]int64{"2025-06-01": 42},
			TokensByHour:   map[string]int64{"08": 42},
		},
	}
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(p.filePath, data, 0o600); err != nil {
		t.Fatalf("cannot write test fixture: %v", err)
	}

	p.LoadAndMerge()

	snap := stats.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("total_requests = %d, want 1", snap.TotalRequests)
	}
	if snap.TotalTokens != 42 {
		t.Errorf("total_tokens = %d, want 42", snap.TotalTokens)
	}
}

func TestPersistencePlugin_LoadAndMerge_MissingFileIsNoop(t *testing.T) {
	p, stats, _ := newTestPlugin(t, time.Minute)
	// filePath does not exist — should not panic or error
	p.LoadAndMerge()
	if snap := stats.Snapshot(); snap.TotalRequests != 0 {
		t.Errorf("expected empty stats after noop load, got %d requests", snap.TotalRequests)
	}
}

func TestPersistencePlugin_RoundTrip(t *testing.T) {
	p, stats, _ := newTestPlugin(t, time.Minute)

	ts := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "rt-key",
		Model:       "rt-model",
		RequestedAt: ts,
		Detail:      coreusage.Detail{InputTokens: 100, OutputTokens: 200, TotalTokens: 300},
	})
	p.dirty.Store(true)
	p.flush()

	// New plugin, new stats — simulate a server restart by loading into fresh stats.
	p2, stats2, _ := newTestPlugin(t, time.Minute)
	p2.filePath = p.filePath
	p2.stats = stats2
	p2.LoadAndMerge()

	snap := stats2.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("after reload total_requests = %d, want 1", snap.TotalRequests)
	}
	if snap.TotalTokens != 300 {
		t.Errorf("after reload total_tokens = %d, want 300", snap.TotalTokens)
	}
}

func TestPersistencePlugin_StopFlushesData(t *testing.T) {
	p, stats, dir := newTestPlugin(t, 10*time.Second)

	stats.Record(context.Background(), coreusage.Record{
		APIKey:      "stop-key",
		Model:       "stop-model",
		RequestedAt: time.Now(),
		Detail:      coreusage.Detail{TotalTokens: 7},
	})
	p.dirty.Store(true)

	p.Start()
	p.Stop()

	path := filepath.Join(dir, persistenceFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file should be written on Stop()")
	}
}
