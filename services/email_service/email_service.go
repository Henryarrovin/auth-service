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
	body := fmt.Sprintf(`Hello,

You requested a password reset. Click the link below to reset your password:

%s

This link expires in 15 minutes.

If you did not request this, ignore this email.
`, resetLink)

	return s.send(email, subject, body)
}

// SendVerificationOTP is used both for signup verification and resends.
func (s *EmailService) SendVerificationOTP(email, otp string) error {
	subject := "Verify your ClipLynk email"
	body := fmt.Sprintf(`Hello,

Your ClipLynk verification code is:

    %s

Enter this code in the app to verify your email address. This code expires in 10 minutes.

If you did not create a ClipLynk account, you can safely ignore this email.
`, otp)

	return s.send(email, subject, body)
}

func (s *EmailService) SendWelcomeEmail(email, name string) error {
	subject := "Welcome to ClipLynk"
	body := fmt.Sprintf(`Hi %s,

Your email is verified and your ClipLynk account is ready to go.

You can now pair your devices and start syncing your clipboard securely across them.

Thanks for signing up!
`, name)

	return s.send(email, subject, body)
}

func (s *EmailService) send(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		s.cfg.From, to, subject, body)

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg)); err != nil {
		s.logger.Error("failed to send email",
			zap.String("to", to), zap.String("subject", subject), zap.Error(err))
		return fmt.Errorf("sending email: %w", err)
	}

	s.logger.Info("email sent", zap.String("to", to), zap.String("subject", subject))
	return nil
}
