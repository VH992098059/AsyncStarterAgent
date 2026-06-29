import { useState, useEffect, useRef } from "react";
import { listAgentRuns, type AgentRunItem } from "../api/client";
import { IconFilter, IconPlus } from "./Icons";
import { showToast } from "./Layout";

interface KanbanBoardProps {
  onSelectTask: (task: AgentRunItem) => void;
  onDeselectTask?: () => void;
  selectedTaskId: string | null;
  onAddTask?: (text: string) => Promise<void>;
}

interface KanbanColumn {
  id: string;
  title: string;
  statusClass: string;
  items: AgentRunItem[];
}

const STATUS_MAP: Record<string, string> = {
  pending: "pending",
  running: "context",
  context_collecting: "context",
  harvesting: "context",
  drafting: "draft",
  synthesizing: "draft",
  delivering: "delivery",
  completed: "done",
  failed: "pending",
};

const COLUMNS_DEFINITION = [
  { id: "pending", title: "待触发", statusClass: "pending" },
  { id: "context", title: "上下文搜集", statusClass: "context" },
  { id: "draft", title: "草稿编辑", statusClass: "draft" },
  { id: "delivery", title: "待交付", statusClass: "delivery" },
  { id: "done", title: "已完成", statusClass: "done" },
];

function getTagByTaskType(taskType: string): { label: string; className: string } {
  const type = (taskType || "").toLowerCase();
  if (type.includes("email") || type.includes("邮件")) return { label: "邮件", className: "email" };
  if (type.includes("doc") || type.includes("文档") || type.includes("周报") || type.includes("报告")) return { label: "文档", className: "doc" };
  if (type.includes("report") || type.includes("总结") || type.includes("会议")) return { label: "报告", className: "report" };
  return { label: "消息", className: "message" };
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = Date.now();
  const diff = (now - d.getTime()) / 1000;
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
  return d.toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function makeDemoItem(id: string, status: string, taskType: string, triggerSource: string, msAgo: number): AgentRunItem {
  return {
    id,
    status,
    task_type: taskType,
    trigger_type: "",
    trigger_source: triggerSource,
    current_stage: status,
    error_message: "",
    created_at: new Date(Date.now() - msAgo).toISOString(),
    completed_at: status === "completed" ? new Date(Date.now() - msAgo + 60000).toISOString() : null,
  };
}

export function KanbanBoard({ onSelectTask, onDeselectTask, selectedTaskId, onAddTask }: KanbanBoardProps) {
  const [columns, setColumns] = useState<KanbanColumn[]>(
    COLUMNS_DEFINITION.map((c) => ({ ...c, items: [] }))
  );
  const [loading, setLoading] = useState(true);
  const [filterActive, setFilterActive] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [addText, setAddText] = useState("");
  const [adding, setAdding] = useState(false);
  const addInputRef = useRef<HTMLInputElement>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  useEffect(() => {
    if (addOpen && addInputRef.current) {
      addInputRef.current.focus();
    }
  }, [addOpen]);

  useEffect(() => {
    const loadRuns = async () => {
      setLoading(true);
      try {
        const res = await listAgentRuns(50);
        const grouped: Record<string, AgentRunItem[]> = {
          pending: [],
          context: [],
          draft: [],
          delivery: [],
          done: [],
        };

        let items = res.runs;

        if (items.length === 0) {
          items = [
            makeDemoItem("demo1", "pending", "回复客户邮件", "手动触发", 3600000),
            makeDemoItem("demo2", "running", "整理本周工作周报", "定时触发", 1800000),
            makeDemoItem("demo3", "context_collecting", "汇总GitHub PR通知", "Webhook", 7200000),
            makeDemoItem("demo4", "drafting", "总结昨天的会议记录", "手动触发", 86400000),
            makeDemoItem("demo5", "synthesizing", "撰写项目进度更新", "关键词触发", 172800000),
            makeDemoItem("demo6", "delivering", "发送会议邀请邮件", "手动触发", 259200000),
            makeDemoItem("demo7", "completed", "上周工作总结", "定时触发", 432000000),
            makeDemoItem("demo8", "completed", "回复合作方邮件", "手动触发", 518400000),
          ];
        }

        items.forEach((item) => {
          const colId = STATUS_MAP[item.status] || "pending";
          if (grouped[colId]) {
            grouped[colId].push(item);
          }
        });

        setColumns(
          COLUMNS_DEFINITION.map((c) => ({
            ...c,
            items: grouped[c.id] || [],
          }))
        );
      } catch {
        const items: AgentRunItem[] = [
          makeDemoItem("demo1", "pending", "回复客户邮件", "手动触发", 3600000),
          makeDemoItem("demo2", "running", "整理本周工作周报", "定时触发", 1800000),
          makeDemoItem("demo3", "drafting", "总结昨天的会议记录", "手动触发", 86400000),
          makeDemoItem("demo4", "completed", "上周工作总结", "定时触发", 432000000),
        ];

        const grouped: Record<string, AgentRunItem[]> = {
          pending: items.filter(i => STATUS_MAP[i.status] === "pending"),
          context: items.filter(i => STATUS_MAP[i.status] === "context"),
          draft: items.filter(i => STATUS_MAP[i.status] === "draft"),
          delivery: items.filter(i => STATUS_MAP[i.status] === "delivery"),
          done: items.filter(i => STATUS_MAP[i.status] === "done"),
        };

        setColumns(
          COLUMNS_DEFINITION.map((c) => ({
            ...c,
            items: grouped[c.id] || [],
          }))
        );
      } finally {
        setLoading(false);
      }
    };

    loadRuns();
  }, [refreshKey]);

  const handleAddSubmit = async () => {
    const text = addText.trim();
    if (!text || adding) return;
    setAdding(true);
    try {
      if (onAddTask) {
        await onAddTask(text);
      }
      setAddText("");
      setAddOpen(false);
      setRefreshKey(k => k + 1);
      showToast("任务已创建", "Agent 已开始处理", "success");
    } catch {
      showToast("创建失败", "请稍后重试", "error");
    } finally {
      setAdding(false);
    }
  };

  const handleAddKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAddSubmit();
    } else if (e.key === "Escape") {
      setAddOpen(false);
      setAddText("");
    }
  };

  const handleColumnsClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!onDeselectTask) return;
    const target = e.target as HTMLElement;
    if (target.closest(".kanban-card")) return;
    if (target.closest(".kanban-column-header")) return;
    if (target.closest(".kanban-add-form")) return;
    onDeselectTask();
  };

  return (
    <>
      <div className="kanban-header" onClick={() => onDeselectTask?.()}>
        <h2 className="kanban-title">工作流看板</h2>
        <div className="kanban-header-actions">
          <button
            className="kanban-add-btn"
            onClick={(e) => { e.stopPropagation(); setAddOpen(!addOpen); if (!addOpen) setAddText(""); }}
            title="新建任务"
          >
            <IconPlus size={16} />
            <span className="hidden md:inline">新建</span>
          </button>
          <button
            className={`kanban-filter-btn ${filterActive ? "active" : ""}`}
            onClick={(e) => { e.stopPropagation(); setFilterActive(!filterActive); }}
            title={filterActive ? "关闭筛选" : "筛选任务"}
          >
            <IconFilter size={16} />
          </button>
        </div>
      </div>
      {addOpen && (
        <div className="kanban-add-form" onClick={(e) => e.stopPropagation()}>
          <input
            ref={addInputRef}
            type="text"
            className="kanban-add-input"
            placeholder="输入任务描述，例如：帮我写本周周报..."
            value={addText}
            onChange={(e) => setAddText(e.target.value)}
            onKeyDown={handleAddKeyDown}
            disabled={adding}
          />
          <button
            className="kanban-add-submit"
            onClick={handleAddSubmit}
            disabled={!addText.trim() || adding}
          >
            {adding ? "创建中..." : "创建"}
          </button>
          <button
            className="kanban-add-cancel"
            onClick={() => { setAddOpen(false); setAddText(""); }}
            disabled={adding}
          >
            取消
          </button>
        </div>
      )}
      <div className="kanban-columns" onClick={handleColumnsClick}>
        {columns.map((col) => (
          <div key={col.id} className="kanban-column">
            <div className="kanban-column-header">
              <span className="kanban-column-title">
                <span className={`status-dot ${col.statusClass}`} />
                {col.title}
              </span>
              <span className="kanban-column-count">{col.items.length}</span>
            </div>
            <div className="kanban-column-cards">
              {col.items.map((item) => {
                const tag = getTagByTaskType(item.task_type || "");
                return (
                  <div
                    key={item.id}
                    className={`kanban-card ${selectedTaskId === item.id ? "selected" : ""}`}
                    onClick={(e) => { e.stopPropagation(); onSelectTask(item); }}
                  >
                    <div className="kanban-card-title">{item.task_type || "未命名任务"}</div>
                    <div className="kanban-card-meta">
                      <span className={`kanban-card-tag ${tag.className}`}>{tag.label}</span>
                      <span>{formatTime(item.created_at)}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
