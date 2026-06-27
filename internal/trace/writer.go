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
		record = cloneTraceRecord(record)
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
	tmp, err := os.CreateTemp(w.dir, traceID+".*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(bytes); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	cleanup = false
	return record, nil
}

func cloneTraceRecord(record *TraceRecord) *TraceRecord {
	if record == nil {
		return nil
	}
	cloned := *record
	cloned.Input = cloneMap(record.Input)
	if len(record.Spans) > 0 {
		cloned.Spans = make([]TraceSpan, len(record.Spans))
		for i, span := range record.Spans {
			cloned.Spans[i] = TraceSpan{
				Node:      span.Node,
				Component: span.Component,
				Attempt:   span.Attempt,
				Skipped:   span.Skipped,
				LatencyMS: span.LatencyMS,
				Input:     cloneMap(span.Input),
				Output:    cloneMap(span.Output),
			}
			if span.Error != nil {
				errCopy := *span.Error
				cloned.Spans[i].Error = &errCopy
			}
		}
	}
	if record.Error != nil {
		errCopy := *record.Error
		cloned.Error = &errCopy
	}
	return &cloned
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch value := v.(type) {
		case map[string]any:
			out[k] = cloneMap(value)
		case []any:
			cloned := make([]any, len(value))
			for i, item := range value {
				cloned[i] = cloneValue(item)
			}
			out[k] = cloned
		default:
			out[k] = v
		}
	}
	return out
}

func cloneValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		cloned := make([]any, len(value))
		for i, item := range value {
			cloned[i] = cloneValue(item)
		}
		return cloned
	default:
		return v
	}
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
