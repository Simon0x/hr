package guard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type contextKey struct{}

// WithContext attaches the step's situation to ctx so a harness can pass it
// to the guard process it spawns. It travels on the context rather than in
// the Harness signature because a fan-out runs several invocations at once
// in one process: process-wide state would have them overwrite each other.
func WithContext(ctx context.Context, c Context) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext returns the situation attached by WithContext.
func FromContext(ctx context.Context) (Context, bool) {
	c, ok := ctx.Value(contextKey{}).(Context)
	return c, ok
}

// NewToken identifies one invocation's denial log. Random rather than
// derived, so two concurrent leads of the same step cannot collide.
func NewToken() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic("guard: no randomness for an invocation token: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
