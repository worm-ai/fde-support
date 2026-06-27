package w2a

import (
	"context"
	"sync"
	"time"
)

type SignalIdempotencyKey struct {
	Environment string
	SensorID    string
	SignalID    string
}

type IdempotencyRecord struct {
	Response   map[string]any
	HTTPStatus int
	StoredAt   time.Time
	ExpiresAt  time.Time
}

type SignalIdempotencyStore interface {
	Get(ctx context.Context, key SignalIdempotencyKey) (*IdempotencyRecord, bool, error)
	Put(ctx context.Context, key SignalIdempotencyKey, record IdempotencyRecord, ttl time.Duration) error
}

type MemorySignalIdempotencyStore struct {
	mu      sync.Mutex
	records map[SignalIdempotencyKey]IdempotencyRecord
	now     func() time.Time
}

func NewMemorySignalIdempotencyStore() *MemorySignalIdempotencyStore {
	return &MemorySignalIdempotencyStore{records: map[SignalIdempotencyKey]IdempotencyRecord{}, now: time.Now}
}

func (s *MemorySignalIdempotencyStore) Get(ctx context.Context, key SignalIdempotencyKey) (*IdempotencyRecord, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[key]
	if !ok {
		return nil, false, nil
	}
	if s.now().After(record.ExpiresAt) {
		delete(s.records, key)
		return nil, false, nil
	}
	return &record, true, nil
}

func (s *MemorySignalIdempotencyStore) Put(ctx context.Context, key SignalIdempotencyKey, record IdempotencyRecord, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := s.now()
	record.StoredAt = now
	record.ExpiresAt = now.Add(ttl)
	s.mu.Lock()
	s.records[key] = record
	s.mu.Unlock()
	return nil
}
