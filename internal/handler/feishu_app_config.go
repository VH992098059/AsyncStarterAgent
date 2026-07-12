package handler

import (
	"net/http"
	"strings"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/internal/feishu"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
)

// FeishuAppConfigHandler 暴露用户级飞书自建应用凭证的 CRUD：
//   - GET    /api/v1/feishu/app-config  查询（app_secret 掩码）
//   - PUT    /api/v1/feishu/app-config  保存/更新（清空旧 OAuth token，强制重新授权）
//   - DELETE /api/v1/feishu/app-config  删除（级联删除 OAuth token）
type FeishuAppConfigHandler struct {
	Store      feishu.AppConfigStore
	TokenStore feishu.TokenStore
}

type appConfigResponse struct {
	AppID           string `json:"app_id"`
	AppSecretMasked string `json:"app_secret_masked"`
	Configured      bool   `json:"configured"`
}

// maskAppSecret 复用 settings 包 maskKey 的规则：前 4 位 + **** + 后 4 位，长度 ≤8 返回空
func maskAppSecret(secret string) string {
	if len(secret) <= 8 {
		return ""
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// isMaskedAppSecret 判断传入值是否是掩码值本身（前端未修改直接回传的情况）
func isMaskedAppSecret(s string) bool {
	return strings.Contains(s, "****")
}

func (h *FeishuAppConfigHandler) Get(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	rec, err := h.Store.Get(c.Request.Context(), uid)
	if err != nil {
		httpx.OK(c, appConfigResponse{Configured: false})
		return
	}
	httpx.OK(c, appConfigResponse{
		AppID:           rec.AppID,
		AppSecretMasked: maskAppSecret(rec.AppSecret),
		Configured:      true,
	})
}

type putAppConfigRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

func (h *FeishuAppConfigHandler) Put(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	var req putAppConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "invalid body: "+err.Error())
		return
	}
	if req.AppID == "" {
		httpx.Fail(c, http.StatusBadRequest, 4001, "app_id is required")
		return
	}

	appSecret := req.AppSecret
	if appSecret == "" || isMaskedAppSecret(appSecret) {
		// 留空或回传掩码值：保留原有 app_secret，仅可能更新 app_id
		existing, err := h.Store.Get(c.Request.Context(), uid)
		if err != nil {
			httpx.Fail(c, http.StatusBadRequest, 4001, "app_secret is required for first-time setup")
			return
		}
		appSecret = existing.AppSecret
	}

	if err := h.Store.Upsert(c.Request.Context(), feishu.AppConfigRecord{
		UserID: uid, AppID: req.AppID, AppSecret: appSecret,
	}); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, err.Error())
		return
	}

	// 凭证变化：旧 OAuth token 必然失效，强制用户重新授权。
	// TokenStore.Delete 底层是 SQL DELETE，对不存在的记录也返回 nil（不会得到 pgx.ErrNoRows），
	// 所以这里不需要区分"未授权过"和"真实错误"——任何非 nil 错误都是真实的 DB 故障。
	if err := h.TokenStore.Delete(c.Request.Context(), uid); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "saved credentials but failed to reset authorization: "+err.Error())
		return
	}

	httpx.OK(c, appConfigResponse{AppID: req.AppID, AppSecretMasked: maskAppSecret(appSecret), Configured: true})
}

func (h *FeishuAppConfigHandler) Delete(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "no user in context")
		return
	}
	if err := h.Store.Delete(c.Request.Context(), uid); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, err.Error())
		return
	}
	if err := h.TokenStore.Delete(c.Request.Context(), uid); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "deleted credentials but failed to cascade-delete authorization: "+err.Error())
		return
	}
	httpx.OK(c, gin.H{"status": "deleted"})
}
