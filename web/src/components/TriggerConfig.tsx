import { useEffect, useState, useCallback } from "react";
import { triggerAgent, listKeywords, listDataSources, type KeywordItem, type DataSourceItem } from "../api/client";
import { useRipple } from "../hooks/useRipple";
import type { PageKey } from "./Layout";

interface TriggerConfigProps {
  onTrigger: (runId: string) => void;
  onError: (msg: string | null) => void;
  onNavigate: (page: PageKey) => void;
}

function formatLastSync(iso?: string | null): string {
  if (!iso) return "从未同步";
  const d = new Date(iso);
  const now = Date.now();
  const diff = (now - d.getTime()) / 1000;
  if (diff < 60) return "刚刚同步";
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前同步`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前同步`;
  return `${Math.floor(diff / 86400)} 天前同步`;
}

const STATUS_BADGE: Record<string, { label: string; color: string; dot: string }> = {
  active: { label: "已连接", color: "text-emerald-500 dark:text-emerald-400", dot: "bg-emerald-500" },
  connected: { label: "已连接", color: "text-emerald-500 dark:text-emerald-400", dot: "bg-emerald-500" },
  inactive: { label: "未连接", color: "text-[var(--text-muted)]", dot: "bg-[var(--text-muted)]" },
  error: { label: "异常", color: "text-red-500 dark:text-red-400", dot: "bg-red-500" },
};

export function TriggerConfig({ onTrigger, onError, onNavigate }: TriggerConfigProps) {
  const [triggerText, setTriggerText] = useState("");
  const [triggering, setTriggering] = useState(false);
  const [testResult, setTestResult] = useState<"idle" | "testing" | "success" | "failed">("idle");

  const [keywords, setKeywords] = useState<KeywordItem[]>([]);
  const [dataSources, setDataSources] = useState<DataSourceItem[]>([]);
  const [loading, setLoading] = useState(true);
  const ripple = useRipple();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [kws, dss] = await Promise.all([listKeywords(), listDataSources()]);
      setKeywords(kws);
      setDataSources(dss);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [onError]);

  useEffect(() => {
    load();
  }, [load]);

  const handleTestTrigger = async () => {
    if (!triggerText.trim()) return;
    setTriggering(true);
    setTestResult("testing");
    onError(null);
    try {
      const res = await triggerAgent(triggerText.trim());
      onTrigger(res.run_id);
      setTestResult("success");
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
      setTestResult("failed");
    } finally {
      setTriggering(false);
    }
  };

  return (
    <div className="pb-28 md:pb-24">
      <div className="mb-8">
        <h1 className="page-title">触发规则</h1>
        <p className="page-subtitle">配置关键词自动触发规则和数据源，当匹配到关键词时自动启动 Agent 任务</p>
      </div>

      <div className="mb-8">
        <h2 className="section-title mb-4">手动触发</h2>
        <p className="text-xs text-[var(--text-secondary)] mb-3">输入一段文本模拟触发，Agent会匹配关键词规则并执行对应的任务流程</p>
        <div className="config-card p-5">
          <div className="flex flex-col sm:flex-row gap-3 mb-4">
            <input
              type="text"
              value={triggerText}
              onChange={(e) => setTriggerText(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") handleTestTrigger(); }}
              placeholder="输入触发文本进行测试..."
              className="flex-1 px-4 py-3 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-subtle)] text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-emerald-500/30"
            />
            <button
              onClick={(e) => { ripple(e); handleTestTrigger(); }}
              disabled={triggering || !triggerText.trim()}
              className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 shrink-0 hover:bg-emerald-400 transition-colors"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M8 2v4M8 10v4M2 8h4M10 8h4"/></svg>
              测试触发
            </button>
          </div>

          {testResult === "testing" && (
            <div className="app-card p-5 space-y-3">
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                <span className="text-sm text-amber-500 dark:text-amber-400">触发测试进行中...</span>
              </div>
              <div className="w-full h-1.5 bg-[var(--bg-tertiary)] rounded-full overflow-hidden">
                <div className="h-full progress-gradient rounded-full" style={{ width: "60%" }} />
              </div>
            </div>
          )}

          {testResult === "success" && (
            <div className="app-card p-5 border-emerald-500/20">
              <div className="flex items-center gap-2 mb-2">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round"><path d="M2 8l4 4 8-8"/></svg>
                <span className="text-sm text-emerald-500 dark:text-emerald-400 font-medium">触发测试成功</span>
              </div>
              <div className="text-xs text-[var(--text-secondary)] mb-3">关键词匹配成功，上下文搜集已启动。</div>
              <button
                onClick={(e) => { ripple(e); onNavigate("context"); }}
                className="px-4 py-2 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-sm font-medium btn-press ripple-container"
              >
                查看上下文搜集
              </button>
            </div>
          )}

          {testResult === "failed" && (
            <div className="app-card p-5 border-red-500/20">
              <div className="flex items-center gap-2">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#ef4444" strokeWidth="2" strokeLinecap="round"><circle cx="8" cy="8" r="6"/><line x1="8" y1="5" x2="8" y2="8.5"/><circle cx="8" cy="11" r="0.5" fill="currentColor"/></svg>
                <span className="text-sm text-red-500 dark:text-red-400 font-medium">触发测试失败</span>
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="mb-8">
        <div className="flex items-center justify-between mb-2">
          <h2 className="section-title">关键词规则</h2>
          <button
            onClick={(e) => { ripple(e); load(); }}
            disabled={loading}
            className="text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)] btn-press ripple-container"
          >
            {loading ? "加载中..." : "刷新"}
          </button>
        </div>
        <p className="text-xs text-[var(--text-secondary)] mb-4">当你输入或收到包含这些关键词的消息时，会自动触发对应的 Agent 任务</p>
        {loading && keywords.length === 0 ? (
          <div className="text-center py-8 text-sm text-[var(--text-muted)]">加载中...</div>
        ) : keywords.length === 0 ? (
          <div className="app-card p-10 text-center">
            <div className="text-sm text-[var(--text-muted)]">暂未配置关键词规则</div>
          </div>
        ) : (
          <div className="space-y-2 stagger-list">
            {keywords.map((kw) => (
              <div key={kw.pattern} className="list-item">
                <div className="w-8 h-8 rounded-lg bg-emerald-500/10 text-emerald-500 dark:text-emerald-400 flex items-center justify-center shrink-0">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><path d="M2 4h12M2 8h12M2 12h12"/></svg>
                </div>
                <span className="text-sm font-medium text-[var(--text-primary)] flex-1 truncate">{kw.pattern}</span>
                <span className="text-[10px] text-[var(--text-muted)] px-2 py-1 rounded-md bg-[var(--bg-elevated)] shrink-0">
                  → {kw.task_type}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div>
        <h2 className="section-title mb-2">数据源</h2>
        <p className="text-xs text-[var(--text-secondary)] mb-4">Agent 在执行任务时从这些数据源获取上下文信息（如 GitHub 仓库、邮件、日历等）</p>
        {loading && dataSources.length === 0 ? (
          <div className="text-center py-8 text-sm text-[var(--text-muted)]">加载中...</div>
        ) : dataSources.length === 0 ? (
          <div className="app-card p-10 text-center">
            <div className="text-sm text-[var(--text-muted)]">尚未绑定任何数据源</div>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3 stagger-list">
            {dataSources.map((ds) => {
              const badge = STATUS_BADGE[ds.status] ?? STATUS_BADGE.inactive;
              return (
                <div key={ds.id} className="app-card p-5 card-lift">
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-lg bg-[var(--bg-tertiary)] flex items-center justify-center text-sm font-semibold text-[var(--text-secondary)]">
                        {ds.name.charAt(0).toUpperCase()}
                      </div>
                      <div>
                        <div className="text-sm font-medium text-[var(--text-primary)]">{ds.name}</div>
                        <div className="text-[10px] text-[var(--text-muted)] uppercase mt-0.5">{ds.type}</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <span className={`w-1.5 h-1.5 rounded-full ${badge.dot}`} />
                      <span className={`text-xs ${badge.color}`}>{badge.label}</span>
                    </div>
                  </div>
                  <div className="text-[11px] text-[var(--text-muted)] pt-3 border-t border-[var(--border-subtle)]">
                    {formatLastSync(ds.last_sync_at)}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
