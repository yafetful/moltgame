package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	APIPort    string
	WSPort     string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	RedisAddr  string
	NATSAddr   string

	// Twitter OAuth 2.0
	TwitterClientID     string
	TwitterClientSecret string
	TwitterCallbackURL  string

	// Twitter API (OAuth 1.0a — for tweet verification)
	TwitterAPIKey            string
	TwitterAPIKeySecret      string
	TwitterAccessToken       string
	TwitterAccessTokenSecret string

	// JWT
	JWTSecret string
}

func Load() *Config {
	// Load .env from project root (best-effort, ignores if not found)
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	return &Config{
		APIPort:    getEnv("API_PORT", "8080"),
		WSPort:     getEnv("WS_PORT", "8081"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "moltgame"),
		DBPassword: getEnv("DB_PASSWORD", "moltgame"),
		DBName:     getEnv("DB_NAME", "moltgame"),
		RedisAddr:  getEnv("REDIS_ADDR", "localhost:6379"),
		NATSAddr:   getEnv("NATS_ADDR", "nats://localhost:4222"),

		TwitterClientID:          getEnv("TWITTER_CLIENT_ID", ""),
		TwitterClientSecret:      getEnv("TWITTER_CLIENT_SECRET", ""),
		TwitterCallbackURL:       getEnv("TWITTER_CALLBACK_URL", "http://localhost:3000/api/auth/twitter/callback"),
		TwitterAPIKey:            getEnv("TWITTER_API_KEY", ""),
		TwitterAPIKeySecret:      getEnv("TWITTER_API_KEY_SECRET", ""),
		TwitterAccessToken:       getEnv("TWITTER_ACCESS_TOKEN", ""),
		TwitterAccessTokenSecret: getEnv("TWITTER_ACCESS_TOKEN_SECRET", ""),

		JWTSecret: getEnv("JWT_SECRET", "moltgame-dev-secret-change-in-prod"),
	}
}

func (c *Config) DatabaseURL() string {
	return "postgres://" + c.DBUser + ":" + c.DBPassword + "@" + c.DBHost + ":" + c.DBPort + "/" + c.DBName + "?sslmode=disable"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
