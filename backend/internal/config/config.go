// Package config reads every value the API needs to boot from the environment.
//
// Nothing here runs at import time: the process must be able to start, serve
// and log before configuration is touched. Load is called once from main and
// the resulting value is passed to the components that need it.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Defaults for the non-secret values. Required secrets have no default on
// purpose — a missing DATABASE_URL, VALKEY_URL or AUTH_SECRET must refuse to
// start and name itself, instead of silently falling back to something unsafe.
const (
	DefaultPort           = "8000"
	DefaultHourRateCents  = 6500
	DefaultFrontendOrigin = "http://localhost:5173"
)

// Config is the fully resolved runtime configuration of the API.
type Config struct {
	DatabaseURL    string
	ValkeyURL      string
	Port           string
	HourRateCents  int
	FrontendOrigin string
	AuthSecret     string
}

// Load resolves the configuration and validates that every required value is
// present. Missing values are collected and reported together so an operator
// can fix them in one pass. Each error message names the variable and points
// at RUN.json, where the value is declared for the automated runner.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    requiredValue("DATABASE_URL"),
		ValkeyURL:      requiredValue("VALKEY_URL"),
		Port:           valueOrDefault("PORT", DefaultPort),
		FrontendOrigin: valueOrDefault("FRONTEND_ORIGIN", DefaultFrontendOrigin),
		AuthSecret:     requiredValue("AUTH_SECRET"),
	}

	hourRate, err := strconv.Atoi(valueOrDefault("HOUR_RATE_CENTS", strconv.Itoa(DefaultHourRateCents)))
	if err != nil || hourRate < 0 {
		return nil, fmt.Errorf("HOUR_RATE_CENTS must be a non-negative integer (see RUN.json), got %q", os.Getenv("HOUR_RATE_CENTS"))
	}
	cfg.HourRateCents = hourRate

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.ValkeyURL == "" {
		missing = append(missing, "VALKEY_URL")
	}
	if cfg.AuthSecret == "" {
		missing = append(missing, "AUTH_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variable(s): %s (declared in RUN.json)", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// requiredValue returns the variable or the empty string. It treats an
// unresolved "${service:...}" placeholder as absent, so a clone without the
// sibling service keeps its working default instead of an unusable literal.
func requiredValue(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if strings.HasPrefix(v, "${") {
		return ""
	}
	return v
}

// valueOrDefault returns the variable, or fallback when it is unset, empty or
// still an unresolved placeholder.
func valueOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" || strings.HasPrefix(v, "${") {
		return fallback
	}
	return v
}
