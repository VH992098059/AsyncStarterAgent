import { useEffect, useState, useCallback } from "react";
import { triggerAgent, listAgentRuns, type Draft, type AgentRunItem, type AgentRunStats } from "../api/client";
import { useRipple } from "../hooks/useRipple";

interface DashboardProps {
  draft: Draft;
  runId: string | null;
  isStreaming: boolean;
  error: string | null;
  onNavigate: (page: number) => void;
  onTrigger: (runId: string) => void;
  onQuickTrigger: (text: string) => void;
  onError: (msg: string | null) => void;
}

const STATUS_COLORS: Record<string, string> = {
  pending: "bg-zinc-800 text-zinc-400",
  running: "bg-amber-500/15 text-amber-400",
  completed: "bg-emerald-500/15 text-emerald-400",
  failed: "bg-red-500/15 text-red-400",
};

const STATUS_LABEL: Record<string, string> = {
  pending: "等待中",
  running: "运行中",
  completed: "已完成",
  failed: "失败",
};

function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = Date.now();
  const diff = (now - d.getTime()) / 1000;
  if (diff < 60) return "刚刚";
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  return d.toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 12) return "早上好";
  if (hour < 14) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
}

function statsToArray(stats: Record<string, number> | undefined): Array<{ key: string; count: number }> {
  if (!stats) return [];
  return Object.entries(stats)
    .map(([key, count]) => ({ key, count }))
    .sort((a, b) => b.count - a.count);
}

const quickActions = [
  { id: 2, label: "配置触发器", desc: "管理关键词规则", icon: <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M12 3v6l5 3"/><circle cx="12" cy="12" r="9"/></svg> },
  { id: 3, label: "上下文搜集", desc: "查看数据源状态", icon: <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3"/><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1"/></svg> },
  { id: 4, label: "草稿箱", desc: "编辑生成内容", icon: <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M4 4h16v16H4z"/><path d="M4 9h16M9 9v11"/></svg> },
  { id: 5, label: "交付通知", desc: "查看发送历史", icon: <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12l9 9 9-9"/><path d="M12 3v18"/></svg> },
];

export function Dashboard({ isStreaming, runId, error, onNavigate, onTrigger, onError }: DashboardProps) {
  const [triggerText, setTriggerText] = useState("");
  const [triggering, setTriggering] = useState(false);
  const [runs, setRuns] = useState<AgentRunItem[]>([]);
  const [stats, setStats] = useState<AgentRunStats | null>(null);
  const [loadingRuns, setLoadingRuns] = useState(true);
  const ripple = useRipple();

  const loadRuns = useCallback(async () => {
    setLoadingRuns(true);
    try {
      const res = await listAgentRuns(20);
      setRuns(res.runs);
      setStats(res.stats);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingRuns(false);
    }
  }, [onError]);

  useEffect(() => {
    loadRuns();
  }, [loadRuns]);

  useEffect(() => {
    if (runId) loadRuns();
  }, [runId, loadRuns]);

  const handleTrigger = async () => {
    if (!triggerText.trim()) return;
    setTriggering(true);
    onError(null);
    try {
      const res = await triggerAgent(triggerText.trim());
      onTrigger(res.run_id);
      onNavigate(3);
      setTriggerText("");
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setTriggering(false);
    }
  };

  const totalAll = statsToArray(stats?.total).reduce((s, x) => s + x.count, 0);
  const todayAll = statsToArray(stats?.today).reduce((s, x) => s + x.count, 0);
  const greeting = getGreeting();

  return (
    <div className="pb-28 md:pb-24">
      {/* Greeting Hero */}
      <div className="mb-8">
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">{greeting}</h1>
        <p className="text-zinc-500 mt-1.5">有什么我可以帮你的吗？</p>
      </div>

      {/* Quick Trigger - prominent like search bar */}
      <div className="app-card-elevated p-5 md:p-6 mb-8 animate-fade-in-up">
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <svg className="absolute left-4 top-1/2 -translate-y-1/2 text-zinc-500" width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><path d="M9 2v4M9 12v4M2 9h4M12 9h4"/><circle cx="9" cy="9" r="7"/></svg>
            <input
              type="text"
              value={triggerText}
              onChange={(e) => setTriggerText(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") handleTrigger(); }}
              placeholder="试试说：'写周报'、'总结会议'、'帮我规划一下'..."
              className="w-full pl-11 pr-4 py-3.5 rounded-xl bg-black/30 border border-white/5 text-sm text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-emerald-500/30 focus:bg-black/40"
            />
          </div>
          <button
            onClick={(e) => { ripple(e); handleTrigger(); }}
            disabled={triggering || !triggerText.trim()}
            className="px-6 py-3.5 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 shrink-0 hover:bg-emerald-400 transition-colors"
          >
            {triggering ? (
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M3 8l10 0M8 3l0 10"/></svg>
            )}
            触发 Agent
          </button>
        </div>
        {error && <div className="mt-3 text-sm text-red-400">{error}</div>}
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-8">
        <div className="app-card p-4 animate-fade-in-up stagger-1">
          <div className="text-xs text-zinc-500 mb-2">今日运行</div>
          <div className="text-2xl font-semibold text-zinc-100">{todayAll}</div>
        </div>
        <div className="app-card p-4 animate-fade-in-up stagger-2">
          <div className="text-xs text-zinc-500 mb-2">累计运行</div>
          <div className="text-2xl font-semibold text-zinc-100">{totalAll}</div>
        </div>
        <div className="app-card p-4 animate-fade-in-up stagger-3">
          <div className="text-xs text-zinc-500 mb-2">已完成</div>
          <div className="text-2xl font-semibold text-emerald-400">
            {stats?.total?.completed ?? 0}
          </div>
        </div>
        <div className="app-card p-4 animate-fade-in-up stagger-4">
          <div className="text-xs text-zinc-500 mb-2">失败</div>
          <div className="text-2xl font-semibold text-red-400">
            {stats?.total?.failed ?? 0}
          </div>
        </div>
      </div>

      {/* Quick Actions */}
      <div className="mb-8">
        <h2 className="section-heading mb-4">快捷入口</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {quickActions.map((action, i) => (
            <button
              key={action.id}
              onClick={(e) => { ripple(e); onNavigate(action.id); }}
              className="app-card card-lift p-5 text-left ripple-container animate-fade-in-up"
              style={{ animationDelay: `${0.05 + i * 0.05}s` }}
            >
              <div className="w-10 h-10 rounded-lg bg-emerald-500/10 text-emerald-400 flex items-center justify-center mb-3">
                {action.icon}
              </div>
              <div className="text-sm font-medium text-zinc-200">{action.label}</div>
              <div className="text-xs text-zinc-500 mt-1">{action.desc}</div>
            </button>
          ))}
        </div>
      </div>

      {/* Recent Runs */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="section-heading">最近运行</h2>
          <button
            onClick={(e) => { ripple(e); loadRuns(); }}
            disabled={loadingRuns}
            className="text-xs text-zinc-500 hover:text-zinc-300 btn-press ripple-container"
          >
            {loadingRuns ? "加载中..." : "刷新"}
          </button>
        </div>

        {loadingRuns && runs.length === 0 ? (
          <div className="text-center py-12 text-sm text-zinc-500">加载中...</div>
        ) : runs.length === 0 ? (
          <div className="app-card p-12 text-center">
            <div className="w-12 h-12 rounded-full bg-zinc-900 flex items-center justify-center mx-auto mb-3">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#52525b" strokeWidth="1.8" strokeLinecap="round"><path d="M12 3v6l5 3"/><circle cx="12" cy="12" r="9"/></svg>
            </div>
            <div className="text-sm text-zinc-400 mb-1">还没有运行记录</div>
            <div className="text-xs text-zinc-600">在上方输入指令，或在触发器页面配置规则</div>
          </div>
        ) : (
          <div className="space-y-2 stagger-list">
            {runs.slice(0, 8).map((r) => (
              <div
                key={r.id}
                className="list-item ripple-container cursor-pointer"
                onClick={(e) => { ripple(e); onNavigate(3); }}
              >
                <span
                  className={`px-2 py-1 rounded-md text-[10px] font-medium shrink-0 ${STATUS_COLORS[r.status] ?? "bg-zinc-800 text-zinc-400"}`}
                >
                  {STATUS_LABEL[r.status] ?? r.status}
                </span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-zinc-200 truncate">
                    {r.task_type || r.trigger_type || "未分类任务"}
                  </div>
                  <div className="text-xs text-zinc-500 mt-0.5 truncate">
                    {r.trigger_source || "手动触发"} · {formatTime(r.created_at)}
                  </div>
                </div>
                <span className="text-[10px] text-zinc-600 font-mono shrink-0">
                  #{r.id.slice(0, 6)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
