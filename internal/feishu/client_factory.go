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

// cachedClient 记录构造 *lark.Client 时使用的凭证，凭证变化时需要重建
type cachedClient struct {
	client    *lark.Client
	appID     string
	appSecret string
}

// ClientFactory 提供 per-user 的 *lark.Client，自动检查 token 过期并刷新
type ClientFactory struct {
	appConfigStore AppConfigStore
	store          TokenStore
	authClient     *AuthClient

	mu          sync.Mutex
	clientCache map[uuid.UUID]cachedClient
	refresh     map[uuid.UUID]chan struct{} // 正在刷新的用户 → 刷新完成时关闭的 channel，用于阻塞等待而非固定 sleep
	revoked     map[uuid.UUID]struct{}      // 在刷新进行中被撤销的用户，refreshToken 完成时据此放弃写回，防止撤销被复活
}

func NewClientFactory(store TokenStore, appConfigStore AppConfigStore, authClient *AuthClient) *ClientFactory {
	return &ClientFactory{
		appConfigStore: appConfigStore,
		store:          store,
		authClient:     authClient,
		clientCache:    make(map[uuid.UUID]cachedClient),
		refresh:        make(map[uuid.UUID]chan struct{}),
		revoked:        make(map[uuid.UUID]struct{}),
	}
}

// getOrBuildClient 返回该用户凭证对应的 *lark.Client；凭证与缓存不一致时重建
func (f *ClientFactory) getOrBuildClient(userID uuid.UUID, appID, appSecret string) *lark.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.clientCache[userID]; ok && c.appID == appID && c.appSecret == appSecret {
		return c.client
	}
	cli := lark.NewClient(appID, appSecret)
	f.clientCache[userID] = cachedClient{client: cli, appID: appID, appSecret: appSecret}
	return cli
}

// ErrNotAuthorized 用户未授权飞书
type ErrNotAuthorized struct{ UserID uuid.UUID }

func (e *ErrNotAuthorized) Error() string {
	return fmt.Sprintf("feishu: user %s not authorized, please connect feishu account first", e.UserID)
}

// ErrAppNotConfigured 用户还没在 Settings 里填写飞书自建应用凭证
// （区别于 ErrNotAuthorized：后者是"填了凭证但没走 OAuth 授权"）
type ErrAppNotConfigured struct{ UserID uuid.UUID }

func (e *ErrAppNotConfigured) Error() string {
	return fmt.Sprintf("feishu: user %s has not configured app credentials, please set them in settings first", e.UserID)
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

// GetClient 返回该用户对应的 *lark.Client 和当前有效的 user_access_token
// 流程：查用户应用凭证 → 读 token → 检查过期（提前 5 分钟） → 需要则刷新 → 返回 client + token
func (f *ClientFactory) GetClient(ctx context.Context, userID uuid.UUID) (*lark.Client, string, error) {
	appCfg, err := f.appConfigStore.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", &ErrAppNotConfigured{UserID: userID}
		}
		return nil, "", fmt.Errorf("feishu: get app config: %w", err)
	}

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
		rec, err = f.refreshToken(ctx, userID, appCfg.AppID, appCfg.AppSecret, rec.RefreshToken)
		if err != nil {
			return nil, "", err
		}
	}

	cli := f.getOrBuildClient(userID, appCfg.AppID, appCfg.AppSecret)
	return cli, rec.AccessToken, nil
}

// refreshToken 用 refresh_token 刷新并存储
// 使用 mutex + per-user channel 防止同一用户并发刷新：并发调用会阻塞等待刷新协程
// 真正完成（channel 关闭）后再重读 store，而不是猜测一个固定 sleep 时长。
func (f *ClientFactory) refreshToken(ctx context.Context, userID uuid.UUID, appID, appSecret, refreshToken string) (*TokenRecord, error) {
	f.mu.Lock()
	if done, ongoing := f.refresh[userID]; ongoing {
		// 已有刷新在进行，阻塞等待其 channel 关闭（真正完成）后再重读
		f.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		rec, err := f.store.Get(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// 并发刷新失败（或被撤销）删除了 token → 视为未授权
				return nil, &ErrNotAuthorized{UserID: userID}
			}
			return nil, fmt.Errorf("feishu: get token after refresh wait: %w", err)
		}
		return rec, nil
	}
	done := make(chan struct{})
	f.refresh[userID] = done
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.refresh, userID)
		// 无论本次是否被撤销，撤销标记只对"当前这次"刷新有效，用完即清，
		// 避免残留污染同一 userID 下一次独立的刷新。
		delete(f.revoked, userID)
		close(done)
		f.mu.Unlock()
	}()

	tr, err := f.authClient.RefreshToken(ctx, appID, appSecret, refreshToken)
	if err != nil {
		// 刷新失败：refresh_token 可能已过期，删除记录，要求重新授权
		_ = f.store.Delete(ctx, userID)
		return nil, &ErrRefreshFailed{UserID: userID, Cause: err}
	}

	rec := tr.ToTokenRecord(userID)

	// 在同一把锁内完成"撤销检查 + 写回"，与 Revoke 的撤销标记/删除操作互斥，
	// 避免 Revoke 发生在刷新进行中时，刷新完成后把已撤销的授权重新写回（"复活"）。
	f.mu.Lock()
	if _, revoked := f.revoked[userID]; revoked {
		delete(f.revoked, userID)
		f.mu.Unlock()
		return nil, &ErrNotAuthorized{UserID: userID}
	}
	err = f.store.Save(ctx, rec)
	f.mu.Unlock()
	if err != nil {
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

// Revoke 撤销授权（删除 token）。若此时有并发的 refreshToken 正在进行，
// 在 refresh map 中标记该用户为"已撤销"：refreshToken 完成时会在同一把锁内
// 检查此标记，若已撤销则放弃 store.Save，防止撤销被刷新完成时悄悄覆盖（复活）。
func (f *ClientFactory) Revoke(ctx context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	if _, ongoing := f.refresh[userID]; ongoing {
		f.revoked[userID] = struct{}{}
	}
	f.mu.Unlock()
	return f.store.Delete(ctx, userID)
}
