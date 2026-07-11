package config

import "testing"

func TestLoad_FeishuRequiresDBEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEISHU_APP_ID", "app-id")
	t.Setenv("FEISHU_APP_SECRET", "app-secret")
	t.Setenv("DB_ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when FEISHU_APP_ID/SECRET set without DB_ENCRYPTION_KEY, got nil")
	}
}

func TestLoad_FeishuWithDBEncryptionKey_OK(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEISHU_APP_ID", "app-id")
	t.Setenv("FEISHU_APP_SECRET", "app-secret")
	t.Setenv("DB_ENCRYPTION_KEY", "some-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBEncryptionKey != "some-key" {
		t.Errorf("DBEncryptionKey = %q, want %q", cfg.DBEncryptionKey, "some-key")
	}
}

func TestLoad_NoFeishu_NoDBEncryptionKeyRequired(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEISHU_APP_ID", "")
	t.Setenv("FEISHU_APP_SECRET", "")
	t.Setenv("DB_ENCRYPTION_KEY", "")

	_, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
