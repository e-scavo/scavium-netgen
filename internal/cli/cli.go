package cli

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func MustEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("missing environment variable: %s", key)
	}
	return v
}

func EnvOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func FatalIf(err error) {
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func RequireArgs(min int, usage string) {
	if len(os.Args) < min {
		fmt.Println(usage)
		os.Exit(1)
	}
}
