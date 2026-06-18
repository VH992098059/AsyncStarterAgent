package trigger

import "time"

type DDLDetector struct {
	defaultLead time.Duration
}

func NewDDLDetector() *DDLDetector {
	return &DDLDetector{defaultLead: 24 * time.Hour}
}

func (d *DDLDetector) DefaultLead() time.Duration { return d.defaultLead }

// ShouldTrigger 判断任务是否应该触发
// - 已逾期 (deadline < now) → 触发
// - 距 deadline ≤ defaultLead → 触发
// - 否则 → 不触发
//
// `lead` 参数当前保留为 API 占位（plan spec 阶段 T008 暂未启用自定义 lead，
// 决策在 T009 整合前与用户确认是否引入）。函数体使用 `d.defaultLead`。
// plan 原始 spec 中 `lead` 类型与测试签名不一致，详见 doc/handoff/T008-handoff.md。
func (d *DDLDetector) ShouldTrigger(deadline time.Time, lead time.Duration, now time.Time) bool {
	_ = lead
	return deadline.Before(now) || deadline.Sub(now) <= d.defaultLead
}
