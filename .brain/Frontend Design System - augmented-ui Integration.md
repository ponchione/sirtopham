
**Status:** ✅ Draft **Last Updated:** 2026-04-02 **Author:** Mitchell **Depends on:** [[07-web-interface-and-streaming]]

---

## Overview

sirtopham's frontend adopts [augmented-ui](https://augmented-ui.com/) as the visual design layer to give the application a cyberpunk/sci-fi aesthetic. augmented-ui is a pure CSS library that applies clipped corners, styled borders, and inlay effects to any HTML element via `data-augmented-ui` attributes and CSS custom properties.

This is an additive visual layer. The existing React component tree, Tailwind utility classes, hooks, WebSocket logic, API layer, and routing remain completely untouched. augmented-ui changes the _geometry and visual treatment_ of containers — it does not replace any component library.

**Design pillars:**

- Dark mode only — no light theme
- Hard edges, clipped corners — zero `border-radius`
- Monospace typography — terminal/HUD aesthetic
- Cyan/green primary palette with amber warnings
- The UI should feel like a ship's diagnostic console, not a SaaS dashboard

---

## Installation & Vite Integration

augmented-ui's CSS uses CSS Space Toggle tricks that many CSS minifiers misparse. It must be loaded _outside_ the Tailwind/Vite CSS pipeline.

**Approach:** Include augmented-ui via a `<link>` tag in `web/index.html`, loaded before the app's CSS:

```html
<!-- web/index.html -->
<link rel="stylesheet" href="/augmented-ui.min.css">
```

Install via npm and copy the CSS to the public directory:

```bash
npm install augmented-ui
cp node_modules/augmented-ui/augmented-ui.min.css public/
```

Add a `postinstall` script to `web/package.json` to keep this in sync:

```json
"scripts": {
  "postinstall": "cp node_modules/augmented-ui/augmented-ui.min.css public/"
}
```

**Important:** Do NOT `@import` augmented-ui inside `index.css` or any Tailwind-processed file. The Vite CSS pipeline will mangle it.

---

## Color Palette

Replace the existing shadcn oklch neutral palette in `web/src/index.css` with a sci-fi color scheme. Drop the light theme entirely — only `.dark` (or make it the root default).

### Core Colors

|Token|Value|Usage|
|---|---|---|
|`--background`|`#0a0e14`|Deep void — main background|
|`--foreground`|`#b0bec5`|Default text — cool gray|
|`--primary`|`#00e5ff`|Cyan — primary actions, active states|
|`--primary-foreground`|`#0a0e14`|Text on primary backgrounds|
|`--accent`|`#00e676`|Green — success, active indicators|
|`--accent-foreground`|`#0a0e14`|Text on accent backgrounds|
|`--warning`|`#ffab00`|Amber — warnings, in-progress states|
|`--destructive`|`#ff1744`|Red — errors, cancel, danger|
|`--muted`|`#131a24`|Slightly lighter void — cards, panels|
|`--muted-foreground`|`#546e7a`|Subdued text|
|`--border`|`#1a2a3a`|Subtle borders between sections|
|`--input`|`#0d1520`|Input field backgrounds|
|`--ring`|`#00e5ff40`|Focus ring glow (cyan with alpha)|
|`--sidebar`|`#080c12`|Sidebar background — darker than main|

### augmented-ui Frame Colors

These CSS custom properties style the augmented borders and inlays:

```css
/* Applied to augmented elements via their own classes */
--aug-border-bg: linear-gradient(135deg, #00e5ff, #00e5ff80);
--aug-inlay-bg: #0a0e1480;

/* Variant: accent (green) */
--aug-border-bg: linear-gradient(135deg, #00e676, #00e67680);

/* Variant: warning (amber) */
--aug-border-bg: linear-gradient(135deg, #ffab00, #ffab0080);

/* Variant: danger (red) */
--aug-border-bg: linear-gradient(135deg, #ff1744, #ff174480);
```

---

## Typography

Replace Geist Variable with a monospace font. Primary candidate: **JetBrains Mono** (open source, excellent readability, ligature support).

```bash
npm install @fontsource-variable/jetbrains-mono
```

Update `index.css`:

```css
@import "@fontsource-variable/jetbrains-mono";

@theme inline {
  --font-sans: 'JetBrains Mono Variable', 'Courier New', monospace;
  --font-heading: 'JetBrains Mono Variable', monospace;
}
```

All text becomes monospace. This is intentional — the entire UI should read like a terminal interface.

---

## Global CSS Changes

### Kill All Border Radius

```css
@theme inline {
  --radius: 0px;
  --radius-sm: 0px;
  --radius-md: 0px;
  --radius-lg: 0px;
  --radius-xl: 0px;
  --radius-2xl: 0px;
  --radius-3xl: 0px;
  --radius-4xl: 0px;
}
```

### Scanline Overlay

A subtle full-screen scanline effect applied to the root layout:

```css
.scanlines::after {
  content: '';
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 9999;
  background: repeating-linear-gradient(
    0deg,
    transparent,
    transparent 2px,
    rgba(0, 0, 0, 0.03) 2px,
    rgba(0, 0, 0, 0.03) 4px
  );
}
```

### Glow Utilities

Tailwind-compatible custom utilities for glow effects:

```css
@layer utilities {
  .glow-cyan {
    box-shadow: 0 0 8px #00e5ff40, 0 0 24px #00e5ff15;
  }
  .glow-green {
    box-shadow: 0 0 8px #00e67640, 0 0 24px #00e67615;
  }
  .glow-amber {
    box-shadow: 0 0 8px #ffab0040, 0 0 24px #ffab0015;
  }
  .glow-red {
    box-shadow: 0 0 8px #ff174440, 0 0 24px #ff174415;
  }
  .text-glow-cyan {
    text-shadow: 0 0 6px #00e5ff60;
  }
  .text-glow-green {
    text-shadow: 0 0 6px #00e67660;
  }
}
```

---

## Element Augmentation Map

This section maps every major UI element to its augmented-ui treatment. Each entry specifies the mixin string, key CSS properties, and any implementation notes.

### Root Layout (`root-layout.tsx`)

The outermost shell. Apply the scanline overlay class here.

```tsx
<div className="scanlines flex h-screen overflow-hidden bg-background text-foreground">
```

No augmented frame on the root — it's the viewport.

### Sidebar (`sidebar.tsx`)

The sidebar is the primary navigation frame. Augment with a clipped top-right corner and a styled border on the right edge.

```tsx
<aside
  data-augmented-ui="tr-clip border"
  className="..."
  style={{
    '--aug-tr': '20px',
    '--aug-border-right': '2px',
    '--aug-border-bg': 'linear-gradient(180deg, #00e5ff, #00e5ff40, transparent)',
  }}
>
```

**Header ("sirtopham" title):** Add `text-glow-cyan` class. Consider uppercase + letter-spacing for a HUD label feel.

**New conversation button:** Augment with small corner clips:

```tsx
<button
  data-augmented-ui="tl-clip br-clip border"
  style={{ '--aug-tl': '8px', '--aug-br': '8px', '--aug-border-all': '1px', '--aug-border-bg': '#00e5ff' }}
>
```

**Conversation list items:** No augmented frame (too noisy at this density). Instead, use a left-edge glow on the active item:

```css
/* Active conversation */
border-left: 2px solid #00e5ff;
box-shadow: inset 4px 0 8px -4px #00e5ff40;
```

### Message Bubbles (`conversation.tsx → MessageBubble`)

**User messages:** Augment with a bottom-right clip. Cyan border.

```tsx
<div
  data-augmented-ui="br-clip border"
  style={{
    '--aug-br': '12px',
    '--aug-border-all': '1px',
    '--aug-border-bg': '#00e5ff60',
  }}
>
```

**Assistant messages:** Augment with a top-left clip. Green border.

```tsx
<div
  data-augmented-ui="tl-clip border"
  style={{
    '--aug-tl': '12px',
    '--aug-border-all': '1px',
    '--aug-border-bg': '#00e67640',
  }}
>
```

**System / compressed messages:** No augmented frame. Keep the existing dashed border but in the warning amber color.

### Tool Call Card (`tool-call-card.tsx`)

Tool calls are a prime target for the HUD treatment. The collapsed header gets a subtle augmented frame; the expanded detail panel gets a more pronounced one.

**Expanded detail panel:**

```tsx
<div
  data-augmented-ui="tl-clip br-clip both"
  style={{
    '--aug-tl': '10px',
    '--aug-br': '10px',
    '--aug-border-all': '1px',
    '--aug-border-bg': statusColor, // green/red/amber based on tool status
    '--aug-inlay-all': '3px',
    '--aug-inlay-bg': '#0a0e1480',
  }}
>
```

**Status indicators:** Replace the current text emoji (✓, ✗, ⟳) with colored dots or small SVG indicators that match the palette.

### Thinking Block (`thinking-block.tsx`)

Subtle treatment — the thinking block is secondary UI. Augment the expanded content area only:

```tsx
<div
  data-augmented-ui="tl-2 border"
  style={{
    '--aug-tl': '6px',
    '--aug-border-left': '2px',
    '--aug-border-bg': '#546e7a',
  }}
>
```

### Context Inspector (`context-inspector.tsx`)

This is the **hero panel** for the design. The inspector shows RAG results, budget allocation, signal detection — it should look like a ship's diagnostic readout.

**Outer frame:**

```tsx
<div
  data-augmented-ui="tl-clip bl-clip border"
  className="flex w-80 flex-col bg-sidebar overflow-hidden"
  style={{
    '--aug-tl': '15px',
    '--aug-bl': '15px',
    '--aug-border-left': '2px',
    '--aug-border-bg': 'linear-gradient(180deg, #00e5ff, #00e67640, #00e5ff)',
  }}
>
```

**Collapsible sections:** Each `CollapsibleSection` gets a left-edge accent bar when open:

```css
border-left: 2px solid var(--section-color);
padding-left: 8px;
```

Where `--section-color` varies by section type:

- Token Budget: cyan (`#00e5ff`)
- Quality: green (`#00e676`)
- Latency: amber (`#ffab00`)
- Signals: purple (`#b388ff`)
- Queries: cyan
- Code Chunks: green
- Brain: amber
- Graph: purple

**Budget bar (`budget-bar.tsx`):** Style the progress bar with a gradient fill and glow:

```css
background: linear-gradient(90deg, #00e676, #ffab00, #ff1744);
box-shadow: 0 0 8px currentColor;
```

The bar color transitions from green (under budget) through amber to red (over budget), with an actual glow effect.

**Signal badges:** Replace `bg-blue-500/20` etc. with augmented-palette variants that include subtle text glow.

### Settings Page (`settings.tsx`)

**Section cards:** Each settings section (Project, Default Model, Providers) gets an augmented frame:

```tsx
<div
  data-augmented-ui="tl-clip br-clip border"
  style={{
    '--aug-tl': '10px',
    '--aug-br': '10px',
    '--aug-border-all': '1px',
    '--aug-border-bg': '#1a2a3a',
  }}
>
```

**Provider status badges:** Use glow utilities — `glow-green` for available, `glow-red` for unavailable.

**Model selection buttons:** Augmented with small clips. Active model gets a cyan glow.

### Landing Page (`conversation-list.tsx`)

The "sirtopham / AI coding assistant" splash. This is the first thing you see.

**Title treatment:** Large, uppercase, letter-spaced, with `text-glow-cyan`. Consider adding a subtle animated flicker or typing effect.

**Input area:** Augment the textarea wrapper with a prominent frame:

```tsx
<div
  data-augmented-ui="tl-clip tr-clip bl-clip br-clip both"
  style={{
    '--aug-tl': '15px',
    '--aug-tr': '15px',
    '--aug-bl': '15px',
    '--aug-br': '15px',
    '--aug-border-all': '2px',
    '--aug-border-bg': '#00e5ff',
    '--aug-inlay-all': '4px',
    '--aug-inlay-bg': '#0d1520',
  }}
>
```

### Input Area (Conversation Page)

The chat input in `conversation.tsx` should have a similar treatment to the landing page input but slightly smaller clips. The Send button gets a corner-clipped augmented frame. The Cancel button uses the destructive/red variant.

### Streaming Indicators

**Pulsing dot (agent thinking):** Replace `animate-pulse rounded-full bg-primary` with a cyan dot that has an actual glow animation:

```css
@keyframes pulse-glow {
  0%, 100% { box-shadow: 0 0 4px #00e5ff, 0 0 8px #00e5ff40; }
  50% { box-shadow: 0 0 8px #00e5ff, 0 0 16px #00e5ff60; }
}
.pulse-glow {
  animation: pulse-glow 1.5s ease-in-out infinite;
}
```

**Streaming cursor:** The text cursor (`animate-pulse bg-current`) becomes a cyan block cursor with glow.

---

## Nesting Rules

augmented-ui requires a `data-augmented-ui-reset` wrapper between nested augmented elements. This affects:

1. **Sidebar → conversation items:** The sidebar is augmented, but individual items are NOT augmented (styled with plain CSS borders instead), so no nesting issue.
    
2. **Inspector → collapsible sections:** The inspector outer frame is augmented. Inner sections use plain CSS accent bars, not augmented frames. No nesting issue.
    
3. **Message bubbles → tool call cards:** Both are augmented. The message bubble contains tool call cards. **This requires a reset layer:**
    

```tsx
{/* Inside MessageBubble for assistant messages */}
<div data-augmented-ui="tl-clip border" style={{...}}>
  {message.blocks.map((block, i) => (
    <div key={i} data-augmented-ui-reset>
      <BlockRenderer block={block} streaming={...} />
    </div>
  ))}
</div>
```

This is the only nesting case in the current UI. If future components introduce nested augmented frames, always wrap the inner element's parent in `data-augmented-ui-reset`.

---

## Component Helper: `useAugmented`

To avoid inline style objects everywhere, create a small utility:

```typescript
// web/src/lib/augmented.ts

export type AugVariant = 'cyan' | 'green' | 'amber' | 'red' | 'muted';

const borderColors: Record<AugVariant, string> = {
  cyan: '#00e5ff',
  green: '#00e676',
  amber: '#ffab00',
  red: '#ff1744',
  muted: '#1a2a3a',
};

export function augStyle(
  variant: AugVariant,
  options?: {
    borderWidth?: string;
    gradient?: boolean;
    inlay?: boolean;
    inlayBg?: string;
  }
): React.CSSProperties {
  const color = borderColors[variant];
  const width = options?.borderWidth ?? '1px';
  const bg = options?.gradient
    ? `linear-gradient(135deg, ${color}, ${color}80)`
    : color;

  const style: Record<string, string> = {
    '--aug-border-all': width,
    '--aug-border-bg': bg,
  };

  if (options?.inlay) {
    style['--aug-inlay-all'] = '3px';
    style['--aug-inlay-bg'] = options?.inlayBg ?? '#0a0e1480';
  }

  return style as unknown as React.CSSProperties;
}
```

Usage:

```tsx
<div
  data-augmented-ui="tl-clip br-clip border"
  style={{ '--aug-tl': '10px', '--aug-br': '10px', ...augStyle('cyan') }}
>
```

---

## Implementation Order

This work is a single epic within the frontend layer. Suggested implementation sequence for an agent session:

1. **Install augmented-ui** — npm install, copy CSS to public, add link tag to index.html
2. **Retheme index.css** — Replace color palette, swap font, kill border-radius, add glow utilities and scanline overlay
3. **Root layout + sidebar** — Apply augmented frame to sidebar, restyle header, conversation items, new conversation button
4. **Landing page** — Title treatment, augmented input frame
5. **Conversation page: messages** — User and assistant bubble augmentations, reset layers for nested tool cards
6. **Conversation page: tool calls + thinking** — Augmented tool detail panels, streaming indicators
7. **Context inspector** — Hero panel treatment, section accent bars, budget bar glow
8. **Settings page** — Section card frames, provider status glow, model button augmentations
9. **Polish pass** — Verify nesting reset layers, check clip zones don't eat interactive areas, test at various viewport widths

---

## Testing Considerations

- **Mobile viewport:** augmented-ui clip-paths can eat into content at small sizes. Test all augmented elements at 360px width minimum. Reduce `--aug-*` sizes on small breakpoints if needed.
- **Input hit areas:** Clipped corners on input containers must not overlap the actual `<textarea>` or `<button>` click targets. The clip is visual only (via `clip-path`), so pointer events should pass through, but verify.
- **Performance:** augmented-ui uses `clip-path: polygon(...)` which is GPU-composited. The number of augmented elements on screen at once (conversation with many tool calls) should be tested for paint performance.
- **embed.FS:** The `augmented-ui.min.css` in `web/public/` must be included in the Go embed. Verify `webfs/embed.go` picks up the public directory.

---

## What This Does NOT Change

- No changes to any file in `web/src/hooks/`
- No changes to any file in `web/src/lib/` (except adding `augmented.ts`)
- No changes to any file in `web/src/types/`
- No changes to WebSocket protocol or REST API
- No changes to Go backend
- No changes to routing (`main.tsx`)
- Component _logic_ (state, effects, event handlers) is untouched — only JSX attributes and className strings change

---

## Dependencies

- [[07-web-interface-and-streaming]] — defines the component architecture being restyled
- [augmented-ui v2](https://augmented-ui.com/docs/) — CSS framework documentation
- [JetBrains Mono](https://www.jetbrains.com/lp/mono/) — monospace font

---

## Open Questions

- **Sound effects?** augmented-ui's sister project Arwes includes "bleeps" — subtle UI sound effects on interactions. Could be interesting for tool call completion, turn completion, etc. Deferred to a future session.
- **CRT vignette?** A radial gradient darkening at screen edges. Easy to add but might be too much. Test during polish pass.
- **Animated borders?** augmented-ui borders support CSS `background` including animations. Pulsing borders on the streaming message could look great but might be distracting. Test during polish pass.