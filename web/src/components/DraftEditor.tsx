import { useState } from "react";
import { type Draft } from "../api/client";
import { showToast } from "./Layout";
import { useRipple } from "../hooks/useRipple";

interface DraftEditorProps {
  draft: Draft;
  runId: string | null;
  onConfirmed: () => void;
  onNavigate: (page: number) => void;
}

export function DraftEditor({ draft, runId, onConfirmed, onNavigate }: DraftEditorProps) {
  const [content, setContent] = useState(draft.content ?? "");
  const [saving, setSaving] = useState(false);
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const ripple = useRipple();

  const handleSave = async () => {
    setSaving(true);
    try {
      await new Promise(r => setTimeout(r, 300));
      showToast("保存成功", "草稿已保存", "success");
    } catch (err) {
      showToast("保存失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setSaving(false);
    }
  };

  const handleConfirm = async () => {
    try {
      await new Promise(r => setTimeout(r, 300));
      showToast("确认成功", "草稿已确认，进入交付环节", "success");
      setShowConfirmModal(false);
      onConfirmed();
    } catch (err) {
      showToast("确认失败", err instanceof Error ? err.message : String(err), "error");
    }
  };

  const charCount = content.length;
  const lineCount = content.split("\n").length;

  if (!runId) {
    return (
      <div className="app-card-elevated p-8 md:p-10 text-center">
        <div className="w-14 h-14 rounded-full bg-zinc-800 flex items-center justify-center mx-auto mb-4">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#71717a" strokeWidth="1.8" strokeLinecap="round"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
        </div>
        <div className="text-zinc-400 text-sm mb-5">暂无可编辑的草稿</div>
        <button
          onClick={() => onNavigate(3)}
          className="px-6 py-3 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press hover:bg-emerald-400 transition-colors"
        >
          查看上下文搜集
        </button>
      </div>
    );
  }

  return (
    <div className="pb-32 md:pb-24">
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
              className="px-4 py-2.5 rounded-xl border border-white/10 text-sm text-zinc-300 hover:bg-zinc-800/60 btn-press ripple-container disabled:opacity-50"
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
      </div>

      <div className="app-card-elevated p-2 mb-5">
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
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
    </div>
  );
}
