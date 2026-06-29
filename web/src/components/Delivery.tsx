import { useState } from "react";
import { deliverDraft, type Draft } from "../api/client";
import { useRipple } from "../hooks/useRipple";

interface DeliveryProps {
  draft: Draft;
  runId: string | null;
  onNavigate: (page: number) => void;
}

export function Delivery({ draft, runId, onNavigate }: DeliveryProps) {
  const [selectedTarget, setSelectedTarget] = useState<"notion" | "obsidian" | null>(null);
  const [delivering, setDelivering] = useState(false);
  const [deliverResult, setDeliverResult] = useState<{ status: string; target_url?: string } | null>(null);
  const ripple = useRipple();

  const handleDeliver = async () => {
    if (!selectedTarget || !runId) return;
    setDelivering(true);
    try {
      const result = await deliverDraft(runId, selectedTarget);
      setDeliverResult({ status: result.status, target_url: result.target_url });
    } catch (err) {
      setDeliverResult({ status: "failed" });
    } finally {
      setDelivering(false);
    }
  };

  const targets = [
    {
      key: "notion" as const,
      name: "Notion",
      desc: "同步到 Notion 数据库",
      icon: <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 7v10M12 7v10M16 7v10"/></svg>,
    },
    {
      key: "obsidian" as const,
      name: "Obsidian",
      desc: "保存为 Markdown 文件",
      icon: <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M14.5 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V7.5L14.5 2z"/><polyline points="14,2 14,8 20,8"/></svg>,
    },
  ];

  return (
    <div className="pb-28 md:pb-24">
      <div className="mb-8">
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">交付通知</h1>
        <p className="text-zinc-500 text-sm mt-1.5">选择交付目标并确认发送</p>
      </div>

      {!runId ? (
        <div className="app-card-elevated p-8 md:p-10 text-center">
          <div className="w-14 h-14 rounded-full bg-zinc-800 flex items-center justify-center mx-auto mb-4">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#71717a" strokeWidth="1.8" strokeLinecap="round"><path d="M22 2L11 13"/><path d="M22 2l-7 20-4-9-9-4 20-7z"/></svg>
          </div>
          <div className="text-zinc-400 text-sm mb-5">暂无可交付的草稿</div>
          <button
            onClick={() => onNavigate(4)}
            className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press hover:bg-emerald-400 transition-colors"
          >
            前往编辑草稿
          </button>
        </div>
      ) : (
        <>
          <div className="app-card p-6 mb-5">
            <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-5">交付目标</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {targets.map((t) => (
                <button
                  key={t.key}
                  onClick={(e) => { ripple(e); setSelectedTarget(t.key); }}
                  className={`p-5 rounded-xl transition-all duration-200 ripple-container text-left ${
                    selectedTarget === t.key
                      ? "bg-emerald-500/10 border border-emerald-500/30"
                      : "bg-black/20 border border-white/5 hover:border-white/10 hover:bg-zinc-800/40"
                  }`}
                >
                  <div className="flex items-center gap-3 mb-2">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                      selectedTarget === t.key ? "bg-emerald-500/20 text-emerald-400" : "bg-zinc-800 text-zinc-400"
                    }`}>{t.icon}</div>
                    <span className="text-sm font-medium text-zinc-200">{t.name}</span>
                    {selectedTarget === t.key && (
                      <svg className="ml-auto" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round"><path d="M2 8l4 4 8-8"/></svg>
                    )}
                  </div>
                  <div className="text-xs text-zinc-500 ml-0">{t.desc}</div>
                </button>
              ))}
            </div>
          </div>

          <div className="app-card p-6 mb-5">
            <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-4">交付预览</h2>
            <div className="bg-black/30 rounded-xl p-5 border border-white/5 text-sm text-zinc-300 leading-relaxed max-h-[200px] overflow-y-auto font-mono">
              {draft.content ? (
                <div className="whitespace-pre-wrap">{draft.content.slice(0, 500)}{draft.content.length > 500 ? "..." : ""}</div>
              ) : (
                <span className="text-zinc-600">暂无草稿内容</span>
              )}
            </div>
          </div>

          <button
            onClick={(e) => { ripple(e); handleDeliver(); }}
            disabled={!selectedTarget || delivering}
            className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 hover:bg-emerald-400 transition-colors"
          >
            {delivering ? (
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            ) : (
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M2 8l4 4 8-8"/></svg>
            )}
            确认交付
          </button>

          {deliverResult && (
            <div className={`app-card p-5 mt-5 ${
              deliverResult.status === "success" ? "border-emerald-500/20" : "border-red-500/20"
            }`}>
              {deliverResult.status === "success" ? (
                <div>
                  <div className="flex items-center gap-2 mb-2">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round"><path d="M2 8l4 4 8-8"/></svg>
                    <span className="text-sm text-emerald-400 font-medium">交付成功</span>
                  </div>
                  {deliverResult.target_url && (
                    <div className="text-xs text-zinc-500">
                      目标地址：<a href={deliverResult.target_url} target="_blank" rel="noopener" className="text-emerald-400 hover:underline">{deliverResult.target_url}</a>
                    </div>
                  )}
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="#ef4444" strokeWidth="2" strokeLinecap="round"><circle cx="8" cy="8" r="6"/><line x1="8" y1="5" x2="8" y2="8.5"/><circle cx="8" cy="11" r="0.5" fill="currentColor"/></svg>
                  <span className="text-sm text-red-400 font-medium">交付失败，请重试</span>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
