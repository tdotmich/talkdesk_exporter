package config

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Talkdesk accepts no more than 16 queries in one Live subscription.
const maxLiveQueries = 16

type Config struct {
	Talkdesk TalkdeskConfig `yaml:"talkdesk"`
}

type TalkdeskConfig struct {
	AccountName  string   `yaml:"account_name"`
	BaseAPIURL   string   `yaml:"api_base_url"`
	TokenURL     string   `yaml:"token_url"`
	LiveQueries  []string `yaml:"live_queries"`
	ClientID     string   `yaml:"-"`
	ClientSecret string   `yaml:"-"`
}

func Load(configFilePath string) (*Config, error) {
	configFile, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf(
			"open config file %q: %w",
			configFilePath,
			err,
		)
	}
	defer configFile.Close()

	var cfg Config

	decoder := yaml.NewDecoder(configFile)
	decoder.KnownFields(true)

	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf(
			"decode config file %q: %w",
			configFilePath,
			err,
		)
	}

	// The secrets come from environment variables, so they are never in the config file.
	clientID, exists := os.LookupEnv("TALKDESK_CLIENT_ID")
	if !exists || clientID == "" {
		return nil, fmt.Errorf("TALKDESK_CLIENT_ID is required")
	}

	clientSecret, exists := os.LookupEnv("TALKDESK_CLIENT_SECRET")
	if !exists || clientSecret == "" {
		return nil, fmt.Errorf("TALKDESK_CLIENT_SECRET is required")
	}

	cfg.Talkdesk.ClientID = clientID
	cfg.Talkdesk.ClientSecret = clientSecret

	if cfg.Talkdesk.AccountName == "" {
		return nil, fmt.Errorf("talkdesk.account_name is required")
	}

	if cfg.Talkdesk.BaseAPIURL == "" {
		return nil, fmt.Errorf("talkdesk.api_base_url is required")
	}

	// Remove a trailing slash so "https://api.example.com/" + "/live-queries" does not become "//live-queries".
	cfg.Talkdesk.BaseAPIURL = strings.TrimRight(cfg.Talkdesk.BaseAPIURL, "/")

	// The token URL is optional. Without it, we use the standard Talkdesk ID address for the account.
	if cfg.Talkdesk.TokenURL == "" {
		cfg.Talkdesk.TokenURL = fmt.Sprintf(
			"https://%s.talkdeskid.com/oauth/token",
			cfg.Talkdesk.AccountName,
		)
	}

	if len(cfg.Talkdesk.LiveQueries) == 0 {
		return nil, fmt.Errorf("at least one talkdesk.live_queries entry is required")
	}

	if len(cfg.Talkdesk.LiveQueries) > maxLiveQueries {
		return nil, fmt.Errorf(
			"talkdesk.live_queries cannot contain more than %d metrics",
			maxLiveQueries,
		)
	}

	return &cfg, nil
}
