package feishu

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// ClientFactory 提供 per-user 的 *lark.Client，自动检查 token 过期并刷新
type ClientFactory struct {
	appID      string
	appSecret  string
	store      TokenStore
	authClient *AuthClient
	// client 是共享的 *lark.Client，在构造时创建一次并复用。
	// user_access_token 由调用方在每次请求时通过 lark.WithUserAccessToken(token) 注入，
	// 因此 client 本身不持有用户身份，可安全共享。
	client *lark.Client

	mu      sync.Mutex
	refresh map[uuid.UUID]struct{} // 正在刷新的用户集合，防止并发刷新
}

func NewClientFactory(appID, appSecret string, store TokenStore, authClient *AuthClient) *ClientFactory {
	return &ClientFactory{
		appID:      appID,
		appSecret:  appSecret,
		store:      store,
		authClient: authClient,
		client:     lark.NewClient(appID, appSecret),
		refresh:    make(map[uuid.UUID]struct{}),
	}
}

// ErrNotAuthorized 用户未授权飞书
type ErrNotAuthorized struct{ UserID uuid.UUID }

func (e *ErrNotAuthorized) Error() string {
	return fmt.Sprintf("feishu: user %s not authorized, please connect feishu account first", e.UserID)
}

// ErrRefreshFailed token 刷新失败（refresh_token 过期等）
type ErrRefreshFailed struct {
	UserID uuid.UUID
	Cause  error
}

func (e *ErrRefreshFailed) Error() string {
	return fmt.Sprintf("feishu: refresh failed for user %s: %v", e.UserID, e.Cause)
}

// Unwrap 暴露 Cause 以支持 errors.Is / errors.As 链式判断
func (e *ErrRefreshFailed) Unwrap() error { return e.Cause }

// GetClient 返回共享的 *lark.Client 和当前有效的 user_access_token
// 流程：读 token → 检查过期（提前 5 分钟） → 需要则刷新 → 返回 client + token
func (f *ClientFactory) GetClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error) {
	rec, err := f.store.Get(ctx, userID)
	if err != nil {
		// 仅 not-found 视为"未授权"；其他错误（DB 故障、解密失败等）原样上抛以便排查
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", &ErrNotAuthorized{UserID: userID}
		}
		return nil, "", fmt.Errorf("feishu: get token: %w", err)
	}

	// 提前 5 分钟判定过期，避免调用时刚好失效
	if time.Until(rec.ExpiresAt) < 5*time.Minute {
		rec, err = f.refreshToken(ctx, userID, rec.RefreshToken)
		if err != nil {
			return nil, "", err
		}
	}

	// 返回共享 client；调用方在每次请求时通过 lark.WithUserAccessToken(rec.AccessToken) 注入用户身份
	return f.client, rec.AccessToken, nil
}

// refreshToken 用 refresh_token 刷新并存储
// 使用 mutex 防止同一用户并发刷新
func (f *ClientFactory) refreshToken(ctx context.Context, userID uuid.UUID, refreshToken string) (*TokenRecord, error) {
	f.mu.Lock()
	if _, ongoing := f.refresh[userID]; ongoing {
		// 已有刷新在进行，等待释放后重读
		f.mu.Unlock()
		// MVP 简化：等待 200ms 后重读 store。
		// 已知限制：若并发刷新尚未完成，此处可能读到旧的（仍过期的）access_token，
		// 调用方请求时会收到 401。V1.5 可改为 condition variable / channel 等待刷新完成。
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		rec, err := f.store.Get(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// 并发刷新失败并删除了 token → 视为未授权
				return nil, &ErrNotAuthorized{UserID: userID}
			}
			return nil, fmt.Errorf("feishu: get token after refresh wait: %w", err)
		}
		return rec, nil
	}
	f.refresh[userID] = struct{}{}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.refresh, userID)
		f.mu.Unlock()
	}()

	tr, err := f.authClient.RefreshToken(ctx, refreshToken)
	if err != nil {
		// 刷新失败：refresh_token 可能已过期，删除记录，要求重新授权
		_ = f.store.Delete(ctx, userID)
		return nil, &ErrRefreshFailed{UserID: userID, Cause: err}
	}

	rec := tr.ToTokenRecord(userID)
	if err := f.store.Save(ctx, rec); err != nil {
		return nil, fmt.Errorf("feishu: save refreshed token: %w", err)
	}
	return &rec, nil
}

// IsAuthorized 检查用户是否已授权（仅检查 token 记录是否存在，不检查过期）
// 过期的 access_token 仍可能通过 refresh 恢复，故视为已授权
func (f *ClientFactory) IsAuthorized(ctx context.Context, userID uuid.UUID) bool {
	_, err := f.store.Get(ctx, userID)
	return err == nil
}

// Revoke 撤销授权（删除 token）
// 已知限制（MVP）：若撤销时仍有刷新在进行，刷新可能在此之后调用 store.Save 重新写入
// token，导致撤销被覆盖。V1.5 可通过取消 in-flight context 解决。
func (f *ClientFactory) Revoke(ctx context.Context, userID uuid.UUID) error {
	return f.store.Delete(ctx, userID)
}
