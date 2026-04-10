package usage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	coreusage "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
	_ "modernc.org/sqlite" // pure-Go SQLite driver — no CGO, no user install required
)

const (
	sqliteFileName      = "usage_stats.db"
	sqliteFlushInterval = 5 * time.Minute
)

// SQLitePersistencePlugin persists every individual request detail to a local
// SQLite database.  It satisfies coreusage.Plugin and is registered alongside
// (or in place of) the JSON PersistencePlugin.
//
// Pure-Go driver via modernc.org/sqlite: zero CGO, no system libraries needed,
// cross-platform binary works out of the box.
//
// Data volume concern: only the per-request detail rows grow.  A WAL-mode SQLite
// file with 100 k requests is roughly 20–30 MB.  A built-in auto-prune keeps
// rows older than maxAgeDays (default 90) to cap file growth.
type SQLitePersistencePlugin struct {
	stats      *RequestStatistics
	db         *sql.DB
	dbPath     string
	interval   time.Duration
	maxAgeDays int // rows older than this are pruned on each flush; 0 = keep all

	dirty   atomic.Bool
	started atomic.Bool
	stopCh  chan struct{}
	once    sync.Once
	stopped chan struct{}
}

// NewSQLitePersistencePlugin opens (or creates) dir/usage_stats.db and runs
// schema migrations.  interval 0 → default 5 min.  maxAgeDays 0 → no pruning.
func NewSQLitePersistencePlugin(dir string, interval time.Duration, maxAgeDays int) (*SQLitePersistencePlugin, error) {
	if interval <= 0 {
		interval = sqliteFlushInterval
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 90 // default: keep 90 days
	}
	dbPath := filepath.Join(dir, sqliteFileName)
	// WAL mode + normal sync = safe and fast; busy_timeout avoids "database is locked"
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single-writer for SQLite
	if err := sqliteMigrateSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLitePersistencePlugin{
		stats:      defaultRequestStatistics,
		db:         db,
		dbPath:     dbPath,
		interval:   interval,
		maxAgeDays: maxAgeDays,
		stopCh:     make(chan struct{}),
		stopped:    make(chan struct{}),
	}, nil
}

// HandleUsage implements coreusage.Plugin — sets the dirty flag so the flush
// loop persists the next snapshot.
func (p *SQLitePersistencePlugin) HandleUsage(_ context.Context, _ coreusage.Record) {
	p.dirty.Store(true)
}

// Start launches the background flush goroutine.  Safe to call once.
func (p *SQLitePersistencePlugin) Start() {
	p.once.Do(func() {
		p.started.Store(true)
		go p.loop()
	})
}

// Stop does a final flush and waits for the goroutine to exit, then closes the DB.
// Safe to call even if Start was never called.
func (p *SQLitePersistencePlugin) Stop() {
	if p.started.Load() {
		close(p.stopCh)
		<-p.stopped
	} else {
		// goroutine never started — do a synchronous final flush
		p.dirty.Store(true)
		p.flush()
	}
	p.db.Close()
}

// LoadAndMerge reads all rows from the SQLite file and merges them into the
// shared in-memory RequestStatistics store (deduplication-safe).
func (p *SQLitePersistencePlugin) LoadAndMerge() {
	snap, err := sqliteLoadSnapshot(p.db)
	if err != nil {
		log.Warnf("usage sqlite: load error: %v", err)
		return
	}
	result := p.stats.MergeSnapshot(snap)
	log.Infof("usage sqlite: loaded %s: merged %d records, skipped %d duplicates",
		p.dbPath, result.Added, result.Skipped)
}

// flush writes the current snapshot to SQLite and prunes old rows.
func (p *SQLitePersistencePlugin) flush() {
	if !p.dirty.Swap(false) {
		return
	}
	snap := p.stats.Snapshot()
	if err := sqliteSaveSnapshot(p.db, snap); err != nil {
		log.Warnf("usage sqlite: flush error: %v", err)
		p.dirty.Store(true) // retry next tick
		return
	}
	if p.maxAgeDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -p.maxAgeDays).UnixMilli()
		if _, err := p.db.Exec("DELETE FROM request_details WHERE ts < ?", cutoff); err != nil {
			log.Warnf("usage sqlite: prune error: %v", err)
		}
	}
}

func (p *SQLitePersistencePlugin) loop() {
	defer close(p.stopped)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.flush()
		case <-p.stopCh:
			p.dirty.Store(true)
			p.flush()
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func sqliteMigrateSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS request_details (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    api_key     TEXT    NOT NULL,
    model       TEXT    NOT NULL,
    ts          INTEGER NOT NULL,
    latency_ms  INTEGER NOT NULL DEFAULT 0,
    source      TEXT    NOT NULL DEFAULT '',
    auth_index  TEXT    NOT NULL DEFAULT '',
    input_tok   INTEGER NOT NULL DEFAULT 0,
    output_tok  INTEGER NOT NULL DEFAULT 0,
    reason_tok  INTEGER NOT NULL DEFAULT 0,
    cached_tok  INTEGER NOT NULL DEFAULT 0,
    total_tok   INTEGER NOT NULL DEFAULT 0,
    failed      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_rd_api_model ON request_details(api_key, model);
CREATE INDEX IF NOT EXISTS idx_rd_ts        ON request_details(ts);
`)
	return err
}

// ---------------------------------------------------------------------------
// Persistence helpers
// ---------------------------------------------------------------------------

// sqliteSaveSnapshot overwrites the DB with the current snapshot in one transaction.
// Truncate-and-reinsert keeps the file compact and avoids cumulative duplicates.
func sqliteSaveSnapshot(db *sql.DB, snap StatisticsSnapshot) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec("DELETE FROM request_details"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
INSERT INTO request_details
    (api_key, model, ts, latency_ms, source, auth_index,
     input_tok, output_tok, reason_tok, cached_tok, total_tok, failed)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for apiKey, apiSnap := range snap.APIs {
		for model, modelSnap := range apiSnap.Models {
			for _, d := range modelSnap.Details {
				failedInt := 0
				if d.Failed {
					failedInt = 1
				}
				if _, err := stmt.Exec(
					apiKey, model,
					d.Timestamp.UnixMilli(), d.LatencyMs,
					d.Source, d.AuthIndex,
					d.Tokens.InputTokens, d.Tokens.OutputTokens,
					d.Tokens.ReasoningTokens, d.Tokens.CachedTokens,
					d.Tokens.TotalTokens, failedInt,
				); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// sqliteLoadSnapshot rebuilds a StatisticsSnapshot from all rows in the DB.
func sqliteLoadSnapshot(db *sql.DB) (StatisticsSnapshot, error) {
	rows, err := db.Query(`
SELECT api_key, model, ts, latency_ms, source, auth_index,
       input_tok, output_tok, reason_tok, cached_tok, total_tok, failed
FROM request_details ORDER BY ts ASC`)
	if err != nil {
		return StatisticsSnapshot{}, err
	}
	defer rows.Close()

	snap := StatisticsSnapshot{
		APIs:           make(map[string]APISnapshot),
		RequestsByDay:  make(map[string]int64),
		RequestsByHour: make(map[string]int64),
		TokensByDay:    make(map[string]int64),
		TokensByHour:   make(map[string]int64),
	}
	for rows.Next() {
		var (
			apiKey, model, source, authIndex    string
			tsMilli, latencyMs                  int64
			inTok, outTok, reTok, caTok, totTok int64
			failedInt                           int
		)
		if err := rows.Scan(&apiKey, &model, &tsMilli, &latencyMs,
			&source, &authIndex,
			&inTok, &outTok, &reTok, &caTok, &totTok, &failedInt); err != nil {
			return snap, err
		}
		ts := time.UnixMilli(tsMilli).UTC()
		d := RequestDetail{
			Timestamp: ts, LatencyMs: latencyMs,
			Source: source, AuthIndex: authIndex,
			Failed: failedInt != 0,
			Tokens: TokenStats{
				InputTokens: inTok, OutputTokens: outTok,
				ReasoningTokens: reTok, CachedTokens: caTok, TotalTokens: totTok,
			},
		}

		snap.TotalRequests++
		if d.Failed {
			snap.FailureCount++
		} else {
			snap.SuccessCount++
		}
		snap.TotalTokens += totTok

		dayKey := ts.Format("2006-01-02")
		hourKey := ts.Format("2006-01-02T15")
		snap.RequestsByDay[dayKey]++
		snap.RequestsByHour[hourKey]++
		snap.TokensByDay[dayKey] += totTok
		snap.TokensByHour[hourKey] += totTok

		apiSnap := snap.APIs[apiKey]
		if apiSnap.Models == nil {
			apiSnap.Models = make(map[string]ModelSnapshot)
		}
		apiSnap.TotalRequests++
		apiSnap.TotalTokens += totTok

		ms := apiSnap.Models[model]
		ms.TotalRequests++
		ms.TotalTokens += totTok
		ms.Details = append(ms.Details, d)
		apiSnap.Models[model] = ms
		snap.APIs[apiKey] = apiSnap
	}
	return snap, rows.Err()
}

// ---------------------------------------------------------------------------
// Global singleton
// ---------------------------------------------------------------------------

var defaultSQLitePlugin *SQLitePersistencePlugin

// InitPersistenceDB creates, loads, and starts the SQLite persistence plugin.
// It is a drop-in replacement for InitPersistence; call one or the other.
// If SQLite initialisation fails, it gracefully falls back to JSON.
// interval 0 → default 5 min.  maxAgeDays 0 → default 90-day rolling window.
func InitPersistenceDB(dir string, interval time.Duration, maxAgeDays int) {
	if defaultSQLitePlugin != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Warnf("usage sqlite: cannot create directory %s: %v", dir, err)
		return
	}
	p, err := NewSQLitePersistencePlugin(dir, interval, maxAgeDays)
	if err != nil {
		log.Warnf("usage sqlite: init error: %v — falling back to JSON persistence", err)
		InitPersistence(dir, interval)
		return
	}
	p.LoadAndMerge()
	p.Start()
	coreusage.RegisterPlugin(p)
	defaultSQLitePlugin = p
	log.Infof("usage sqlite: enabled, db=%s interval=%s retention=%d days",
		p.dbPath, p.interval, p.maxAgeDays)
}

// StopPersistenceDB flushes and stops the SQLite plugin.
func StopPersistenceDB() {
	if p := defaultSQLitePlugin; p != nil {
		p.Stop()
	}
}
