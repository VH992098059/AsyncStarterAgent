import type { Draft } from "../api/client";

interface ContextCollectionProps {
  draft: Draft;
  runId: string | null;
  onNavigate: (page: number) => void;
}

export function ContextCollection({ draft, runId, onNavigate }: ContextCollectionProps) {
  const isStreaming = runId !== null && draft.completeness < 1;

  return (
    <div className="pb-28 md:pb-24">
      <div className="mb-8">
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">上下文搜集</h1>
        <p className="text-zinc-500 text-sm mt-1.5">
          {runId ? (
            <>任务 <span className="text-emerald-400 font-mono text-xs">{runId.slice(0, 12)}</span></>
          ) : (
            "暂无运行中的任务"
          )}
        </p>
      </div>

      {!runId ? (
        <div className="app-card-elevated p-8 md:p-10 text-center">
          <div className="w-14 h-14 rounded-full bg-zinc-800 flex items-center justify-center mx-auto mb-4">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#71717a" strokeWidth="1.8" strokeLinecap="round"><circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/></svg>
          </div>
          <div className="text-zinc-400 text-sm mb-5">暂无运行中的 Agent 任务</div>
          <button
            onClick={() => onNavigate(2)}
            className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press hover:bg-emerald-400 transition-colors"
          >
            前往触发
          </button>
        </div>
      ) : (
        <>
          <div className="app-card-elevated p-6 mb-5">
            <div className="flex items-center gap-6 flex-wrap mb-4">
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full ${isStreaming ? "bg-amber-500 animate-pulse" : "bg-emerald-500"}`} />
                <span className="text-sm text-zinc-300">{isStreaming ? "搜集进行中" : "搜集完成"}</span>
              </div>
              <div className="flex-1" />
              <span className="text-sm text-zinc-500 font-medium">
                {Math.round(draft.completeness * 100)}%
              </span>
            </div>
            <div className="w-full h-1.5 bg-zinc-800 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-700 ease-out ${isStreaming ? "progress-gradient" : "bg-emerald-500"}`}
                style={{ width: `${Math.round(draft.completeness * 100)}%` }}
              />
            </div>
          </div>

          {draft.content && (
            <div className="app-card p-6 mb-5">
              <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-4">草稿预览</h2>
              <div className="font-mono text-xs text-zinc-400 bg-black/30 rounded-xl p-5 border border-white/5 whitespace-pre-wrap max-h-[400px] overflow-y-auto">
                {draft.content}
                {isStreaming && <span className="typing-cursor" />}
              </div>
            </div>
          )}

          {draft.marks.length > 0 && (
            <div className="app-card p-6 mb-5">
              <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-4">
                待补充标记 <span className="text-zinc-600 font-normal">({draft.marks.length})</span>
              </h2>
              <div className="space-y-2">
                {draft.marks.map((m) => (
                  <div key={m.id} className="list-item">
                    <span className="w-1.5 h-1.5 rounded-full bg-amber-500 shrink-0" />
                    <span className="text-sm text-zinc-300 flex-1">{m.hint}</span>
                    <span className={`text-xs ${m.resolved ? "text-emerald-400" : "text-amber-400"} shrink-0`}>
                      {m.resolved ? "已解决" : "待补充"}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {!isStreaming && draft.content && (
            <button
              onClick={() => onNavigate(4)}
              className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press flex items-center gap-2 hover:bg-emerald-400 transition-colors"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M3 3h10v10H3z"/><path d="M3 6h10"/></svg>
              编辑草稿
            </button>
          )}
        </>
      )}
    </div>
  );
}
