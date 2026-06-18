package trigger

import (
	"time"

	"github.com/google/uuid"
)

// Source 触发来源标识
type Source string

const (
	SourceTodoist Source = "todoist"
	SourceFeishu  Source = "feishu"
	SourceNotion  Source = "notion"
	SourceManual  Source = "manual"
	SourceKeyword Source = "keyword"
	SourceDDL     Source = "ddl"
)

// TriggerEvent 标准化后的触发事件
type TriggerEvent struct {
	EventID    string                 `json:"event_id"`
	Source     Source                 `json:"source"`
	EventType  string                 `json:"event_type"`
	Payload    map[string]interface{} `json:"payload"`
	OccurredAt time.Time              `json:"occurred_at"`
}

// NewWebhookEvent 构造一个 webhook 触发事件
func NewWebhookEvent(source Source, eventType, eventID string, payload map[string]interface{}) TriggerEvent {
	return TriggerEvent{
		EventID:    eventID,
		Source:     source,
		EventType:  eventType,
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
	}
}

// AgentRunInput 转换为 AgentRun 创建参数
type AgentRunInput struct {
	UserID      uuid.UUID
	TaskType    string
	TriggerType Source
	TriggerSrc  string
}
