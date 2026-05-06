package management

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/redisqueue"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
)

type usageExportPayload struct {
	Version    int                      `json:"version"`
	ExportedAt time.Time                `json:"exported_at"`
	Usage      usage.StatisticsSnapshot `json:"usage"`
}

type usageImportPayload struct {
	Version int                      `json:"version"`
	Usage   usage.StatisticsSnapshot `json:"usage"`
}

type usageQueueRecord []byte

func (r usageQueueRecord) MarshalJSON() ([]byte, error) {
	if json.Valid(r) {
		return append([]byte(nil), r...), nil
	}
	return json.Marshal(string(r))
}

// GetUsageStatistics returns the in-memory request statistics snapshot.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, gin.H{
		"usage":           snapshot,
		"failed_requests": snapshot.FailureCount,
	})
}

// ExportUsageStatistics returns a complete usage snapshot for backup/migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, usageExportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Usage:      snapshot,
	})
}

// GetUsageDetails returns a flat list of all individual request details,
// optionally filtered by ?api=...&model=...&limit=N.
func (h *Handler) GetUsageDetails(c *gin.Context) {
	if h == nil || h.usageStats == nil {
		c.JSON(http.StatusOK, gin.H{"details": []struct{}{}})
		return
	}
	apiFilter := c.Query("api")
	modelFilter := c.Query("model")
	limitStr := c.DefaultQuery("limit", "500")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 10000 {
		limit = 500
	}

	snap := h.usageStats.Snapshot()

	type detailItem struct {
		API       string           `json:"api"`
		Model     string           `json:"model"`
		Timestamp time.Time        `json:"timestamp"`
		LatencyMs int64            `json:"latency_ms"`
		Source    string           `json:"source"`
		AuthIndex string           `json:"auth_index"`
		Failed    bool             `json:"failed"`
		Tokens    usage.TokenStats `json:"tokens"`
	}

	var items []detailItem
	for apiKey, apiSnap := range snap.APIs {
		if apiFilter != "" && apiKey != apiFilter {
			continue
		}
		for model, modelSnap := range apiSnap.Models {
			if modelFilter != "" && model != modelFilter {
				continue
			}
			for _, d := range modelSnap.Details {
				items = append(items, detailItem{
					API:       apiKey,
					Model:     model,
					Timestamp: d.Timestamp,
					LatencyMs: d.LatencyMs,
					Source:    d.Source,
					AuthIndex: d.AuthIndex,
					Failed:    d.Failed,
					Tokens:    d.Tokens,
				})
			}
		}
	}

	// Preserve the previous newest-first behavior for the flattened detail list.
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	if len(items) > limit {
		items = items[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(items),
		"limit":   limit,
		"details": items,
	})
}

// GetUsageRetention returns the current retention days setting.
func (h *Handler) GetUsageRetention(c *gin.Context) {
	days := 90
	if h != nil && h.cfg != nil {
		if h.cfg.UsageRetentionDays > 0 {
			days = h.cfg.UsageRetentionDays
		}
	}
	c.JSON(http.StatusOK, gin.H{"retention_days": days})
}

// PutUsageRetention updates the retention days setting and persists it to config.yaml.
func (h *Handler) PutUsageRetention(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "config unavailable"})
		return
	}
	var body struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if body.RetentionDays < 1 || body.RetentionDays > 3650 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "retention_days must be between 1 and 3650"})
		return
	}
	h.cfg.UsageRetentionDays = body.RetentionDays
	if err := config.SaveConfigPreserveComments(h.configFilePath, h.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}
	usage.UpdateRetentionDays(body.RetentionDays)
	c.JSON(http.StatusOK, gin.H{"retention_days": body.RetentionDays})
}

// ImportUsageStatistics merges a previously exported usage snapshot into memory.
func (h *Handler) ImportUsageStatistics(c *gin.Context) {
	if h == nil || h.usageStats == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage statistics unavailable"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload usageImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Version != 0 && payload.Version != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported version"})
		return
	}

	result := h.usageStats.MergeSnapshot(payload.Usage)
	snapshot := h.usageStats.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"added":           result.Added,
		"skipped":         result.Skipped,
		"total_requests":  snapshot.TotalRequests,
		"failed_requests": snapshot.FailureCount,
	})
}

// GetUsageQueue pops queued usage records from the usage queue.
func (h *Handler) GetUsageQueue(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	count, errCount := parseUsageQueueCount(c.Query("count"))
	if errCount != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errCount.Error()})
		return
	}

	items := redisqueue.PopOldest(count)
	records := make([]usageQueueRecord, 0, len(items))
	for _, item := range items {
		records = append(records, usageQueueRecord(append([]byte(nil), item...)))
	}

	c.JSON(http.StatusOK, records)
}

func parseUsageQueueCount(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, nil
	}
	count, errCount := strconv.Atoi(value)
	if errCount != nil || count <= 0 {
		return 0, errors.New("count must be a positive integer")
	}
	return count, nil
}
