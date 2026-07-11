package config

import "testing"

func TestLoad_DBEncryptionKeyRequired(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DB_ENCRYPTION_KEY is empty")
	}
}

func TestLoad_DBEncryptionKeyProvided_OK(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DB_ENCRYPTION_KEY", "some-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBEncryptionKey != "some-key" {
		t.Errorf("DBEncryptionKey = %q, want %q", cfg.DBEncryptionKey, "some-key")
	}
}
