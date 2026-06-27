package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fde-support/internal/registry"
)

type TriggerSpec struct {
	Type       string `json:"type"`
	Sensor     string `json:"sensor,omitempty"`
	SignalType string `json:"signalType,omitempty"`
}

type TraceWriter interface {
	Start(ctx context.Context, meta TraceRecord) (*TraceRecord, error)
	AppendSpan(ctx context.Context, traceID string, span TraceSpan) error
	Finish(ctx context.Context, traceID string, status string, errSummary *RuntimeErrorSummary, latency time.Duration) (*TraceRecord, error)
	WriteImmediate(ctx context.Context, record TraceRecord) (*TraceRecord, error)
}

type RuntimeErrorSummary = registry.RuntimeErrorSummary

type TraceRecord struct {
	TraceID     string               `json:"traceId"`
	Solution    string               `json:"solution"`
	Version     string               `json:"version"`
	Environment string               `json:"environment"`
	Trigger     TriggerSpec          `json:"trigger"`
	Input       map[string]any       `json:"input"`
	Spans       []TraceSpan          `json:"spans"`
	LatencyMS   int64                `json:"latencyMs"`
	Status      string               `json:"status"`
	Error       *RuntimeErrorSummary `json:"error"`
	Duplicate   bool                 `json:"duplicate,omitempty"`
	CreatedAt   time.Time            `json:"createdAt"`
}

type TraceSpan struct {
	Node      string               `json:"node"`
	Component string               `json:"component"`
	Attempt   int                  `json:"attempt,omitempty"`
	Skipped   bool                 `json:"skipped,omitempty"`
	LatencyMS int64                `json:"latencyMs"`
	Input     map[string]any       `json:"input,omitempty"`
	Output    map[string]any       `json:"output,omitempty"`
	Error     *RuntimeErrorSummary `json:"error,omitempty"`
}

type FileTraceWriter struct {
	dir     string
	mu      sync.Mutex
	records map[string]*TraceRecord
}

func NewFileTraceWriter(dir string) *FileTraceWriter {
	return &FileTraceWriter{dir: dir, records: map[string]*TraceRecord{}}
}

func (w *FileTraceWriter) Start(ctx context.Context, meta TraceRecord) (*TraceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	meta.TraceID = newTraceID()
	meta.CreatedAt = time.Now().UTC()
	meta.Status = "running"
	w.mu.Lock()
	w.records[meta.TraceID] = &meta
	w.mu.Unlock()
	return &meta, nil
}

func (w *FileTraceWriter) AppendSpan(ctx context.Context, traceID string, span TraceSpan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if record, ok := w.records[traceID]; ok {
		record.Spans = append(record.Spans, span)
	}
	return nil
}

func (w *FileTraceWriter) Finish(ctx context.Context, traceID string, status string, errSummary *RuntimeErrorSummary, latency time.Duration) (*TraceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w.mu.Lock()
	record := w.records[traceID]
	if record != nil {
		record.Status = status
		record.Error = errSummary
		record.LatencyMS = latency.Milliseconds()
	}
	w.mu.Unlock()
	if record == nil {
		return nil, nil
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return nil, err
	}
	bytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(w.dir, traceID+".json")
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return nil, err
	}
	return record, nil
}

func (w *FileTraceWriter) WriteImmediate(ctx context.Context, record TraceRecord) (*TraceRecord, error) {
	start := time.Now()
	r, err := w.Start(ctx, record)
	if err != nil {
		return nil, err
	}
	status := record.Status
	if status == "" {
		status = "failed"
	}
	return w.Finish(ctx, r.TraceID, status, record.Error, time.Since(start))
}

func (w *FileTraceWriter) List(ctx context.Context, limit int) ([]TraceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []TraceRecord{}, nil
		}
		return nil, err
	}

	type listEntry struct {
		record  TraceRecord
		modTime time.Time
	}
	list := make([]listEntry, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		bytes, err := os.ReadFile(filepath.Join(w.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var record TraceRecord
		if err := json.Unmarshal(bytes, &record); err != nil {
			return nil, err
		}
		list = append(list, listEntry{record: record, modTime: info.ModTime()})
	}

	sort.Slice(list, func(i, j int) bool {
		left := list[i].record.CreatedAt
		if left.IsZero() {
			left = list[i].modTime
		}
		right := list[j].record.CreatedAt
		if right.IsZero() {
			right = list[j].modTime
		}
		if left.Equal(right) {
			return list[i].record.TraceID > list[j].record.TraceID
		}
		return left.After(right)
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}

	records := make([]TraceRecord, len(list))
	for i, entry := range list {
		records[i] = entry.record
	}
	return records, nil
}

func (w *FileTraceWriter) Load(ctx context.Context, traceID string) (*TraceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if traceID == "" || strings.ContainsAny(traceID, `/\`) {
		return nil, os.ErrNotExist
	}
	bytes, err := os.ReadFile(filepath.Join(w.dir, traceID+".json"))
	if err != nil {
		return nil, err
	}
	var record TraceRecord
	if err := json.Unmarshal(bytes, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func newTraceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "trace_" + hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return "trace_" + hex.EncodeToString(b[:])
}
