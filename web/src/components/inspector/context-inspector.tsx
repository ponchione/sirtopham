import type { UseContextReportReturn } from "@/hooks/use-context-report";
import { CollapsibleSection } from "@/components/inspector/collapsible-section";
import { BudgetBar } from "@/components/inspector/budget-bar";
import type {
  ContextSignal,
  RAGResult,
  BrainResult,
  GraphResult,
} from "@/types/metrics";

interface ContextInspectorProps {
  ctx: UseContextReportReturn;
  onClose: () => void;
}

export function ContextInspector({ ctx, onClose }: ContextInspectorProps) {
  const { report, loading, currentTurn, totalTurns, nextTurn, prevTurn } = ctx;

  return (
    <div
      data-augmented-ui="tl-clip bl-clip border"
      className="flex w-80 flex-col bg-sidebar overflow-hidden"
      style={{
        "--aug-tl": "15px",
        "--aug-bl": "15px",
        "--aug-border-left": "2px",
        "--aug-border-bg":
          "linear-gradient(180deg, #00e5ff, #00e67640, #00e5ff)",
      } as React.CSSProperties}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-xs font-semibold uppercase tracking-widest text-primary text-glow-cyan">
          Context Inspector
        </span>
        <button
          type="button"
          onClick={onClose}
          className="p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label="Close inspector"
        >
          <XIcon />
        </button>
      </div>

      {/* Turn navigation */}
      <div className="flex items-center justify-between border-b border-border px-3 py-1.5">
        <button
          type="button"
          onClick={prevTurn}
          disabled={currentTurn <= 1}
          className="p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
        >
          <ChevronLeftIcon />
        </button>
        <span className="text-xs text-muted-foreground">
          {totalTurns > 0 ? `Turn ${currentTurn} of ${totalTurns}` : "No turns"}
        </span>
        <button
          type="button"
          onClick={nextTurn}
          disabled={currentTurn >= totalTurns}
          className="p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
        >
          <ChevronRightIcon />
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-3 py-2 space-y-1">
        {loading && (
          <p className="py-4 text-center text-xs text-muted-foreground">Loading…</p>
        )}

        {!loading && !report && (
          <p className="py-4 text-center text-xs text-muted-foreground">
            {totalTurns === 0 ? "No context data yet" : "No data for this turn"}
          </p>
        )}

        {report && (
          <>
            {/* Budget */}
            <CollapsibleSection title="Token Budget" sectionColor="#00e5ff" defaultOpen>
              <BudgetBar
                used={report.budget_used ?? 0}
                total={report.budget_total ?? 0}
                categories={report.budget_breakdown ?? []}
              />
            </CollapsibleSection>

            {/* Quality */}
            <CollapsibleSection title="Quality" sectionColor="#00e676">
              <QualityMetrics
                hitRate={report.context_hit_rate}
                usedSearch={report.agent_used_search_tool}
                agentFiles={report.agent_read_files}
                includedCount={report.included_count}
                excludedCount={report.excluded_count}
              />
            </CollapsibleSection>

            {/* Latency */}
            <CollapsibleSection title="Latency" sectionColor="#ffab00">
              <LatencyDisplay
                analysis={report.analysis_latency_ms}
                retrieval={report.retrieval_latency_ms}
                total={report.total_latency_ms}
              />
            </CollapsibleSection>

            {/* Signals */}
            <CollapsibleSection title="Signals" sectionColor="#b388ff">
              <SignalsList signals={report.signals ?? []} />
            </CollapsibleSection>

            {/* Queries */}
            <CollapsibleSection title="Queries" sectionColor="#00e5ff">
              <QueriesList queries={report.needs?.queries ?? []} />
            </CollapsibleSection>

            {/* RAG Results */}
            <CollapsibleSection title={`Code Chunks (${report.rag_results?.length ?? 0})`} sectionColor="#00e676">
              <RAGResultsList results={report.rag_results ?? []} />
            </CollapsibleSection>

            {/* Brain Results */}
            <CollapsibleSection title={`Brain (${report.brain_results?.length ?? 0})`} sectionColor="#ffab00">
              <BrainResultsList results={report.brain_results ?? []} />
            </CollapsibleSection>

            {/* Graph Results */}
            <CollapsibleSection title={`Graph (${report.graph_results?.length ?? 0})`} sectionColor="#b388ff">
              <GraphResultsList results={report.graph_results ?? []} />
            </CollapsibleSection>
          </>
        )}
      </div>
    </div>
  );
}

// ── Sub-sections ─────────────────────────────────────────────────────

function QualityMetrics({
  hitRate,
  usedSearch,
  agentFiles,
  includedCount,
  excludedCount,
}: {
  hitRate?: number;
  usedSearch?: number;
  agentFiles?: string[];
  includedCount?: number;
  excludedCount?: number;
}) {
  const hitColor =
    hitRate == null ? "text-muted-foreground"
      : hitRate > 0.7 ? "text-accent"
      : hitRate > 0.4 ? "text-[#ffab00]"
      : "text-destructive";

  return (
    <div className="space-y-1.5 text-xs">
      <div className="flex justify-between">
        <span className="text-muted-foreground">Hit rate</span>
        <span className={hitColor}>
          {hitRate != null ? `${(hitRate * 100).toFixed(0)}%` : "—"}
        </span>
      </div>
      <div className="flex justify-between">
        <span className="text-muted-foreground">Reactive search</span>
        <span className={usedSearch ? "text-[#ffab00]" : "text-accent"}>
          {usedSearch ? "Yes ⚠" : "No"}
        </span>
      </div>
      <div className="flex justify-between">
        <span className="text-muted-foreground">Included / excluded</span>
        <span>{includedCount ?? 0} / {excludedCount ?? 0}</span>
      </div>
      {agentFiles && agentFiles.length > 0 && (
        <div>
          <span className="text-muted-foreground">Agent read files:</span>
          <div className="mt-0.5 text-[10px] text-muted-foreground/80 max-h-20 overflow-y-auto">
            {agentFiles.map((f, i) => <div key={i}>{f}</div>)}
          </div>
        </div>
      )}
    </div>
  );
}

function LatencyDisplay({
  analysis,
  retrieval,
  total,
}: {
  analysis?: number;
  retrieval?: number;
  total?: number;
}) {
  const fmt = (ms?: number) => {
    if (ms == null) return "—";
    return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
  };
  const color = (ms?: number) => {
    if (ms == null) return "";
    if (ms < 200) return "text-accent";
    if (ms < 500) return "text-[#ffab00]";
    return "text-destructive";
  };

  return (
    <div className="space-y-1 text-xs">
      {[
        ["Analysis", analysis],
        ["Retrieval", retrieval],
        ["Total", total],
      ].map(([label, ms]) => (
        <div key={label as string} className="flex justify-between">
          <span className="text-muted-foreground">{label as string}</span>
          <span className={color(ms as number | undefined)}>{fmt(ms as number | undefined)}</span>
        </div>
      ))}
    </div>
  );
}

function SignalsList({ signals }: { signals: ContextSignal[] }) {
  if (signals.length === 0) {
    return <p className="text-xs text-muted-foreground">No signals detected</p>;
  }

  const typeColors: Record<string, string> = {
    file_ref: "bg-primary/20 text-primary text-glow-cyan",
    symbol_ref: "bg-[#b388ff]/20 text-[#b388ff]",
    intent_verb: "bg-accent/20 text-accent text-glow-green",
    momentum: "bg-[#ffab00]/20 text-[#ffab00]",
  };

  return (
    <div className="flex flex-wrap gap-1">
      {signals.map((s, i) => (
        <span
          key={i}
          className={`px-1.5 py-0.5 text-[10px] font-medium ${typeColors[s.type] ?? "bg-muted text-muted-foreground"}`}
          title={`${s.type}: ${s.value}${s.source ? ` (${s.source})` : ""}`}
        >
          {s.value}
        </span>
      ))}
    </div>
  );
}

function QueriesList({ queries }: { queries: string[] }) {
  if (queries.length === 0) {
    return <p className="text-xs text-muted-foreground">No queries generated</p>;
  }
  return (
    <div className="space-y-0.5">
      {queries.map((q, i) => (
        <div key={i} className="bg-muted/50 px-2 py-1 text-[10px]">
          {q}
        </div>
      ))}
    </div>
  );
}

function RAGResultsList({ results }: { results: RAGResult[] }) {
  if (results.length === 0) {
    return <p className="text-xs text-muted-foreground">No code chunks</p>;
  }
  return (
    <div className="space-y-0.5">
      {results.map((r, i) => (
        <div key={i} className="flex items-center gap-1.5 text-[10px]">
          <span className={`shrink-0 px-1 py-0.5 font-medium ${r.included ? "bg-accent/20 text-accent" : "bg-destructive/20 text-destructive"}`}>
            {r.score.toFixed(2)}
          </span>
          <span className="truncate text-muted-foreground" title={r.file_path}>
            {r.chunk_name ?? r.file_path}
          </span>
        </div>
      ))}
    </div>
  );
}

function BrainResultsList({ results }: { results: BrainResult[] }) {
  if (results.length === 0) {
    return <p className="text-xs text-muted-foreground">No brain results</p>;
  }
  return (
    <div className="space-y-0.5">
      {results.map((r, i) => (
        <div key={i} className="flex items-center gap-1.5 text-[10px]">
          <span className="shrink-0 bg-muted px-1 py-0.5 font-medium">
            {r.score.toFixed(2)}
          </span>
          <span className="truncate text-muted-foreground" title={r.vault_path}>
            {r.title ?? r.vault_path}
          </span>
          {r.match_mode && (
            <span className="shrink-0 text-muted-foreground/50">{r.match_mode}</span>
          )}
        </div>
      ))}
    </div>
  );
}

function GraphResultsList({ results }: { results: GraphResult[] }) {
  if (results.length === 0) {
    return <p className="text-xs text-muted-foreground">No graph results</p>;
  }
  return (
    <div className="space-y-0.5">
      {results.map((r, i) => (
        <div key={i} className="text-[10px]">
          <span className="font-medium">{r.symbol}</span>
          <span className="text-muted-foreground"> → {r.relationship} </span>
          <span className="text-muted-foreground/70">{r.file_path}</span>
        </div>
      ))}
    </div>
  );
}

// ── Icons ────────────────────────────────────────────────────────────

function XIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M18 6 6 18" /><path d="m6 6 12 12" />
    </svg>
  );
}

function ChevronLeftIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="m15 18-6-6 6-6" />
    </svg>
  );
}

function ChevronRightIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}
