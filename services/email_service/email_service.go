package email_service

import (
	"fmt"
	"net/smtp"

	"github.com/Henryarrovin/auth-service/config"
	"go.uber.org/zap"
)

type EmailService struct {
	cfg    config.EmailConfig
	logger *zap.Logger
}

func NewEmailService(cfg config.EmailConfig, logger *zap.Logger) *EmailService {
	return &EmailService{cfg: cfg, logger: logger}
}

func (s *EmailService) SendPasswordReset(email, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.BaseURL, token)

	subject := "Password Reset Request"
	body := fmt.Sprintf(`
		Hello,

		You requested a password reset. Click the link below to reset your password:

		%s

		This link expires in 15 minutes.

		If you did not request this, ignore this email.
		`, resetLink)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		s.cfg.From, email, subject, body)

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{email}, []byte(msg)); err != nil {
		s.logger.Error("failed to send reset email",
			zap.String("email", email),
			zap.Error(err),
		)
		return fmt.Errorf("sending reset email: %w", err)
	}

	s.logger.Info("reset email sent", zap.String("email", email))
	return nil
}
