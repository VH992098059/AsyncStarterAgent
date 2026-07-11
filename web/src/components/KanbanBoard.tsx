import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import {
  listAgentRuns,
  retryRun,
  deleteRun,
  archiveRun,
  type AgentRunItem,
} from "../api/client";
import { IconFilter, IconPlus, IconMore } from "./Icons";
import { showToast } from "./Layout";
import { usePolling } from "../hooks/usePolling";
import { CardMenu } from "./CardMenu";

interface KanbanBoardProps {
  onSelectTask: (task: AgentRunItem) => void;
  onDeselectTask?: () => void;
  selectedTaskId: string | null;
  onAddTask?: (text: string) => Promise<void>;
  /** 查看草稿：从卡片菜单跳转到该 run 的草稿页 */
  onViewDraft?: (runId: string) => void;
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
  cancelled: "pending",
};

const COLUMNS_DEFINITION = [
  { id: "pending", title: "待触发", statusClass: "pending" },
  { id: "context", title: "上下文搜集", statusClass: "context" },
  { id: "draft", title: "草稿编辑", statusClass: "draft" },
  { id: "delivery", title: "待交付", statusClass: "delivery" },
  { id: "done", title: "已完成", statusClass: "done" },
];

type FilterValue = "all" | "email" | "doc" | "report" | "message" | "failed";

const FILTER_OPTIONS: { value: FilterValue; label: string }[] = [
  { value: "all", label: "全部" },
  { value: "email", label: "邮件" },
  { value: "doc", label: "文档" },
  { value: "report", label: "报告" },
  { value: "message", label: "消息" },
  { value: "failed", label: "失败" },
];

interface MenuState {
  runId: string;
  status: string;
  anchorRect: DOMRect;
}

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

function groupByColumn(runs: AgentRunItem[]): Record<string, AgentRunItem[]> {
  const grouped: Record<string, AgentRunItem[]> = {
    pending: [],
    context: [],
    draft: [],
    delivery: [],
    done: [],
  };
  for (const item of runs) {
    const colId = STATUS_MAP[item.status] || "pending";
    if (grouped[colId]) grouped[colId].push(item);
  }
  return grouped;
}

function applyFilter(items: AgentRunItem[], filter: FilterValue): AgentRunItem[] {
  if (filter === "all") return items;
  if (filter === "failed") return items.filter((i) => i.status === "failed");
  return items.filter((i) => getTagByTaskType(i.task_type || "").className === filter);
}

export function KanbanBoard({ onSelectTask, onDeselectTask, selectedTaskId, onAddTask, onViewDraft }: KanbanBoardProps) {
  const [columns, setColumns] = useState<KanbanColumn[]>(
    COLUMNS_DEFINITION.map((c) => ({ ...c, items: [] }))
  );
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [filterValue, setFilterValue] = useState<FilterValue>("all");
  const [filterOpen, setFilterOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [addText, setAddText] = useState("");
  const [adding, setAdding] = useState(false);
  const [menu, setMenu] = useState<MenuState | null>(null);
  const addInputRef = useRef<HTMLInputElement>(null);
  const filterBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (addOpen && addInputRef.current) {
      addInputRef.current.focus();
    }
  }, [addOpen]);

  // 是否存在活跃任务（pending/running）→ 决定是否启用轮询
  const hasActiveRuns = useMemo(
    () => columns.some((c) => c.items.some((i) => i.status === "pending" || i.status === "running")),
    [columns]
  );

  const loadRuns = useCallback(async () => {
    setError(null);
    try {
      const res = await listAgentRuns(50);
      const grouped = groupByColumn(res.runs);
      setColumns(COLUMNS_DEFINITION.map((c) => ({ ...c, items: grouped[c.id] || [] })));
      const last = res.runs[res.runs.length - 1];
      setCursor(last ? last.created_at : null);
      setHasMore(res.runs.length === 50);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  // 首次加载必定执行一次（usePolling 在 enabled=false 时不会 tick）
  useEffect(() => {
    loadRuns();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 活跃任务时 5s 轮询；页面不可见自动暂停
  const polling = usePolling(loadRuns, {
    interval: 5000,
    enabled: hasActiveRuns,
    immediate: false,
  });

  const handleLoadMore = async () => {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const res = await listAgentRuns(20, cursor);
      const grouped = groupByColumn(res.runs);
      setColumns((prev) =>
        prev.map((col) => ({ ...col, items: [...col.items, ...(grouped[col.id] || [])] }))
      );
      const last = res.runs[res.runs.length - 1];
      setCursor(last ? last.created_at : null);
      setHasMore(res.runs.length === 20);
    } catch (err) {
      showToast("加载更多失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setLoadingMore(false);
    }
  };

  const handleRetry = async (item: AgentRunItem) => {
    try {
      const res = await retryRun(item.id);
      showToast("已重试", `新 run: ${res.run_id.slice(0, 8)}...`, "success");
      polling.refresh();
    } catch (err) {
      showToast("重试失败", err instanceof Error ? err.message : String(err), "error");
    }
  };

  const handleDelete = async (item: AgentRunItem) => {
    try {
      await deleteRun(item.id);
      showToast("已删除", "任务及关联数据已清除", "success");
      polling.refresh();
    } catch (err) {
      showToast("删除失败", err instanceof Error ? err.message : String(err), "error");
    }
  };

  const handleArchive = async (item: AgentRunItem) => {
    try {
      await archiveRun(item.id);
      showToast("已归档", "任务已取消", "success");
      polling.refresh();
    } catch (err) {
      showToast("归档失败", err instanceof Error ? err.message : String(err), "error");
    }
  };

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
      polling.refresh();
      showToast("任务已创建", "Agent 已开始处理", "success");
    } catch (err) {
      showToast("创建失败", err instanceof Error ? err.message : String(err), "error");
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

  const handleOpenMenu = (e: React.MouseEvent, item: AgentRunItem) => {
    e.stopPropagation();
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setMenu({ runId: item.id, status: item.status, anchorRect: rect });
  };

  const filteredColumns = useMemo(
    () => columns.map((col) => ({ ...col, items: applyFilter(col.items, filterValue) })),
    [columns, filterValue]
  );

  const handleColumnsClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!onDeselectTask) return;
    const target = e.target as HTMLElement;
    if (target.closest(".kanban-card")) return;
    if (target.closest(".kanban-column-header")) return;
    if (target.closest(".kanban-add-form")) return;
    if (target.closest(".kanban-filter-dropdown")) return;
    if (target.closest(".card-menu")) return;
    if (target.closest(".kanban-load-more")) return;
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
          <div className="kanban-filter-wrapper">
            <button
              ref={filterBtnRef}
              className={`kanban-filter-btn ${filterValue !== "all" ? "active" : ""}`}
              onClick={(e) => { e.stopPropagation(); setFilterOpen(!filterOpen); }}
              title="筛选任务"
            >
              <IconFilter size={16} />
            </button>
            {filterOpen && (
              <div className="kanban-filter-dropdown" onClick={(e) => e.stopPropagation()}>
                {FILTER_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    className={`kanban-filter-option ${filterValue === opt.value ? "active" : ""}`}
                    onClick={() => { setFilterValue(opt.value); setFilterOpen(false); }}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            )}
          </div>
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
        {error ? (
          <div className="kanban-error-banner">
            <div className="kanban-error-text">加载失败：{error}</div>
            <button
              className="kanban-error-retry"
              onClick={(e) => { e.stopPropagation(); setLoading(true); polling.refresh(); }}
            >
              重试
            </button>
          </div>
        ) : (
          filteredColumns.map((col) => (
            <div key={col.id} className="kanban-column">
              <div className="kanban-column-header">
                <span className="kanban-column-title">
                  <span className={`status-dot ${col.statusClass}`} />
                  {col.title}
                </span>
                <span className="kanban-column-count">{col.items.length}</span>
              </div>
              <div className="kanban-column-cards">
                {loading ? (
                  Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="kanban-card skeleton">
                      <div className="kanban-card-skeleton-title" />
                      <div className="kanban-card-skeleton-meta" />
                    </div>
                  ))
                ) : col.items.length === 0 ? (
                  <div className="kanban-empty-column">暂无任务</div>
                ) : (
                  col.items.map((item) => {
                    const tag = getTagByTaskType(item.task_type || "");
                    const isFailed = item.status === "failed";
                    return (
                      <div
                        key={item.id}
                        className={`kanban-card ${selectedTaskId === item.id ? "selected" : ""} ${isFailed ? "failed" : ""}`}
                        onClick={(e) => { e.stopPropagation(); onSelectTask(item); }}
                      >
                        <div className="kanban-card-title">{item.trigger_source || item.task_type || "未命名任务"}</div>
                        <div className="kanban-card-meta">
                          <span className={`kanban-card-tag ${tag.className}`}>{tag.label}</span>
                          {isFailed && <span className="kanban-card-tag failed">失败</span>}
                          <span>{formatTime(item.created_at)}</span>
                        </div>
                        <button
                          className="kanban-card-menu-btn"
                          onClick={(e) => handleOpenMenu(e, item)}
                          aria-label="任务操作"
                          title="操作菜单"
                        >
                          <IconMore size={14} />
                        </button>
                      </div>
                    );
                  })
                )}
                {hasMore && col.id === "pending" && !loading && (
                  <button
                    className="kanban-load-more"
                    onClick={(e) => { e.stopPropagation(); handleLoadMore(); }}
                    disabled={loadingMore}
                  >
                    {loadingMore ? "加载中..." : "加载更多"}
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>
      {menu && (
        <CardMenu
          runId={menu.runId}
          status={menu.status}
          anchorRect={menu.anchorRect}
          onClose={() => setMenu(null)}
          onRetry={() => {
            const item = columns.flatMap((c) => c.items).find((i) => i.id === menu.runId);
            if (item) handleRetry(item);
          }}
          onDelete={() => {
            const item = columns.flatMap((c) => c.items).find((i) => i.id === menu.runId);
            if (item) handleDelete(item);
          }}
          onArchive={() => {
            const item = columns.flatMap((c) => c.items).find((i) => i.id === menu.runId);
            if (item) handleArchive(item);
          }}
          onViewDraft={onViewDraft ? () => onViewDraft(menu.runId) : undefined}
        />
      )}
    </>
  );
}
