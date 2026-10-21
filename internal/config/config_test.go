package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	configFilePath := filepath.Join(t.TempDir(), "talkdesk_exporter.yaml")

	err := os.WriteFile(configFilePath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return configFilePath
}

func TestLoad(t *testing.T) {
	t.Setenv("TALKDESK_CLIENT_ID", "id")
	t.Setenv("TALKDESK_CLIENT_SECRET", "secret")

	configFilePath := writeConfigFile(t, `
talkdesk:
  account_name: my-account
  api_base_url: https://api.talkdeskapp.com/
  live_queries:
    - total_arrived_contacts
`)

	cfg, err := Load(configFilePath)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}

	if cfg.Talkdesk.BaseAPIURL != "https://api.talkdeskapp.com" {
		t.Errorf("expected the trailing slash to be removed, got %q", cfg.Talkdesk.BaseAPIURL)
	}

	if cfg.Talkdesk.TokenURL != "https://my-account.talkdeskid.com/oauth/token" {
		t.Errorf("unexpected default token URL %q", cfg.Talkdesk.TokenURL)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("TALKDESK_CLIENT_ID", "")
	t.Setenv("TALKDESK_CLIENT_SECRET", "")

	configFilePath := writeConfigFile(t, `
talkdesk:
  account_name: my-account
  api_base_url: https://api.talkdeskapp.com
  live_queries:
    - total_arrived_contacts
`)

	_, err := Load(configFilePath)
	if err == nil {
		t.Fatal("expected an error when TALKDESK_CLIENT_ID is empty")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Setenv("TALKDESK_CLIENT_ID", "id")
	t.Setenv("TALKDESK_CLIENT_SECRET", "secret")

	// A client secret in the file is an unknown field. It must stay in the environment.
	configFilePath := writeConfigFile(t, `
talkdesk:
  account_name: my-account
  api_base_url: https://api.talkdeskapp.com
  client_secret: do-not-put-me-here
  live_queries:
    - total_arrived_contacts
`)

	_, err := Load(configFilePath)
	if err == nil {
		t.Fatal("expected an error for an unknown field")
	}
}
