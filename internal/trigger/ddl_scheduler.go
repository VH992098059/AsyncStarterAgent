package trigger

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DDLPollInterval 规范要求 15 分钟（FR-A02）
const DDLPollInterval = 15 * time.Minute

type DDLPollHandler func(ctx context.Context, userTaskID, userID, title string) error

func RunDDLScheduler(ctx context.Context, pool *pgxpool.Pool, det *DDLDetector, h DDLPollHandler) {
	ticker := time.NewTicker(DDLPollInterval)
	defer ticker.Stop()

	run := func() {
		// deadline_at 上界 = NOW() + det.DefaultLead()，与 DDLDetector.ShouldTrigger 保持一致，
		// 避免 defaultLead 调整后两边漂移。
		const q = `SELECT id::text, user_id::text, title FROM user_tasks
			WHERE completed = false AND triggered_at IS NULL
			AND deadline_at IS NOT NULL
			AND deadline_at <= NOW() + make_interval(secs => $1)`
		rows, err := pool.Query(ctx, q, det.DefaultLead().Seconds())
		if err != nil {
			log.Printf("[ddl-scheduler] query: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var taskID, userID, title string
			if err := rows.Scan(&taskID, &userID, &title); err != nil {
				log.Printf("[ddl-scheduler] scan: %v", err)
				continue
			}
			if err := h(ctx, taskID, userID, title); err != nil {
				log.Printf("[ddl-scheduler] handle %s: %v", taskID, err)
				continue
			}
			// 标记已触发（FR-A02 不重复触发）。检查 RowsAffected 避免 0 行更新被静默忽略。
			tag, err := pool.Exec(ctx, `UPDATE user_tasks SET triggered_at = NOW() WHERE id = $1`, taskID)
			if err != nil {
				log.Printf("[ddl-scheduler] mark triggered: %v", err)
				continue
			}
			if tag.RowsAffected() == 0 {
				log.Printf("[ddl-scheduler] mark triggered: 0 rows for task %s (可能已被其他实例处理)", taskID)
			}
		}
	}

	run() // 启动时立即跑一次
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
