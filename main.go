package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using system env")
	}

	redisAddr := os.Getenv("AUTH_REDIS_ADDR")
	log.Println("Redis Address:", redisAddr)
}
