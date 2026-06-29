package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/asyncstarter/agent/internal/auth"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthHandler 暴露注册/登录/登出/当前用户四个端点。
type AuthHandler struct {
	Svc *auth.Service
	Mgr *auth.Manager
	BL  *auth.Blacklist
	TTL time.Duration
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type authResponse struct {
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	ExpiresIn int    `json:"expires_in"` // 秒
}

// Register POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "请求格式不正确")
		return
	}
	id, err := h.Svc.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			httpx.Fail(c, http.StatusConflict, 4009, "该用户名已被注册，请更换")
		case errors.Is(err, auth.ErrUsernameLength):
			httpx.Fail(c, http.StatusBadRequest, 4001, "用户名长度应为 3-64 个字符")
		case errors.Is(err, auth.ErrPasswordLength):
			httpx.Fail(c, http.StatusBadRequest, 4001, "密码至少需要 6 位")
		default:
			httpx.Fail(c, http.StatusBadRequest, 4001, "注册失败，请稍后重试")
		}
		return
	}
	tok, err := h.Mgr.Sign(id.String(), req.Username)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "登录凭证生成失败，请稍后重试")
		return
	}
	httpx.OK(c, authResponse{
		Token:     tok,
		UserID:    id.String(),
		Username:  req.Username,
		ExpiresIn: int(h.TTL.Seconds()),
	})
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, 4001, "请求格式不正确")
		return
	}
	id, username, err := h.Svc.Login(c.Request.Context(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredential) {
			httpx.Fail(c, http.StatusUnauthorized, 4001, "用户名或密码错误，请检查后重试")
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, 5001, "登录失败，请稍后重试")
		return
	}
	tok, err := h.Mgr.Sign(id.String(), username)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "登录凭证生成失败，请稍后重试")
		return
	}
	httpx.OK(c, authResponse{
		Token:     tok,
		UserID:    id.String(),
		Username:  username,
		ExpiresIn: int(h.TTL.Seconds()),
	})
}

// Logout POST /api/v1/auth/logout
// 把当前 token 的 signature 段加入黑名单。
// 需要先经过 Middleware，所以 c 上一定有 user_id。
func (h *AuthHandler) Logout(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httpx.Fail(c, http.StatusBadRequest, 4001, "缺少登录凭证")
		return
	}
	tok := strings.TrimPrefix(header, "Bearer ")
	tokID := auth.ExtractTokenID(tok)
	if tokID == "" {
		httpx.Fail(c, http.StatusBadRequest, 4001, "登录凭证格式不正确")
		return
	}
	h.BL.Revoke(tokID, h.TTL)
	httpx.OK(c, gin.H{"revoked": true})
}

// Me GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	uid, ok := auth.MustUserID(c)
	if !ok {
		httpx.Fail(c, http.StatusUnauthorized, 4001, "登录已过期，请重新登录")
		return
	}
	username, _ := c.Get(auth.ContextUsernameKey)
	httpx.OK(c, gin.H{
		"user_id":  uid.String(),
		"username": username,
	})
}

// 编译期检查 uuid 包被使用（防止未使用 import 编译错）
var _ = uuid.Nil
