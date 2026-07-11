import { useState, useEffect } from "react";
import { type Draft, type Mark, updateDraft, resolveMark } from "../api/client";
import { showToast } from "./Layout";
import { Breadcrumb } from "./Breadcrumb";
import type { PageKey } from "./Layout";
import { useRipple } from "../hooks/useRipple";
import { useKeyboardShortcut } from "../hooks/useKeyboardShortcut";

interface DraftEditorProps {
  draft: Draft;
  runId: string | null;
  onConfirmed: () => void;
  onNavigate: (page: PageKey) => void;
}

export function DraftEditor({ draft, runId, onConfirmed, onNavigate }: DraftEditorProps) {
  const [content, setContent] = useState(draft.content ?? "");
  const [marks, setMarks] = useState<Mark[]>(draft.marks ?? []);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [resolvingMark, setResolvingMark] = useState<Mark | null>(null);
  const [resolveValue, setResolveValue] = useState("");
  const [resolving, setResolving] = useState(false);
  const ripple = useRipple();

  // SSE 流式更新 draft.content/marks 时同步到本地编辑态（仅在未手动编辑时同步，避免覆盖用户输入）
  useEffect(() => {
    if (!dirty) {
      setContent(draft.content ?? "");
      setMarks(draft.marks ?? []);
    }
  }, [draft.content, draft.marks, dirty]);

  // beforeunload：有未保存改动时警告
  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);

  const handleSave = async () => {
    if (!runId) return;
    setSaving(true);
    try {
      await updateDraft(runId, content);
      setDirty(false);
      showToast("保存成功", "草稿已保存", "success");
    } catch (err) {
      showToast("保存失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setSaving(false);
    }
  };

  const handleConfirm = async () => {
    if (!runId) return;
    try {
      await updateDraft(runId, content);
      setDirty(false);
      showToast("确认成功", "草稿已确认，进入交付环节", "success");
      setShowConfirmModal(false);
      onConfirmed();
    } catch (err) {
      showToast("确认失败", err instanceof Error ? err.message : String(err), "error");
    }
  };

  // marks 集成：点击未解决的 mark chip → 弹输入框 → 调 resolveMark → 替换 content
  const handleResolveMark = async () => {
    if (!runId || !resolvingMark) return;
    const value = resolveValue.trim();
    if (!value) return;
    setResolving(true);
    try {
      const { markdown } = await resolveMark(runId, resolvingMark.id, value);
      setContent(markdown);
      setMarks((prev) =>
        prev.map((m) => (m.id === resolvingMark.id ? { ...m, resolved: true } : m))
      );
      setDirty(true);
      setResolvingMark(null);
      setResolveValue("");
      showToast("标记已解决", "草稿已更新，记得保存", "success");
    } catch (err) {
      showToast("解决失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setResolving(false);
    }
  };

  // Ctrl+S / Cmd+S 保存
  useKeyboardShortcut("mod+s", (e) => {
    e.preventDefault();
    if (runId && dirty && !saving) {
      void handleSave();
    }
  }, [runId, content, dirty, saving]);

  const charCount = content.length;
  const lineCount = content.split("\n").length;
  const unresolvedCount = marks.filter((m) => !m.resolved).length;

  if (!runId) {
    return (
      <div className="app-card-elevated p-8 md:p-10 text-center">
        <div className="w-14 h-14 rounded-full bg-zinc-800 flex items-center justify-center mx-auto mb-4">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#71717a" strokeWidth="1.8" strokeLinecap="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </div>
        <div className="text-zinc-400 text-sm mb-5">暂无可编辑的草稿</div>
        <button
          onClick={() => onNavigate("context")}
          className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press hover:bg-emerald-400 transition-colors"
        >
          查看上下文搜集
        </button>
      </div>
    );
  }

  return (
    <div className="pb-32 md:pb-24">
      <Breadcrumb title="草稿编辑" runId={runId} onBackToBoard={() => onNavigate("board")} />
      <div className="mb-6">
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">草稿编辑</h1>
            <p className="text-zinc-500 text-sm mt-1.5">编辑、微调后确认交付</p>
          </div>
          <div className="flex gap-2 shrink-0">
            <button
              onClick={(e) => { ripple(e); handleSave(); }}
              disabled={saving}
              className={`px-4 py-2.5 rounded-xl border text-sm btn-press ripple-container disabled:opacity-50 transition-colors ${
                dirty
                  ? "border-emerald-500/40 text-emerald-400 hover:bg-emerald-500/10"
                  : "border-white/10 text-zinc-300 hover:bg-zinc-800/60"
              }`}
            >
              {saving ? "保存中..." : "保存草稿"}
            </button>
            <button
              onClick={(e) => { ripple(e); setShowConfirmModal(true); }}
              className="px-5 py-2.5 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 hover:bg-emerald-400 transition-colors"
            >
              确认草稿
            </button>
          </div>
        </div>
        {dirty && (
          <div className="text-xs text-amber-400 mt-2">有未保存的改动（Ctrl+S 保存）</div>
        )}
      </div>

      {marks.length > 0 && (
        <div className="app-card p-4 mb-5">
          <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-3">
            待补充标记
            <span className="text-zinc-600 font-normal ml-1">
              ({unresolvedCount} 待处理 / {marks.length} 总计)
            </span>
          </h2>
          <div className="flex flex-wrap gap-2">
            {marks.map((m) => (
              <button
                key={m.id}
                onClick={() => {
                  if (!m.resolved) {
                    setResolvingMark(m);
                    setResolveValue("");
                  }
                }}
                disabled={m.resolved}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all btn-press ${
                  m.resolved
                    ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 cursor-default"
                    : "bg-amber-500/10 text-amber-400 border border-amber-500/20 hover:bg-amber-500/20"
                }`}
                title={m.resolved ? "已解决" : `点击补充: ${m.hint}`}
              >
                {m.resolved ? "已解决 " : ""}{m.hint}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="app-card-elevated p-2 mb-5">
        <textarea
          value={content}
          onChange={(e) => { setContent(e.target.value); setDirty(true); }}
          className="w-full h-[50vh] min-h-[400px] bg-transparent text-sm text-zinc-200 font-mono p-4 rounded-xl resize-none focus:outline-none placeholder:text-zinc-600 leading-relaxed"
          placeholder="草稿内容将在这里展示..."
        />
      </div>

      <div className="flex items-center justify-between text-xs text-zinc-500">
        <div className="flex items-center gap-4">
          <span>{charCount} 字符</span>
          <span>{lineCount} 行</span>
          <span className="px-2 py-0.5 rounded bg-zinc-800 text-zinc-400">Markdown</span>
        </div>
      </div>

      {showConfirmModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 modal-overlay">
          <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" onClick={() => setShowConfirmModal(false)} />
          <div className="relative app-card-elevated p-6 w-full max-w-sm shadow-2xl animate-fade-in-up">
            <div className="w-12 h-12 rounded-full bg-emerald-500/10 flex items-center justify-center mx-auto mb-4">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="2" strokeLinecap="round"><path d="M2 8l4 4 8-8"/><circle cx="12" cy="12" r="10"/></svg>
            </div>
            <h3 className="text-lg font-semibold text-center mb-2">确认草稿？</h3>
            <p className="text-sm text-zinc-500 text-center mb-6">
              确认后草稿将进入交付环节，可选择发送到 Notion 或 Obsidian。
            </p>
            <div className="flex gap-3">
              <button
                onClick={(e) => { ripple(e); setShowConfirmModal(false); }}
                className="flex-1 px-4 py-3 rounded-xl border border-white/10 text-sm font-medium btn-press ripple-container"
              >
                取消
              </button>
              <button
                onClick={(e) => { ripple(e); handleConfirm(); }}
                className="flex-1 px-4 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container flex items-center justify-center gap-2 hover:bg-emerald-400 transition-colors"
              >
                确认并交付
              </button>
            </div>
          </div>
        </div>
      )}

      {resolvingMark && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 modal-overlay">
          <div
            className="absolute inset-0 bg-black/70 backdrop-blur-sm"
            onClick={() => { if (!resolving) setResolvingMark(null); }}
          />
          <div className="relative app-card-elevated p-6 w-full max-w-sm shadow-2xl animate-fade-in-up">
            <h3 className="text-lg font-semibold mb-2">补充标记</h3>
            <p className="text-sm text-zinc-500 mb-4">
              为 <span className="text-amber-400 font-mono">[{resolvingMark.hint}]</span> 填入内容，将替换草稿中的占位符。
            </p>
            <textarea
              value={resolveValue}
              onChange={(e) => setResolveValue(e.target.value)}
              placeholder="输入替换内容..."
              rows={3}
              className="w-full bg-zinc-900/50 border border-white/10 rounded-xl p-3 text-sm text-zinc-200 resize-none focus:outline-none focus:border-emerald-500/30 mb-4"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault();
                  if (!resolving && resolveValue.trim()) {
                    void handleResolveMark();
                  }
                }
              }}
            />
            <div className="flex gap-3">
              <button
                onClick={(e) => { ripple(e); setResolvingMark(null); }}
                disabled={resolving}
                className="flex-1 px-4 py-3 rounded-xl border border-white/10 text-sm font-medium btn-press ripple-container disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={(e) => { ripple(e); handleResolveMark(); }}
                disabled={resolving || !resolveValue.trim()}
                className="flex-1 px-4 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 hover:bg-emerald-400 transition-colors flex items-center justify-center gap-2"
              >
                {resolving ? "解决中..." : "确认替换"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
