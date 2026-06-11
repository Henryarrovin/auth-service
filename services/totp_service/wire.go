package totp_service

import (
	"os"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewTOTPService, ProvideTOTPIssuer)

func ProvideTOTPIssuer() string {
	issuer := os.Getenv("AUTH_TOTP_ISSUER")
	if issuer == "" {
		return "auth-service"
	}
	return issuer
}
