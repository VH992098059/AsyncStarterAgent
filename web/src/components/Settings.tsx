import { useEffect, useState, useCallback } from "react";
import {
  getSettings,
  updateSettings,
  testLLMConnection,
  MODEL_PRESETS,
  isMaskedKey,
  type UserSettings,
} from "../api/client";
import {
  getFeishuStatus,
  startFeishuAuth,
  revokeFeishuAuth,
  getFeishuAppConfig,
  updateFeishuAppConfig,
  type FeishuAuthStatus,
  type FeishuAppConfig,
} from "../api/feishu";
import { showToast } from "./Layout";
import { useRipple } from "../hooks/useRipple";

const DEFAULT_SETTINGS: UserSettings = {
  llm_api_key: "",
  llm_base_url: "",
  llm_model: "",
  llm_temperature: 0.3,
  llm_max_tokens: 0,
  embed_api_key: "",
  embed_base_url: "",
  embed_model: "",
  notion_api_key: "",
  notion_parent_page: "",
  obsidian_vault_path: "",
};

function detectPreset(settings: UserSettings): string {
  for (const p of MODEL_PRESETS) {
    if (p.id === "custom") continue;
    if (settings.llm_base_url === p.base_url && settings.llm_model === p.llm_model) {
      return p.id;
    }
  }
  return "custom";
}

interface FieldProps {
  label: string;
  hint?: string;
  children: React.ReactNode;
}

function Field({ label, hint, children }: FieldProps) {
  return (
    <div>
      <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1.5">{label}</label>
      {children}
      {hint && <p className="text-[11px] text-[var(--text-muted)] mt-1">{hint}</p>}
    </div>
  );
}

function TextInput(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={`w-full px-3 py-2.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-subtle)] text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-emerald-500/30 transition-colors ${props.className ?? ""}`}
    />
  );
}

function SectionCard({ title, desc, children }: { title: string; desc?: string; children: React.ReactNode }) {
  return (
    <div className="app-card p-6 mb-4">
      <div className="mb-5">
        <h3 className="text-base font-semibold tracking-tight text-[var(--text-primary)]">{title}</h3>
        {desc && <p className="text-xs text-[var(--text-secondary)] mt-1">{desc}</p>}
      </div>
      <div className="space-y-4">{children}</div>
    </div>
  );
}

export function Settings() {
  const [form, setForm] = useState<UserSettings>(DEFAULT_SETTINGS);
  const [originalMasked, setOriginalMasked] = useState<Record<string, boolean>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [preset, setPreset] = useState<string>("custom");
  const [feishuStatus, setFeishuStatus] = useState<FeishuAuthStatus | null>(null);
  const [feishuLoading, setFeishuLoading] = useState(false);
  const [feishuAppConfig, setFeishuAppConfig] = useState<FeishuAppConfig>({ app_id: "", app_secret_masked: "", configured: false });
  const [feishuAppIdInput, setFeishuAppIdInput] = useState("");
  const [feishuAppSecretInput, setFeishuAppSecretInput] = useState("");
  const [savingFeishuConfig, setSavingFeishuConfig] = useState(false);
  const ripple = useRipple();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const s = await getSettings();
      setForm(s);
      setPreset(detectPreset(s));
      const masked: Record<string, boolean> = {};
      for (const k of ["llm_api_key", "embed_api_key", "notion_api_key"] as const) {
        masked[k] = isMaskedKey(s[k]);
      }
      setOriginalMasked(masked);
    } catch (err) {
      showToast("加载失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // 飞书授权状态加载
  // 用户从飞书授权页返回应用时（页面重新可见），自动刷新授权状态。
  // Callback 不再重定向到前端（后端返回 HTML 成功页），改用 visibilitychange 感知返回。
  useEffect(() => {
    getFeishuStatus()
      .then(setFeishuStatus)
      .catch(() => {
        // 静默：未授权或接口不可用时 status 保持 null，UI 显示"未授权"
      });
    getFeishuAppConfig()
      .then((cfg) => {
        setFeishuAppConfig(cfg);
        setFeishuAppIdInput(cfg.app_id);
        setFeishuAppSecretInput(cfg.app_secret_masked);
      })
      .catch(() => {});
    const handleVisibility = () => {
      if (document.visibilityState === "visible") {
        getFeishuStatus()
          .then(setFeishuStatus)
          .catch(() => {});
      }
    };
    document.addEventListener("visibilitychange", handleVisibility);
    return () => document.removeEventListener("visibilitychange", handleVisibility);
  }, []);

  const handlePresetChange = (presetId: string) => {
    setPreset(presetId);
    if (presetId === "custom") return;
    const p = MODEL_PRESETS.find((x) => x.id === presetId);
    if (!p) return;
    setForm((f) => ({
      ...f,
      llm_base_url: p.base_url,
      llm_model: p.llm_model,
      embed_model: p.embed_model || f.embed_model,
    }));
  };

  const handleChange = (field: keyof UserSettings, value: string | number) => {
    setForm((f) => ({ ...f, [field]: value }));
    if (preset !== "custom" && (field === "llm_base_url" || field === "llm_model")) {
      setPreset("custom");
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const body: Partial<UserSettings> = {};
      (Object.keys(form) as Array<keyof UserSettings>).forEach((k) => {
        if (k.endsWith("_api_key") && originalMasked[k]) {
          const val = String(form[k] ?? "");
          if (!val || isMaskedKey(val)) return;
        }
        body[k] = form[k] as never;
      });
      const updated = await updateSettings(body);
      setForm(updated);
      setPreset(detectPreset(updated));
      const masked: Record<string, boolean> = {};
      for (const k of ["llm_api_key", "embed_api_key", "notion_api_key"] as const) {
        masked[k] = isMaskedKey(updated[k]);
      }
      setOriginalMasked(masked);
      showToast("保存成功", "配置已更新，下次生成即生效", "success");
    } catch (err) {
      showToast("保存失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      await testLLMConnection();
      showToast("连接成功", "LLM 配置可用", "success");
    } catch (err) {
      showToast("连接失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setTesting(false);
    }
  };

  // 拉起飞书 OAuth 授权：调后端拿 authorize_url，浏览器跳转过去
  const handleStartFeishuAuth = async (e: React.MouseEvent<HTMLElement>) => {
    ripple(e);
    if (feishuLoading) return;
    setFeishuLoading(true);
    try {
      const res = await startFeishuAuth();
      window.location.href = res.authorize_url;
      // 不重置 loading：页面即将跳转，重置会导致按钮短暂可点击
    } catch (err) {
      showToast("授权失败", err instanceof Error ? err.message : String(err), "error");
      setFeishuLoading(false);
    }
  };

  // 撤销飞书授权：删除后端 token 记录
  const handleRevokeFeishu = async (e: React.MouseEvent<HTMLElement>) => {
    ripple(e);
    if (feishuLoading) return;
    setFeishuLoading(true);
    try {
      await revokeFeishuAuth();
      setFeishuStatus({ status: "not_authorized", name: "" });
      showToast("已解除授权", "飞书授权已撤销", "success");
    } catch (err) {
      showToast("解除失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setFeishuLoading(false);
    }
  };

  const handleSaveFeishuAppConfig = async (e: React.MouseEvent<HTMLElement>) => {
    ripple(e);
    if (savingFeishuConfig) return;
    if (!feishuAppIdInput) {
      showToast("保存失败", "App ID 不能为空", "error");
      return;
    }
    setSavingFeishuConfig(true);
    try {
      const updated = await updateFeishuAppConfig(feishuAppIdInput, feishuAppSecretInput);
      setFeishuAppConfig(updated);
      setFeishuAppSecretInput(updated.app_secret_masked);
      // 后端保存新凭证时会级联清除旧 OAuth token，前端同步刷新授权状态
      setFeishuStatus({ status: "not_authorized", name: "" });
      showToast("保存成功", "飞书应用凭证已更新，请重新授权", "success");
    } catch (err) {
      showToast("保存失败", err instanceof Error ? err.message : String(err), "error");
    } finally {
      setSavingFeishuConfig(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <svg className="animate-spin h-5 w-5 text-emerald-500" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      </div>
    );
  }

  const cloudPresets = MODEL_PRESETS.filter((p) => p.cloud);
  const localPresets = MODEL_PRESETS.filter((p) => !p.cloud);

  return (
    <div className="pb-28 md:pb-24">
      <div className="mb-6 flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl md:text-3xl font-semibold tracking-tight text-[var(--text-primary)]">设置</h1>
          <p className="text-[var(--text-secondary)] text-sm mt-1.5">AI 模型、Embedding 及第三方服务配置</p>
        </div>
        <div className="flex gap-2 shrink-0">
          <button
            onClick={(e) => { ripple(e); handleTest(); }}
            disabled={testing || saving}
            className="px-4 py-2.5 rounded-xl border border-[var(--border-default)] text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] btn-press ripple-container disabled:opacity-50 flex items-center gap-2"
          >
            {testing ? (
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            ) : (
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M7 2v3M7 9v3M2 7h3M9 7h3"/></svg>
            )}
            测试连接
          </button>
          <button
            onClick={(e) => { ripple(e); handleSave(); }}
            disabled={saving || testing}
            className="px-5 py-2.5 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 flex items-center gap-2 hover:bg-emerald-400 transition-colors"
          >
            {saving ? (
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            ) : null}
            保存配置
          </button>
        </div>
      </div>

      <SectionCard title="模型预设" desc="快速选择常用云服务或本地部署，自动填充 URL 和模型名">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          {[...cloudPresets, ...localPresets].map((p) => (
            <button
              key={p.id}
              onClick={(e) => { ripple(e); handlePresetChange(p.id); }}
              className={`px-3 py-2.5 rounded-xl border text-sm text-left transition-all btn-press ripple-container ${
                preset === p.id
                  ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300"
                  : "border-[var(--border-subtle)] bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:border-[var(--border-default)] hover:text-[var(--text-primary)]"
              }`}
            >
              <div className="flex items-center gap-2">
                {!p.cloud && <span className="text-[9px] px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-muted)] shrink-0">本地</span>}
                <span className="font-medium truncate">{p.name}</span>
              </div>
            </button>
          ))}
        </div>
      </SectionCard>

      <SectionCard title="LLM 大语言模型" desc="驱动草稿生成的对话模型。支持所有兼容 OpenAI 接口格式的服务">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="API Key" hint={originalMasked.llm_api_key && form.llm_api_key ? "已掩码，留空保持原值不变" : undefined}>
            <TextInput
              type="password"
              value={form.llm_api_key}
              onChange={(e) => handleChange("llm_api_key", e.target.value)}
              placeholder="sk-..."
              autoComplete="off"
            />
          </Field>
          <Field label="Base URL">
            <TextInput
              type="text"
              value={form.llm_base_url}
              onChange={(e) => handleChange("llm_base_url", e.target.value)}
              placeholder="https://api.openai.com/v1"
            />
          </Field>
          <Field label="模型名称">
            <TextInput
              type="text"
              value={form.llm_model}
              onChange={(e) => handleChange("llm_model", e.target.value)}
              placeholder="gpt-4o-mini"
            />
          </Field>
          <Field label="最大 Token 数" hint="0 表示使用模型默认值">
            <TextInput
              type="number"
              min={0}
              value={form.llm_max_tokens || ""}
              onChange={(e) => handleChange("llm_max_tokens", parseInt(e.target.value) || 0)}
              placeholder="0"
            />
          </Field>
        </div>
        <Field label={`温度 (Temperature): ${form.llm_temperature.toFixed(2)}`} hint="越低越确定，越高越有创造性，推荐 0.3~0.7">
          <input
            type="range"
            min={0}
            max={2}
            step={0.05}
            value={form.llm_temperature}
            onChange={(e) => handleChange("llm_temperature", parseFloat(e.target.value))}
            className="w-full accent-emerald-500"
          />
          <div className="flex justify-between text-[10px] text-[var(--text-muted)] mt-1">
            <span>精确 (0.0)</span>
            <span>平衡 (0.7)</span>
            <span>创意 (2.0)</span>
          </div>
        </Field>
      </SectionCard>

      <SectionCard title="Embedding 向量模型" desc="用于上下文检索的 Embedding 服务。留空则复用 LLM 的 API Key 和 URL">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="API Key" hint="留空则使用 LLM 的 API Key">
            <TextInput
              type="password"
              value={form.embed_api_key}
              onChange={(e) => handleChange("embed_api_key", e.target.value)}
              placeholder="留空复用 LLM Key"
              autoComplete="off"
            />
          </Field>
          <Field label="Base URL" hint="留空则使用 LLM 的 Base URL">
            <TextInput
              type="text"
              value={form.embed_base_url}
              onChange={(e) => handleChange("embed_base_url", e.target.value)}
              placeholder="留空复用 LLM URL"
            />
          </Field>
          <Field label="模型名称">
            <TextInput
              type="text"
              value={form.embed_model}
              onChange={(e) => handleChange("embed_model", e.target.value)}
              placeholder="text-embedding-3-small"
            />
          </Field>
        </div>
      </SectionCard>

      <SectionCard title="Notion" desc="将草稿直接发布到 Notion 页面">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="API Key (Integration Token)">
            <TextInput
              type="password"
              value={form.notion_api_key}
              onChange={(e) => handleChange("notion_api_key", e.target.value)}
              placeholder="ntn_xxx 或 secret_xxx"
              autoComplete="off"
            />
          </Field>
          <Field label="Parent Page ID" hint="目标 Notion 页面的 32 位 ID">
            <TextInput
              type="text"
              value={form.notion_parent_page}
              onChange={(e) => handleChange("notion_parent_page", e.target.value)}
              placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
            />
          </Field>
        </div>
      </SectionCard>

      <SectionCard title="Obsidian" desc="将草稿保存为 Markdown 文件到本地 Obsidian Vault">
        <Field label="Vault 路径" hint="本地 Vault 目录的绝对路径">
          <TextInput
            type="text"
            value={form.obsidian_vault_path}
            onChange={(e) => handleChange("obsidian_vault_path", e.target.value)}
            placeholder="C:\\Users\\YourName\\Obsidian\\MyVault"
          />
        </Field>
      </SectionCard>

      <SectionCard title="飞书集成" desc="授权后可拉取飞书任务、接收任务事件、交付到飞书文档">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-5">
          <Field label="App ID">
            <TextInput
              type="text"
              value={feishuAppIdInput}
              onChange={(e) => setFeishuAppIdInput(e.target.value)}
              placeholder="cli_xxxxxxxxxxxx"
            />
          </Field>
          <Field label="App Secret" hint={isMaskedKey(feishuAppSecretInput) ? "已掩码，留空或不改保持原值" : undefined}>
            <TextInput
              type="password"
              value={feishuAppSecretInput}
              onChange={(e) => setFeishuAppSecretInput(e.target.value)}
              placeholder="飞书开放平台 → 凭证与基础信息"
              autoComplete="off"
            />
          </Field>
        </div>
        <div className="flex justify-end mb-5">
          <button
            onClick={handleSaveFeishuAppConfig}
            disabled={savingFeishuConfig}
            className="px-4 py-2.5 rounded-xl border border-[var(--border-default)] text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] btn-press ripple-container disabled:opacity-50 flex items-center gap-2"
          >
            {savingFeishuConfig ? (
              <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            ) : null}
            保存凭证
          </button>
        </div>
        {feishuStatus?.status === "authorized" ? (
          <div className="flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-3 min-w-0">
              <span className="inline-flex items-center justify-center w-9 h-9 rounded-xl bg-emerald-500/10 text-emerald-500 shrink-0">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M20 6L9 17l-5-5" />
                </svg>
              </span>
              <div className="min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)] truncate">
                  {feishuStatus.name || "已授权"}
                </p>
                <p className="text-xs text-[var(--text-muted)]">飞书账号已绑定</p>
              </div>
            </div>
            <button
              onClick={handleRevokeFeishu}
              disabled={feishuLoading}
              className="px-4 py-2.5 rounded-xl border border-[var(--border-default)] text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] btn-press ripple-container disabled:opacity-50 flex items-center gap-2"
            >
              {feishuLoading ? (
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>
              ) : null}
              解除授权
            </button>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-3 min-w-0">
              <span className="inline-flex items-center justify-center w-9 h-9 rounded-xl bg-[var(--bg-tertiary)] text-[var(--text-muted)] shrink-0">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" y1="8" x2="12" y2="12" />
                  <line x1="12" y1="16" x2="12.01" y2="16" />
                </svg>
              </span>
              <div className="min-w-0">
                <p className="text-sm font-medium text-[var(--text-primary)]">未授权</p>
                <p className="text-xs text-[var(--text-muted)]">点击右侧按钮绑定飞书账号</p>
              </div>
            </div>
            <button
              onClick={handleStartFeishuAuth}
              disabled={feishuLoading || !feishuAppConfig.configured}
              title={!feishuAppConfig.configured ? "请先保存飞书应用凭证" : undefined}
              className="px-5 py-2.5 rounded-xl bg-emerald-500 text-white text-sm font-medium btn-press ripple-container disabled:opacity-50 flex items-center gap-2 hover:bg-emerald-400 transition-colors"
            >
              {feishuLoading ? (
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>
              ) : null}
              授权飞书账号
            </button>
          </div>
        )}
      </SectionCard>

      <div className="text-center py-4">
        <p className="text-xs text-[var(--text-muted)]">所有配置仅保存到服务端数据库</p>
      </div>
    </div>
  );
}
