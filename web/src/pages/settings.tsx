import { useEffect, useState } from "react";
import { useProviders } from "@/hooks/use-providers";
import { useProjectInfo } from "@/hooks/use-project-info";
import { api } from "@/lib/api";
import type { AppConfig } from "@/types/metrics";

function formatTimestamp(value?: string): string {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatBrainIndexStatus(status?: string): string {
  switch (status) {
    case "clean":
      return "Clean";
    case "stale":
      return "Stale";
    case "never_indexed":
      return "Never indexed";
    default:
      return status ?? "Unknown";
  }
}

export function SettingsPage() {
  const { providers, loading: provLoading } = useProviders();
  const { project, loading: projLoading } = useProjectInfo();
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [configLoading, setConfigLoading] = useState(true);

  // Load config on mount.
  useEffect(() => {
    api
      .get<AppConfig>("/api/config")
      .then((c) => {
        setConfig(c);
        setConfigLoading(false);
      })
      .catch(() => setConfigLoading(false));
  }, []);

  return (
    <div className="flex-1 overflow-y-auto px-4 py-6">
      <div className="mx-auto max-w-2xl space-y-6">
        <h1 className="text-xl font-bold uppercase tracking-widest text-primary text-glow-cyan">
          Settings
        </h1>

        {/* Project Info */}
        <section className="space-y-2">
          <h2 className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
            Project
          </h2>
          {projLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : project ? (
            <div
              data-augmented-ui="tl-clip br-clip border"
              className="border-0 bg-muted p-3 text-sm space-y-1"
              style={{
                "--aug-tl": "10px",
                "--aug-br": "10px",
                "--aug-border-all": "1px",
                "--aug-border-bg": "#1a2a3a",
              } as React.CSSProperties}
            >
              <div className="flex justify-between">
                <span className="text-muted-foreground">Name</span>
                <span className="font-medium">{project.name}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-muted-foreground">Path</span>
                <span className="text-xs">{project.root_path}</span>
              </div>
              {project.language && (
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Language</span>
                  <span>{project.language}</span>
                </div>
              )}
              <div className="flex justify-between gap-3">
                <span className="text-muted-foreground">Last indexed</span>
                <span className="text-right text-xs">{formatTimestamp(project.last_indexed_at)}</span>
              </div>
              <div className="flex justify-between gap-3">
                <span className="text-muted-foreground">Indexed commit</span>
                <span className="text-right text-xs font-mono">{project.last_indexed_commit ?? "Unknown"}</span>
              </div>
              {project.brain_index && (
                <>
                  <div className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Brain index</span>
                    <span className={`text-right text-xs ${project.brain_index.status === "stale" ? "text-destructive" : project.brain_index.status === "clean" ? "text-accent" : "text-muted-foreground"}`}>
                      {formatBrainIndexStatus(project.brain_index.status)}
                    </span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span className="text-muted-foreground">Brain indexed</span>
                    <span className="text-right text-xs">{formatTimestamp(project.brain_index.last_indexed_at)}</span>
                  </div>
                  {project.brain_index.status === "stale" && (
                    <>
                      <div className="flex justify-between gap-3">
                        <span className="text-muted-foreground">Brain stale since</span>
                        <span className="text-right text-xs">{formatTimestamp(project.brain_index.stale_since)}</span>
                      </div>
                      <div className="flex justify-between gap-3">
                        <span className="text-muted-foreground">Brain stale reason</span>
                        <span className="text-right text-xs font-mono">{project.brain_index.stale_reason ?? "Unknown"}</span>
                      </div>
                    </>
                  )}
                </>
              )}
            </div>
          ) : (
            <p className="text-xs text-muted-foreground">No project info available</p>
          )}
        </section>

        {/* Default Model */}
        <section className="space-y-2">
          <h2 className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
            Default Model
          </h2>
          {configLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : config ? (
            <div
              data-augmented-ui="tl-clip br-clip border"
              className="border-0 bg-muted p-3 space-y-3"
              style={{
                "--aug-tl": "10px",
                "--aug-br": "10px",
                "--aug-border-all": "1px",
                "--aug-border-bg": "#1a2a3a",
              } as React.CSSProperties}
            >
              <div className="space-y-1 text-sm">
                <div className="text-muted-foreground">Current default</div>
                <div className="font-medium text-primary">
                  {config.default_model} <span className="text-muted-foreground">({config.default_provider})</span>
                </div>
              </div>

              <div className="space-y-1 text-[10px] text-muted-foreground/60">
                <div>
                  Agent: max {config.agent.max_iterations} iterations, extended thinking{" "}
                  {config.agent.extended_thinking ? "on" : "off"}
                </div>
                <div>Tool output max tokens: {config.agent.tool_output_max_tokens}</div>
                <div>
                  Anthropic prompt cache markers: system{" "}
                  {config.agent.cache_system_prompt ? "on" : "off"}, context{" "}
                  {config.agent.cache_assembled_context ? "on" : "off"}, history{" "}
                  {config.agent.cache_conversation_history ? "on" : "off"}
                </div>
                {config.agent.tool_result_store_root && (
                  <div className="break-all">
                    Persisted tool result store: {config.agent.tool_result_store_root}
                  </div>
                )}
              </div>
            </div>
          ) : null}
        </section>

        {/* Providers */}
        <section className="space-y-2">
          <h2 className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
            Providers
          </h2>
          {provLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : providers.length === 0 ? (
            <p className="text-xs text-muted-foreground">No providers configured</p>
          ) : (
            <div className="space-y-2">
              {providers.map((prov) => (
                <div
                  key={prov.name}
                  data-augmented-ui="tl-clip br-clip border"
                  className="border-0 bg-muted p-3"
                  style={{
                    "--aug-tl": "10px",
                    "--aug-br": "10px",
                    "--aug-border-all": "1px",
                    "--aug-border-bg": "#1a2a3a",
                  } as React.CSSProperties}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{prov.name}</span>
                      <span className="bg-muted-foreground/10 px-1.5 py-0.5 text-[10px] text-muted-foreground">
                        {prov.type}
                      </span>
                    </div>
                    <span
                      className={`px-2 py-0.5 text-[10px] font-medium ${
                        prov.status === "available"
                          ? "bg-accent/20 text-accent glow-green"
                          : "bg-destructive/20 text-destructive glow-red"
                      }`}
                    >
                      {prov.status}
                    </span>
                  </div>
                  {prov.models.length > 0 && (
                    <div className="mt-2 space-y-0.5">
                      {prov.models.map((m) => (
                        <div key={m.id} className="flex items-center gap-2 text-xs text-muted-foreground">
                          <span>{m.id}</span>
                          <span className="text-[10px]">{(m.context_window / 1000).toFixed(0)}k ctx</span>
                          {m.supports_tools && <span className="text-[10px]">🔧</span>}
                          {m.supports_thinking && <span className="text-[10px]">💭</span>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
