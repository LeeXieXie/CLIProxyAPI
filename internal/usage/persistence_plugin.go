package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

const (
	persistenceDefaultInterval = 5 * time.Minute
	persistenceFileName        = "usage_stats.json"
	persistenceVersion         = 1
)

// PersistencePlugin writes the in-memory usage snapshot to disk on a regular
// interval and restores it when the server starts.  It satisfies coreusage.Plugin
// so it can be registered alongside LoggerPlugin without any changes to main.
type PersistencePlugin struct {
	stats    *RequestStatistics
	filePath string
	interval time.Duration

	dirty   atomic.Bool // true when at least one record has arrived since last flush
	stopCh  chan struct{}
	once    sync.Once
	stopped chan struct{}
}

type persistencePayload struct {
	Version   int                 `json:"version"`
	SavedAt   time.Time           `json:"saved_at"`
	Snapshot  StatisticsSnapshot  `json:"snapshot"`
}

// NewPersistencePlugin constructs a persistence plugin that saves to dir/usage_stats.json
// and flushes every interval (0 → default 5 min).
func NewPersistencePlugin(dir string, interval time.Duration) *PersistencePlugin {
	if interval <= 0 {
		interval = persistenceDefaultInterval
	}
	return &PersistencePlugin{
		stats:    defaultRequestStatistics,
		filePath: filepath.Join(dir, persistenceFileName),
		interval: interval,
		stopCh:   make(chan struct{}),
		stopped:  make(chan struct{}),
	}
}

// HandleUsage implements coreusage.Plugin.  We just mark the stats as dirty;
// the background flusher does the actual I/O.
func (p *PersistencePlugin) HandleUsage(_ context.Context, _ coreusage.Record) {
	p.dirty.Store(true)
}

// Start launches the background flush loop and must be called once after
// LoadAndMerge.  It is safe to call from an init() function.
func (p *PersistencePlugin) Start() {
	p.once.Do(func() {
		go p.loop()
	})
}

// Stop flushes one final time and waits for the goroutine to exit.
func (p *PersistencePlugin) Stop() {
	close(p.stopCh)
	<-p.stopped
}

// LoadAndMerge reads the on-disk snapshot (if any) and merges it into the
// shared RequestStatistics store.  Call this once before Start.
func (p *PersistencePlugin) LoadAndMerge() {
	data, err := os.ReadFile(p.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warnf("usage persistence: cannot read %s: %v", p.filePath, err)
		}
		return
	}

	var payload persistencePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Warnf("usage persistence: cannot parse %s: %v", p.filePath, err)
		return
	}
	if payload.Version != 0 && payload.Version != persistenceVersion {
		log.Warnf("usage persistence: unsupported version %d in %s, skipping", payload.Version, p.filePath)
		return
	}

	result := p.stats.MergeSnapshot(payload.Snapshot)
	log.Infof("usage persistence: loaded %s (saved at %s): merged %d records, skipped %d duplicates",
		p.filePath, payload.SavedAt.Format(time.RFC3339), result.Added, result.Skipped)
}

// flush writes the current snapshot to disk only when dirty.
func (p *PersistencePlugin) flush() {
	if !p.dirty.Swap(false) {
		return
	}
	snapshot := p.stats.Snapshot()
	payload := persistencePayload{
		Version:  persistenceVersion,
		SavedAt:  time.Now().UTC(),
		Snapshot: snapshot,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Warnf("usage persistence: marshal error: %v", err)
		p.dirty.Store(true) // restore dirty so we retry next tick
		return
	}
	// Atomic write: write to temp file then rename.
	tmp := p.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Warnf("usage persistence: write error: %v", err)
		p.dirty.Store(true)
		return
	}
	if err := os.Rename(tmp, p.filePath); err != nil {
		log.Warnf("usage persistence: rename error: %v", err)
		p.dirty.Store(true)
	}
}

func (p *PersistencePlugin) loop() {
	defer close(p.stopped)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.flush()
		case <-p.stopCh:
			p.dirty.Store(true) // force final flush
			p.flush()
			return
		}
	}
}

var defaultPersistencePlugin *PersistencePlugin

// InitPersistence creates, loads, and starts the global persistence plugin.
// dir is the directory to store usage_stats.json; interval controls flush frequency (0 = 5 min).
// Safe to call multiple times; only the first call takes effect.
func InitPersistence(dir string, interval time.Duration) {
	if defaultPersistencePlugin != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Warnf("usage persistence: cannot create directory %s: %v", dir, err)
		return
	}
	p := NewPersistencePlugin(dir, interval)
	p.LoadAndMerge()
	p.Start()
	coreusage.RegisterPlugin(p)
	defaultPersistencePlugin = p
	log.Infof("usage persistence: enabled, file=%s interval=%s", p.filePath, p.interval)
}

// StopPersistence flushes and stops the global persistence plugin.
func StopPersistence() {
	if p := defaultPersistencePlugin; p != nil {
		p.Stop()
	}
}
