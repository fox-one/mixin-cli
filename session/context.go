package session

import (
	"context"

	"github.com/spf13/cobra"
)

type contextKey struct{}

func With(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

func From(ctx context.Context) *Session {
	return ctx.Value(contextKey{}).(*Session)
}

func FromCmd(cmd *cobra.Command) *Session {
	return From(cmd.Context())
}
