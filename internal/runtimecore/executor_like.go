package runtimecore

import (
	"context"

	"fde-support/internal/trace"
)

type ExecutorLike interface {
	Execute(ctx context.Context, req RuntimeRequest) (map[string]any, *trace.TraceRecord, error)
}
