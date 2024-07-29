package config

import (
	"fmt"
	"os"
)

type PostgresConfig struct {
	host   string
	port   string
	user   string
	passw  string
	dbname string
}

func GetConnectionString() string {
	cfg := GetPostgresConfig()
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.host, cfg.port, cfg.user, cfg.passw, cfg.dbname)
}

func GetPostgresConfig() *PostgresConfig {
	cfg := &PostgresConfig{}
	cfg.dbname = getEnv("POSTGRES_DB", "postgres")
	cfg.host = getEnv("POSTGRES_HOST", "localhost")
	cfg.port = getEnv("POSTGRES_PORT", "5433")
	cfg.user = getEnv("POSTGRES_USER", "user")
	cfg.passw = getEnv("POSTGRES_PASSWORD", "1234")
	return cfg
}

func GetHttpPort() string {
	return getEnv("SCHOOL_MATERIALS_PORT", ":8080")
}

func getEnv(name, defaultValue string) string {
	key, ok := os.LookupEnv(name)
	if !ok || key == "" {
		return defaultValue
	}
	return key
}
