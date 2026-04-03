import type { ContentBlock, ToolCallBlock, TextBlock } from '../hooks/use-conversation.ts';

const LOW_VALUE_PATTERNS = [
  /^done[.!]?$/i,
  /^let me know if you'd like me to .*$/i,
  /^let me know if you would like me to .*$/i,
  /^i already have the content\b/i,
  /^i've already searched\b/i,
  /^i already searched\b/i,
  /^i found the phrase\b/i,
  /^notes?\/.* contains:?$/i,
  /^i tried to replace\b/i,
  /^let me check that[.!]?$/i,
  /^checking[.!]?$/i,
];

export function isBrainToolName(toolName: string): boolean {
  return /^brain_/.test(toolName);
}

export function isLowValueToolFollowupText(text: string): boolean {
  const normalized = text.trim().replace(/\s+/g, ' ');
  if (!normalized) {
    return true;
  }
  return LOW_VALUE_PATTERNS.some((pattern) => pattern.test(normalized));
}

function isCompletedBrainTool(block: ContentBlock): block is ToolCallBlock {
  return block.kind === 'tool_call' && block.done && isBrainToolName(block.toolName);
}

function hasMeaningfulText(text: TextBlock): boolean {
  return !isLowValueToolFollowupText(text.text);
}

export function getDisplayBlocks(blocks: ContentBlock[]): ContentBlock[] {
  const brainToolBlocks = blocks.filter(isCompletedBrainTool);
  if (brainToolBlocks.length === 0) {
    return blocks;
  }

  const otherToolBlocks = blocks.filter((block) => block.kind === 'tool_call' && !isCompletedBrainTool(block));
  const meaningfulTextBlocks = blocks.filter((block): block is TextBlock => block.kind === 'text' && hasMeaningfulText(block));
  const thinkingBlocks = blocks.filter((block) => block.kind === 'thinking');

  const ordered: ContentBlock[] = [
    ...thinkingBlocks,
    ...brainToolBlocks,
    ...otherToolBlocks,
    ...meaningfulTextBlocks,
  ];

  return ordered;
}
