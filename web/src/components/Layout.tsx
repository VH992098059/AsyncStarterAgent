import { useState, useEffect, useRef, type ReactNode } from "react";
import {
  IconBoard,
  IconSettings,
  IconSearch,
  IconMenu,
  IconSun,
  IconMoon,
  IconUser,
  IconLogout,
  IconChevronDown,
  IconTrigger,
} from "./Icons";
import { useIsMobile } from "../hooks/useIsMobile";
import { useClickOutside } from "../hooks/useClickOutside";
import { showToast } from "../hooks/useToast";
import { ToastContainer } from "./ToastContainer";

// re-export 保持 App.tsx / KanbanBoard.tsx 等现有 `import { showToast } from "./Layout"` 兼容。
export { showToast };

export type PageKey = "board" | "trigger" | "context" | "draft" | "delivery" | "settings";

interface NavItem {
  key: PageKey;
  label: string;
  icon: ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  { key: "board", label: "看板", icon: <IconBoard /> },
  { key: "trigger", label: "规则", icon: <IconTrigger /> },
  { key: "settings", label: "设置", icon: <IconSettings /> },
];

interface LayoutProps {
  currentPage: PageKey;
  onNavigate: (page: PageKey) => void;
  chatOpen: boolean;
  onCloseChat?: () => void;
  onLogout?: () => void;
  username?: string;
  onSearch?: (text: string) => Promise<void> | void;
  children: ReactNode;
  chatPanel: ReactNode;
}

function UserMenu({ username, onLogout }: { username?: string; onLogout?: () => void }) {
  const [open, setOpen] = useState(false);
  const btnRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const [dropdownPos, setDropdownPos] = useState<{ top: number; left: number } | null>(null);

  useClickOutside(btnRef, () => setOpen(false), [dropdownRef]);

  useEffect(() => {
    if (open && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      setDropdownPos({
        top: Math.max(8, rect.top - 60),
        left: rect.right + 8,
      });
    } else {
      setDropdownPos(null);
    }
  }, [open]);

  const displayName = username || "用户";
  const initial = displayName.charAt(0).toUpperCase();

  return (
    <div className="user-menu-wrapper">
      <button
        ref={btnRef}
        className={`user-avatar-btn ${open ? "open" : ""}`}
        onClick={() => setOpen(!open)}
        aria-label="用户菜单"
      >
        <div className="user-avatar">
          <span>{initial}</span>
        </div>
        <span className="user-name">{displayName}</span>
      </button>
      {open && dropdownPos && (
        <div
          ref={dropdownRef}
          className="user-dropdown"
          style={{
            position: "fixed",
            top: dropdownPos.top,
            left: dropdownPos.left,
            animation: "dropdownSlideIn 0.15s var(--ease-out-expo) forwards",
          }}
        >
          <div className="user-dropdown-info">
            <div className="user-dropdown-name">{displayName}</div>
            <div className="user-dropdown-desc">已登录</div>
          </div>
          <div className="user-dropdown-divider" />
          <button
            className="user-dropdown-item"
            onClick={() => {
              setOpen(false);
              onLogout?.();
            }}
          >
            <IconLogout size={16} />
            <span>退出登录</span>
          </button>
        </div>
      )}
    </div>
  );
}

export function Layout({ currentPage, onNavigate, chatOpen, onCloseChat, onLogout, username, onSearch, children, chatPanel }: LayoutProps) {
  const isMobile = useIsMobile();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [searching, setSearching] = useState(false);
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem("asa-theme");
    return saved ? saved === "dark" : true;
  });

  const handleSearchKeyDown = async (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== "Enter" || !onSearch) return;
    const text = searchText.trim();
    if (!text || searching) return;
    e.preventDefault();
    setSearching(true);
    try {
      await onSearch(text);
      setSearchText("");
    } catch (err) {
      showToast("触发失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setSearching(false);
    }
  };

  useEffect(() => {
    document.documentElement.classList.toggle("dark", darkMode);
    localStorage.setItem("asa-theme", darkMode ? "dark" : "light");
  }, [darkMode]);

  useEffect(() => {
    setSidebarOpen(false);
  }, [currentPage]);

  const handleNavClick = (page: PageKey) => {
    onNavigate(page);
    if (page !== "board" && onCloseChat) onCloseChat();
    if (isMobile) setSidebarOpen(false);
  };

  const showMobileNav = isMobile && currentPage !== "board";

  return (
    <div className={`app-container ${chatOpen ? "chat-open" : ""}`}>
      <ToastContainer />
      <div className="top-bar">
        <div className="top-bar-left">
          <button
            className="icon-btn hamburger-btn"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="菜单"
          >
            <IconMenu />
          </button>

          <div className="top-bar-logo">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#10b981" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
            <span className="hidden sm:inline">Async Starter Agent</span>
          </div>
        </div>

        <div className="top-bar-search">
          <div className="top-bar-search-inner">
            <div className="top-bar-search-icon">
              <IconSearch />
            </div>
            <input
              type="text"
              placeholder="输入指令快速触发 Agent..."
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              onKeyDown={handleSearchKeyDown}
              disabled={searching || !onSearch}
            />
          </div>
        </div>

        <div className="top-bar-actions">
          <button className="icon-btn" onClick={() => setDarkMode(!darkMode)} aria-label="切换主题">
            {darkMode ? <IconSun /> : <IconMoon />}
          </button>
        </div>
      </div>

      {sidebarOpen && isMobile && (
        <div className="sidebar-overlay show" onClick={() => setSidebarOpen(false)} />
      )}

      <div className="main-content">
        <nav className={`sidebar ${sidebarOpen ? "show" : ""}`}>
          <div className="sidebar-nav">
            {NAV_ITEMS.map((item) => (
              <button
                key={item.key}
                className={`sidebar-item ${currentPage === item.key ? "active" : ""}`}
                onClick={() => handleNavClick(item.key)}
                title={item.label}
              >
                {item.icon}
                <span>{item.label}</span>
              </button>
            ))}
          </div>

          <div className="sidebar-bottom">
            <UserMenu username={username} onLogout={onLogout} />
          </div>
        </nav>

        {currentPage === "board" ? (
          <>
            <div className="kanban-area">{children}</div>
            {chatPanel}
          </>
        ) : (
          <div className="page-content">{children}</div>
        )}
      </div>

      {showMobileNav && (
        <div className="mobile-bottom-nav">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.key}
              className={`mobile-bottom-nav-item ${currentPage === item.key ? "active" : ""}`}
              onClick={() => handleNavClick(item.key)}
            >
              {item.icon}
              <span>{item.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export type { LayoutProps };
