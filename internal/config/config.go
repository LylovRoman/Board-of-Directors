package config

import (
	"log"
	"os"
)

type Config struct {
	Port        string
	PostgresDSN string
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

	return Config{
		Port:        port,
		PostgresDSN: dsn,
	}
}

