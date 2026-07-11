package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/asyncstarter/agent/internal/synthesis"
	"github.com/asyncstarter/agent/pkg/httpx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftStreamHandler struct {
	Svc  *synthesis.Service
	Pool *pgxpool.Pool
}

// Stream 处理草稿流式响应：
//   - 草稿已存在 → 重放（不改变 run 状态）
//   - 草稿不存在 → 先尝试认领该 run（repository.ClaimRunForSynthesis，原子 UPDATE ...
//     WHERE status='pending'）：
//   - 认领成功 → 本请求负责生成，跑完整 GenerateDraftStream（实时 token 流式输出）
//     成功 → handler 标 completed/synthesis；失败 → handler 标 failed/synthesis
//   - 认领失败（已被 worker 认领，问题 #11：手动触发同时入队 asynq） → 说明后台
//     worker 正在生成，本请求改为轮询等待草稿出现后重放（无实时 token，但避免
//     重复调用一次 LLM）
//
// 状态回写失败不阻断流式响应，仅记日志。
func (h *DraftStreamHandler) Stream(c *gin.Context) {
	if h.Svc == nil {
		httpx.Fail(c, http.StatusServiceUnavailable, 5002, "draft service not configured")
		return
	}
	runID := c.Param("id")
	ctx := c.Request.Context()

	a2ui, err := synthesis.NewA2UIWriter(c.Writer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, 5001, "streaming not supported")
		return
	}

	exists, err := h.Svc.DraftExists(ctx, runID)
	if err != nil {
		_ = a2ui.WriteError("check draft: " + err.Error())
		return
	}

	if !exists {
		runUUID, parseErr := uuid.Parse(runID)
		if parseErr != nil {
			_ = a2ui.WriteError("invalid run id: " + parseErr.Error())
			return
		}
		if h.Pool != nil {
			claimed, claimErr := repository.ClaimRunForSynthesis(ctx, h.Pool, runUUID)
			if claimErr != nil {
				_ = a2ui.WriteError("claim run: " + claimErr.Error())
				return
			}
			if !claimed {
				// 已被后台 worker 认领（或早于本请求已在其他地方生成中），
				// 轮询等待草稿出现后重放，避免重复调用 LLM。
				h.waitAndReplay(ctx, runID, a2ui)
				return
			}
		}

		var userID, taskType string
		if err = h.Svc.QueryRunInfo(ctx, runID, &userID, &taskType); err != nil {
			_ = a2ui.WriteError("run not found: " + err.Error())
			return
		}
		genErr := h.Svc.GenerateDraftStream(ctx, runID, userID, taskType, a2ui)
		h.writeRunStatus(ctx, runID, genErr)
		if genErr != nil {
			_ = a2ui.WriteError(genErr.Error())
		}
		return
	}

	if err := h.Svc.StreamDraft(ctx, runID, a2ui); err != nil {
		_ = a2ui.WriteError(err.Error())
	}
}

// waitAndReplay 轮询等待草稿出现（后台 worker 正在生成中），出现后重放；
// 超时（60s）或 run 转为 failed 则报错退出。
func (h *DraftStreamHandler) waitAndReplay(ctx context.Context, runID string, a2ui *synthesis.A2UIWriter) {
	const (
		pollInterval = 500 * time.Millisecond
		timeout      = 60 * time.Second
	)
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		exists, err := h.Svc.DraftExists(ctx, runID)
		if err != nil {
			_ = a2ui.WriteError("check draft: " + err.Error())
			return
		}
		if exists {
			if err := h.Svc.StreamDraft(ctx, runID, a2ui); err != nil {
				_ = a2ui.WriteError(err.Error())
			}
			return
		}
		if time.Now().After(deadline) {
			_ = a2ui.WriteError("draft generation timed out, please retry later")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// writeRunStatus 在 GenerateDraftStream 结束后回写 run 状态：
//   - genErr == nil → completed/synthesis
//   - genErr != nil → failed/synthesis + errMsg
//
// 仅当 Pool 已注入且 runID 是合法 UUID 时执行；回写失败仅记日志。
func (h *DraftStreamHandler) writeRunStatus(ctx context.Context, runID string, genErr error) {
	if h.Pool == nil {
		return
	}
	runUUID, parseErr := uuid.Parse(runID)
	if parseErr != nil {
		log.Printf("[draft] invalid run id %q: %v", runID, parseErr)
		return
	}
	status := "completed"
	stage := "synthesis"
	errMsg := ""
	if genErr != nil {
		status = "failed"
		errMsg = genErr.Error()
	}
	if err := repository.UpdateAgentRunStatus(ctx, h.Pool, runUUID, status, stage, errMsg); err != nil {
		log.Printf("[draft] update run status to %s/%s: %v", status, stage, err)
	}
}
