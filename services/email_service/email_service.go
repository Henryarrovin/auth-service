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
	subject := "Your ClipLynk verification code"
	html := fmt.Sprintf(`
<div style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 24px;">
  <h2 style="color: #111; margin-bottom: 8px;">ClipLynk</h2>
  <p style="color: #333; font-size: 15px;">Use the code below to verify your email address.</p>
  <div style="background: #f4f4f5; border-radius: 8px; padding: 16px; text-align: center; margin: 20px 0;">
    <span style="font-size: 28px; letter-spacing: 6px; font-weight: 600; color: #111;">%s</span>
  </div>
  <p style="color: #666; font-size: 13px;">This code expires in 10 minutes. We'll never ask you to share this code with anyone — if someone asks for it, it's a scam.</p>
  <p style="color: #666; font-size: 13px;">If you didn't request this, you can safely ignore this email.</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 24px 0;">
  <p style="color: #999; font-size: 12px;">ClipLynk · %s</p>
</div>`, otp, s.cfg.BaseURL)

	return s.sendHTML(email, subject, html)
}

func (s *EmailService) SendWelcomeEmail(email, name string) error {
	subject := "Welcome to ClipLynk"
	html := fmt.Sprintf(`
<div style="font-family: -apple-system, Segoe UI, Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 24px;">
  <h2 style="color: #111; margin-bottom: 8px;">ClipLynk</h2>
  <p style="color: #333; font-size: 15px;">Hi %s,</p>
  <p style="color: #333; font-size: 15px;">Your email is verified and your account is ready to go.</p>
  <p style="color: #333; font-size: 15px;">You can now pair your devices and start syncing your clipboard securely across them.</p>
  <p style="color: #333; font-size: 15px;">Thanks for signing up!</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 24px 0;">
  <p style="color: #999; font-size: 12px;">ClipLynk · %s</p>
</div>`, name, s.cfg.BaseURL)

	return s.sendHTML(email, subject, html)
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

func (s *EmailService) sendHTML(to, subject, htmlBody string) error {
	msgID, err := generateMessageID(s.cfg.From)
	if err != nil {
		return fmt.Errorf("generating message id: %w", err)
	}

	boundary := "clipsync-boundary-42"
	plainFallback := "Open this email in an HTML-capable mail client to view your verification code."

	msg := fmt.Sprintf(
		"From: ClipLynk <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Message-ID: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/alternative; boundary=\"%s\"\r\n"+
			"\r\n"+
			"--%s\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n"+
			"%s\r\n\r\n"+
			"--%s\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n\r\n"+
			"%s\r\n\r\n"+
			"--%s--",
		s.cfg.From, to, subject, time.Now().Format(time.RFC1123Z), msgID,
		boundary, boundary, plainFallback, boundary, htmlBody, boundary,
	)

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg)); err != nil {
		s.logger.Error("failed to send email", zap.String("to", to), zap.String("subject", subject), zap.Error(err))
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
