package totp_service

import (
	"os"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewTOTPService, ProvideTOTPIssuer)

func ProvideTOTPIssuer() TOTPIssuer {
	issuer := os.Getenv("AUTH_TOTP_ISSUER")
	if issuer == "" {
		return "auth-service"
	}
	return TOTPIssuer(issuer)
}
