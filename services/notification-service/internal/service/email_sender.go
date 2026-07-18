package service

import (
	"context"
	"log/slog"

	"github.com/finora/notification-service/internal/domain"
)

// LoggingEmailSender is the only domain.EmailSender implementation that
// exists today. It sends nothing — it just logs what would have been sent —
// so the seam described in domain.EmailSender is visibly wired into the
// dependency graph (see cmd/server/main.go) ahead of a real SMTP-backed
// implementation landing in a future phase.
type LoggingEmailSender struct {
	log *slog.Logger
}

// NewLoggingEmailSender builds a no-op EmailSender that logs via log.
func NewLoggingEmailSender(log *slog.Logger) *LoggingEmailSender {
	return &LoggingEmailSender{log: log}
}

// Send logs the would-be email instead of sending it.
func (s *LoggingEmailSender) Send(_ context.Context, to, subject, body string) error {
	s.log.Info("email send skipped (no-op sender)",
		slog.String("to", to),
		slog.String("subject", subject),
		slog.Int("body_len", len(body)),
	)
	return nil
}

var _ domain.EmailSender = (*LoggingEmailSender)(nil)
