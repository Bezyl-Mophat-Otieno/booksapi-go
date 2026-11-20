package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port                  string
	DatabasePath          string
	PesapalConsumerKey    string
	PesapalConsumerSecret string
	PesapalBaseURL        string
	PesapalIPNID          string
}

func Load() (Config, error) {
	var missing []string

	required := func(key string) string {
		v, ok := os.LookupEnv(key)
		if !ok || strings.TrimSpace(v) == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		Port:                  optional("PORT", "8080"),
		DatabasePath:          optional("DATABASE_PATH", "books.db"),
		PesapalConsumerKey:    required("PESAPAL_CONSUMER_KEY"),
		PesapalConsumerSecret: required("PESAPAL_CONSUMER_SECRET"),
		PesapalBaseURL:        required("PESAPAL_BASE_URL"),
		PesapalIPNID:          required("PESAPAL_IPN_ID"),
	}

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s",
			strings.Join(missing, ", "))
	}
	return cfg, nil
}

func optional(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}
