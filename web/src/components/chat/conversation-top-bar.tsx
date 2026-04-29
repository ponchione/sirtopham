import type { AppConfig } from "@/types/metrics";

export function ConversationTopBar({
  connectionStatus,
  conversationId,
  config,
  metricsOpen,
  inspectorOpen,
  onToggleMetrics,
  onToggleInspector,
}: {
  connectionStatus: string;
  conversationId: string | null;
  config: AppConfig | null;
  metricsOpen: boolean;
  inspectorOpen: boolean;
  onToggleMetrics: () => void;
  onToggleInspector: () => void;
}) {
  return (
    <div className="flex items-center justify-between border-b border-border px-4 py-1.5 gap-3">
      <div className="flex items-center gap-3 min-w-0">
        <div className="text-xs text-muted-foreground shrink-0">
          {connectionStatus !== "connected"
            ? connectionStatus === "connecting" ? "Connecting…" : "Disconnected — reconnecting…"
            : conversationId ? `${conversationId.slice(0, 8)}…` : "New conversation"}
        </div>
        {config && (
          <div className="flex items-center gap-2 min-w-0">
            <span className="max-w-72 truncate text-xs text-muted-foreground">
              {config.default_provider}/{config.default_model}
            </span>
          </div>
        )}
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <button
          type="button"
          onClick={onToggleMetrics}
          className={`p-1 text-xs ${metricsOpen ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`}
          title="Conversation metrics"
        >
          📊
        </button>
        <button
          type="button"
          onClick={onToggleInspector}
          className={`p-1 text-xs ${inspectorOpen ? "bg-muted text-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`}
          title="Context inspector"
        >
          🔍
        </button>
      </div>
    </div>
  );
}
