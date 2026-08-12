package email_service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"time"

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
	msgID, err := generateMessageID(s.cfg.From)
	if err != nil {
		return fmt.Errorf("generating message id: %w", err)
	}

	msg := fmt.Sprintf(
		"From: ClipLynk <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Message-ID: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"\r\n%s",
		s.cfg.From, to, subject, time.Now().Format(time.RFC1123Z), msgID, body,
	)

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

func generateMessageID(from string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	domain := "cliplynk.local"
	if at := indexOf(from, '@'); at != -1 {
		domain = from[at+1:]
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain), nil
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
