import { useState } from "react";
import type { ToolCallBlock } from "@/hooks/use-conversation";
import { isBrainToolName } from "@/lib/tool-transcript";

interface ToolCallCardProps {
  block: ToolCallBlock;
}

function formatDuration(ns?: number): string {
  if (ns == null) return "";
  const ms = ns / 1_000_000;
  if (ms < 1000) return `${ms.toFixed(0)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function ToolCallCard({ block }: ToolCallCardProps) {
  const isBrainTool = isBrainToolName(block.toolName);
  const [open, setOpen] = useState(isBrainTool);

  const statusColor = block.done
    ? block.success !== false
      ? "text-accent"
      : "text-destructive"
    : "text-[#ffab00]";

  const borderColor = block.done
    ? block.success !== false
      ? "#00e676"
      : "#ff1744"
    : "#ffab00";

  const statusIcon = block.done
    ? block.success !== false
      ? "✓"
      : "✗"
    : "⟳";

  return (
    <div className="my-1.5">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1.5 text-xs hover:text-foreground transition-colors"
      >
        <svg
          className={`h-3 w-3 text-muted-foreground transition-transform ${open ? "rotate-90" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
        </svg>
        <span className={statusColor}>{statusIcon}</span>
        <span className="font-medium text-foreground">{block.toolName}</span>
        {isBrainTool && (
          <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider text-primary">
            brain
          </span>
        )}
        {block.duration != null && (
          <span className="text-muted-foreground/60">{formatDuration(block.duration)}</span>
        )}
        {!block.done && (
          <span className="ml-1 inline-block h-2 w-2 bg-[#ffab00] pulse-glow" />
        )}
      </button>
      {open && (
        <div
          data-augmented-ui="tl-clip br-clip both"
          className="mt-1.5 ml-4 space-y-2 border-0 bg-muted/30 px-3 py-2 text-xs"
          style={{
            "--aug-tl": "10px",
            "--aug-br": "10px",
            "--aug-border-all": "1px",
            "--aug-border-bg": borderColor,
            "--aug-inlay-all": "3px",
            "--aug-inlay-bg": "#0a0e1480",
          } as React.CSSProperties}
        >
          {/* Arguments */}
          {block.args && Object.keys(block.args).length > 0 && (
            <div>
              <div className="mb-0.5 font-semibold text-muted-foreground uppercase tracking-wider text-[10px]">Arguments</div>
              <pre className="whitespace-pre-wrap text-foreground/80 max-h-40 overflow-y-auto">
                {JSON.stringify(block.args, null, 2)}
              </pre>
            </div>
          )}

          {/* Streaming output */}
          {block.output && block.output !== block.result && (
            <div>
              <div className="mb-0.5 font-semibold text-muted-foreground uppercase tracking-wider text-[10px]">Output</div>
              <pre className="whitespace-pre-wrap text-foreground/80 max-h-48 overflow-y-auto">
                {block.output}
                {!block.done && (
                  <span className="ml-0.5 inline-block h-3 w-1.5 bg-primary pulse-glow" />
                )}
              </pre>
            </div>
          )}

          {/* Final result (if different from streaming output) */}
          {block.done && block.result && (
            <div>
              <div className="mb-0.5 font-semibold text-muted-foreground uppercase tracking-wider text-[10px]">
                {isBrainTool ? "Brain result" : "Result"}
              </div>
              <pre className="whitespace-pre-wrap text-foreground/80 max-h-48 overflow-y-auto">
                {block.result}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
