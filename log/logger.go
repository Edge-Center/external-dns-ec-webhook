package log

import (
	"context"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	TraceIDKey = "trace_id"
	ErrorKey   = "error"
	DNSNameKey = "dns_name"
	DryRunKey  = "dry_run"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

// Trace adds tracing ID to ctx
func Trace(ctx context.Context) context.Context {
	traceId, err := uuid.NewRandom()
	if err != nil {
		logrus.Errorf("failed to generate UUID for tracing: %s", err)
	}
	return context.WithValue(ctx, TraceIDKey, traceId)
}

// Logger provides logger with embedded ctx in it
func Logger(ctx context.Context) *logrus.Entry {
	return logrus.StandardLogger().WithContext(ctx).WithField(TraceIDKey, ctx.Value(TraceIDKey))
}
