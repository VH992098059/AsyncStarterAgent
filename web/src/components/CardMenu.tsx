import { useState, useRef, useEffect } from "react";
import { useClickOutside } from "../hooks/useClickOutside";
import { showToast } from "./Layout";
import { IconRetry, IconTrash, IconCopy, IconArchive, IconDraft } from "./Icons";

export interface CardMenuProps {
  /** 卡片对应的 run id */
  runId: string;
  /** 当前 run 状态，用于决定哪些操作可用 */
  status: string;
  /** 触发按钮（三点）的 getBoundingClientRect，用于定位菜单 */
  anchorRect: DOMRect;
  onClose: () => void;
  onRetry: () => void;
  onDelete: () => void;
  onArchive: () => void;
  /** 查看草稿：跳转到该 run 的草稿页 */
  onViewDraft?: () => void;
}

/**
 * CardMenu 看板卡片的操作菜单，fixed 定位（沿用 UserMenu 模式避免 overflow 裁剪）。
 * 菜单项：重试 / 复制 ID / 归档 / 删除（inline 二次确认，不用 window.confirm）。
 * 删除二次确认：第一次点"删除"展开"确认删除？" + 是/否。
 */
export function CardMenu({ runId, status, anchorRect, onClose, onRetry, onDelete, onArchive, onViewDraft }: CardMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useClickOutside(menuRef, () => onClose(), []);

  useEffect(() => {
    // 菜单宽度约 180px，弹在按钮左下方，避免溢出右边
    const menuWidth = 180;
    const menuHeight = 200;
    let left = anchorRect.right - menuWidth;
    let top = anchorRect.bottom + 4;
    if (left < 8) left = 8;
    if (top + menuHeight > window.innerHeight) {
      top = Math.max(8, anchorRect.top - menuHeight - 4);
    }
    setPos({ top, left });
  }, [anchorRect]);

  const handleCopyId = async () => {
    try {
      await navigator.clipboard.writeText(runId);
      showToast("已复制", "Run ID 已写入剪贴板", "success");
    } catch {
      showToast("复制失败", "剪贴板不可用，请手动复制", "error");
    }
    onClose();
  };

  const handleRetry = () => {
    onRetry();
    onClose();
  };

  const handleArchive = () => {
    onArchive();
    onClose();
  };

  const handleViewDraft = () => {
    onViewDraft?.();
    onClose();
  };

  const handleDeleteConfirm = () => {
    onDelete();
    onClose();
  };

  const isCompleted = status === "completed";
  const isFailed = status === "failed";
  const isCancelled = status === "cancelled";
  // 已完成/失败/取消 的卡片允许重试；运行中的不允许
  const canRetry = isCompleted || isFailed || isCancelled;
  // 运行中（pending/running）允许归档（取消）；已终态不允许
  const canArchive = status === "pending" || status === "running";
  // 非 pending 的任务已有/将有草稿，可查看
  const canViewDraft = status !== "pending" && !!onViewDraft;

  if (!pos) return null;

  return (
    <div
      ref={menuRef}
      className="card-menu"
      style={{
        position: "fixed",
        top: pos.top,
        left: pos.left,
        animation: "dropdownSlideIn 0.15s var(--ease-out-expo) forwards",
      }}
    >
      {!confirmDelete ? (
        <>
          {canViewDraft && (
            <button
              className="card-menu-item"
              onClick={handleViewDraft}
              title="查看此任务的草稿"
            >
              <IconDraft size={14} />
              <span>查看草稿</span>
            </button>
          )}
          <button
            className="card-menu-item"
            onClick={handleRetry}
            disabled={!canRetry}
            title={canRetry ? "基于此任务创建新 run" : "仅已结束的任务可重试"}
          >
            <IconRetry size={14} />
            <span>重试</span>
          </button>
          <button className="card-menu-item" onClick={handleCopyId}>
            <IconCopy size={14} />
            <span>复制 ID</span>
          </button>
          <button
            className="card-menu-item"
            onClick={handleArchive}
            disabled={!canArchive}
            title={canArchive ? "取消并归档此任务" : "仅运行中的任务可归档"}
          >
            <IconArchive size={14} />
            <span>归档</span>
          </button>
          <div className="card-menu-divider" />
          <button
            className="card-menu-item danger"
            onClick={() => setConfirmDelete(true)}
          >
            <IconTrash size={14} />
            <span>删除</span>
          </button>
        </>
      ) : (
        <div className="card-menu-confirm">
          <div className="card-menu-confirm-text">确认删除此任务？</div>
          <div className="card-menu-confirm-actions">
            <button
              className="card-menu-confirm-yes"
              onClick={handleDeleteConfirm}
            >
              删除
            </button>
            <button
              className="card-menu-confirm-no"
              onClick={() => setConfirmDelete(false)}
            >
              取消
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
