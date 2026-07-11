import type { PageKey } from "./Layout";

interface BreadcrumbProps {
  /** 当前页标题（如"上下文搜集"/"草稿编辑"/"交付"） */
  title: string;
  /** 当前 runId（可选，显示前 8 位） */
  runId?: string | null;
  /** 返回看板回调 */
  onBackToBoard: () => void;
}

/**
 * Breadcrumb 工作流页顶部面包屑：看板 / 当前页 / runId 前 8 位。
 * 让用户在 context/draft/delivery 页能快速回看板。
 */
export function Breadcrumb({ title, runId, onBackToBoard }: BreadcrumbProps) {
  return (
    <nav className="breadcrumb" aria-label="面包屑">
      <button className="breadcrumb-link" onClick={onBackToBoard}>
        看板
      </button>
      <span className="breadcrumb-sep">/</span>
      <span className="breadcrumb-current">{title}</span>
      {runId && (
        <span className="breadcrumb-run-id" title={runId}>
          {runId.slice(0, 8)}
        </span>
      )}
    </nav>
  );
}

/** 导出类型供其他组件复用 onNavigate 签名 */
export type { PageKey };
