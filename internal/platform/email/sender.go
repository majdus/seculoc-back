package email

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type EmailSender interface {
	SendInvitation(ctx context.Context, toEmail, link string) error
}

type MockEmailSender struct {
	logger *zap.Logger
}

func NewMockEmailSender(logger *zap.Logger) *MockEmailSender {
	return &MockEmailSender{logger: logger}
}

func (m *MockEmailSender) SendInvitation(ctx context.Context, toEmail, link string) error {
	// For dev/test, just log it.
	m.logger.Info("📧 MOCK EMAIL SENT 📧")
	m.logger.Info(fmt.Sprintf("To: %s", toEmail))
	m.logger.Info(fmt.Sprintf("Link: %s", link))
	m.logger.Info("---------------------------------------------------")
	return nil
}
