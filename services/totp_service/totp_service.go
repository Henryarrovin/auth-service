package totp_service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type TOTPService struct {
	issuer string
	logger *zap.Logger
}

func NewTOTPService(issuer string, logger *zap.Logger) *TOTPService {
	return &TOTPService{issuer: issuer, logger: logger}
}

// GenerateSecret creates a new TOTP secret for a user
func (s *TOTPService) GenerateSecret(email string) (string, string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: email,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("generating totp key: %w", err)
	}

	// Generate QR code image as base64
	img, err := key.Image(256, 256)
	if err != nil {
		return "", "", "", fmt.Errorf("generating qr image: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", "", fmt.Errorf("encoding qr image: %w", err)
	}

	qrBase64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	s.logger.Info("totp secret generated", zap.String("email", email))
	return key.Secret(), key.URL(), qrBase64, nil
}

// ValidateOTP checks if an OTP is valid for a secret
func (s *TOTPService) ValidateOTP(secret, code string) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})

	if err != nil {
		s.logger.Warn("otp validation error", zap.Error(err))
		return false
	}

	return valid
}

// GenerateBackupCodes generates 8 one-time use backup codes
func (s *TOTPService) GenerateBackupCodes() ([]string, string, error) {
	codes := make([]string, 8)
	hashed := make([]string, 8)

	for i := range codes {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			return nil, "", err
		}
		codes[i] = fmt.Sprintf("%x", b) // 10 char hex code

		hash, err := bcrypt.GenerateFromPassword([]byte(codes[i]), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", err
		}
		hashed[i] = string(hash)
	}

	hashedJSON, err := json.Marshal(hashed)
	if err != nil {
		return nil, "", err
	}

	return codes, string(hashedJSON), nil
}

// ValidateBackupCode checks if a backup code is valid and removes it
func (s *TOTPService) ValidateBackupCode(backupCodesJSON, code string) (bool, string, error) {
	var hashes []string
	if err := json.Unmarshal([]byte(backupCodesJSON), &hashes); err != nil {
		return false, "", err
	}

	for i, hash := range hashes {
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)); err == nil {
			// Remove used code
			hashes = append(hashes[:i], hashes[i+1:]...)
			newJSON, _ := json.Marshal(hashes)
			return true, string(newJSON), nil
		}
	}

	return false, "", nil
}
