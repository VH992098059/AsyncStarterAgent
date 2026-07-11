package feishu

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// memAppConfigStore 是内存版 AppConfigStore，供本包其他测试文件复用
type memAppConfigStore struct {
	mu   sync.Mutex
	data map[uuid.UUID]*AppConfigRecord
}

func newMemAppConfigStore() *memAppConfigStore {
	return &memAppConfigStore{data: make(map[uuid.UUID]*AppConfigRecord)}
}

func (m *memAppConfigStore) Get(_ context.Context, userID uuid.UUID) (*AppConfigRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.data[userID]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return rec, nil
}

func (m *memAppConfigStore) Upsert(_ context.Context, rec AppConfigRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	recCopy := rec
	m.data[rec.UserID] = &recCopy
	return nil
}

func (m *memAppConfigStore) Delete(_ context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, userID)
	return nil
}
