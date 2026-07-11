package queue

// TaskDraftGenerate 是草稿生成任务的类型名（问题 #11：接入 asynq 异步生成）。
// payload 是 AgentRun 的 UUID 字符串。
const TaskDraftGenerate = "draft:generate"
