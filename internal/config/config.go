package config

import (
	"log"
	"os"
)

type Config struct {
	Port        string
	PostgresDSN string
	JWTSecret   string
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	return Config{
		Port:        port,
		PostgresDSN: dsn,
		JWTSecret:   jwtSecret,
	}
}
