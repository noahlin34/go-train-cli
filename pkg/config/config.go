package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	APIKey  string
	BaseURL string
}

const DefaultBaseURL = "https://api.openmetrolinx.com/OpenDataAPI"

func Load(explicitKey string) Config {
	loadDotEnv(".env")
	key := strings.TrimSpace(explicitKey)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("GO_API_KEY"))
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("GO_TRAIN_API_KEY"))
	}
	baseURL := strings.TrimSpace(os.Getenv("GO_API_BASE_URL"))
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return Config{APIKey: key, BaseURL: strings.TrimRight(baseURL, "/")}
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
