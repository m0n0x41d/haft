package tools

import "context"

type activeToolCallIDKey struct{}

// WithActiveToolCallID annotates a tool execution context with the current call ID.
func WithActiveToolCallID(ctx context.Context, callID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if callID == "" {
		return ctx
	}

	return context.WithValue(ctx, activeToolCallIDKey{}, callID)
}

// ActiveToolCallID returns the current tool call ID, if one was attached.
func ActiveToolCallID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	callID, _ := ctx.Value(activeToolCallIDKey{}).(string)
	return callID
}
