package log

import (
	"context"
	"testing"
)

func TestTrace(t *testing.T) {
	ctx := context.Background()
	ctx = Trace(ctx)
	id := ctx.Value(TraceIDKey)
	if id == nil {
		t.Error("trace id is not present in ctx")
	}

	logger := Logger(ctx)
	logger.Info("test")
}
