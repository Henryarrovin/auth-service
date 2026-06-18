package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/cors"
)

func CORSHandler(handler http.Handler) http.Handler {
	allowedOrigins := getAllowedOrigins()

	allowCredentials := true
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		allowCredentials = false
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"X-Correlation-ID"},
		AllowCredentials: allowCredentials,
		MaxAge:           86400,
		Debug:            os.Getenv("AUTH_SERVER_ENV") == "development",
	})

	return c.Handler(handler)
}

// reads from env or defaults to allow all.
func getAllowedOrigins() []string {
	origins := os.Getenv("AUTH_CORS_ALLOWED_ORIGINS")
	if origins == "" {
		return []string{"*"}
	}
	result := []string{}
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			result = append(result, o)
		}
	}
	return result
}
