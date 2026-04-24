package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

func BuildCanonicalString(method, path, date, service string) string {
	return strings.Join([]string{
		strings.ToUpper(method),
		path,
		date,
		service,
	}, "\n")
}

func main() {
	var token string

	fmt.Print("Enter JWT token: ")
	fmt.Scanln(&token)

	secret := os.Getenv("JWT_CANONICAL_SECRET")
	if secret == "" {
		panic("JWT_CANONICAL_SECRET not set")
	}

	method := "GET"
	path := "/api/v1/test"
	service := "test-service"
	date := time.Now().UTC().Format(time.RFC3339)

	canonicalString := BuildCanonicalString(method, path, date, service)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(canonicalString))
	signature := hex.EncodeToString(mac.Sum(nil))

	fmt.Println("\n--- Use this in your request ---")
	fmt.Println("canonical_date:", date)
	fmt.Println("canonical_sig :", signature)

	fmt.Printf(`
	{
		"token": "%s",
		"canonical_method": "%s",
		"canonical_path": "%s",
		"canonical_date": "%s",
		"service_name": "%s",
		"canonical_sig": "%s"
	}
	`, token, method, path, date, service, signature)
}
