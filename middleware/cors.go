package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/cors"
)

func CORSHandler(handler http.Handler) http.Handler {
	allowedOrigins := []string{
		"http://localhost:8081",
		"exp://192.168.0.113:8083",
	}

	if origins := os.Getenv("AUTH_CORS_ALLOWED_ORIGINS"); origins != "" {
		allowedOrigins = strings.Split(origins, ",")
	}

	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Correlation-ID",
			"X-Requested-With",
			"Grpc-Metadata-Authorization",
		},
		ExposedHeaders: []string{
			"X-Correlation-ID",
		},
		AllowCredentials: true,
		MaxAge:           86400,
		Debug:            os.Getenv("AUTH_SERVER_ENV") == "development",
	})

	return c.Handler(handler)
}
