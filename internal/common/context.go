package common

import (
	"context"
	"time"
)

// Context keys for storing values in context
type contextKey string

const (
	ContextKeyRequestID contextKey = "request_id"
	ContextKeyProfileID contextKey = "profile_id"
	ContextKeyUserID    contextKey = "user_id"
	ContextKeyLogger    contextKey = "logger"
)

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// RequestIDFromContext extracts the request ID from context
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return requestID
	}
	return ""
}

// WithProfileID adds a profile ID to the context
func WithProfileID(ctx context.Context, profileID string) context.Context {
	return context.WithValue(ctx, ContextKeyProfileID, profileID)
}

// ProfileIDFromContext extracts the profile ID from context
func ProfileIDFromContext(ctx context.Context) string {
	if profileID, ok := ctx.Value(ContextKeyProfileID).(string); ok {
		return profileID
	}
	return ""
}

// WithTimeout creates a context with the specified timeout
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// WithDeadline creates a context with the specified deadline
func WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}
