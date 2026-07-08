package feishu

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// memTokenStore 是内存版 TokenStore，用于单测
type memTokenStore struct {
	mu   sync.Mutex
	data map[uuid.UUID]*TokenRecord
	err  error
}

func newMemTokenStore() *memTokenStore {
	return &memTokenStore{data: make(map[uuid.UUID]*TokenRecord)}
}

func (m *memTokenStore) Save(_ context.Context, rec TokenRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	recCopy := rec
	m.data[rec.UserID] = &recCopy
	return nil
}

func (m *memTokenStore) Get(_ context.Context, userID uuid.UUID) (*TokenRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	rec, ok := m.data[userID]
	if !ok {
		return nil, errors.New("not found")
	}
	return rec, nil
}

func (m *memTokenStore) Delete(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, userID)
	return nil
}

func TestTokenStore_SaveAndGet(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	rec := TokenRecord{
		UserID:       uid,
		AccessToken:  "acc-123",
		RefreshToken: "ref-456",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		OpenID:       "ou_abc",
		Name:         "张三",
	}
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Get(context.Background(), uid)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessToken != "acc-123" {
		t.Errorf("access token: %s", got.AccessToken)
	}
	if got.RefreshToken != "ref-456" {
		t.Errorf("refresh token: %s", got.RefreshToken)
	}
	if got.OpenID != "ou_abc" {
		t.Errorf("open id: %s", got.OpenID)
	}
}

func TestTokenStore_Get_NotFound(t *testing.T) {
	store := newMemTokenStore()
	_, err := store.Get(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestTokenStore_Delete(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "x"})
	if err := store.Delete(context.Background(), uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := store.Get(context.Background(), uid)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestTokenStore_Save_Overwrite(t *testing.T) {
	store := newMemTokenStore()
	uid := uuid.New()
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "old"})
	_ = store.Save(context.Background(), TokenRecord{UserID: uid, AccessToken: "new"})
	got, _ := store.Get(context.Background(), uid)
	if got.AccessToken != "new" {
		t.Errorf("expected overwrite to 'new', got %s", got.AccessToken)
	}
}
