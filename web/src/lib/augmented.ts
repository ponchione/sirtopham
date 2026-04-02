import type { CSSProperties } from "react";

export type AugVariant = "cyan" | "green" | "amber" | "red" | "muted" | "purple";

const borderColors: Record<AugVariant, string> = {
  cyan: "#00e5ff",
  green: "#00e676",
  amber: "#ffab00",
  red: "#ff1744",
  muted: "#1a2a3a",
  purple: "#b388ff",
};

export function augStyle(
  variant: AugVariant,
  options?: {
    borderWidth?: string;
    gradient?: boolean;
    inlay?: boolean;
    inlayBg?: string;
  },
): CSSProperties {
  const color = borderColors[variant];
  const width = options?.borderWidth ?? "1px";
  const bg = options?.gradient
    ? `linear-gradient(135deg, ${color}, ${color}80)`
    : color;

  const style: Record<string, string> = {
    "--aug-border-all": width,
    "--aug-border-bg": bg,
  };

  if (options?.inlay) {
    style["--aug-inlay-all"] = "3px";
    style["--aug-inlay-bg"] = options?.inlayBg ?? "#0a0e1480";
  }

  return style as unknown as CSSProperties;
}

export function augCorners(
  sizes: Partial<Record<"tl" | "tr" | "bl" | "br", string>>,
): CSSProperties {
  const style: Record<string, string> = {};
  for (const [corner, size] of Object.entries(sizes)) {
    style[`--aug-${corner}`] = size;
  }
  return style as unknown as CSSProperties;
}
