import { expect, test } from 'vitest';

import { getDisplayBlocks, isLowValueToolFollowupText, isBrainToolName } from '../src/lib/tool-transcript.ts';
import type { ContentBlock } from '../src/hooks/use-conversation.ts';

test('isBrainToolName matches brain tools', () => {
  expect(isBrainToolName('brain_read')).toBe(true);
  expect(isBrainToolName('brain_search')).toBe(true);
  expect(isBrainToolName('shell')).toBe(false);
});

test('isLowValueToolFollowupText suppresses redundant brain narration', () => {
  expect(isLowValueToolFollowupText('I already have the content of notes/hello.md.')).toBe(true);
  expect(isLowValueToolFollowupText('I\'ve already searched for that phrase and the results are the same.')).toBe(true);
  expect(isLowValueToolFollowupText('Done.')).toBe(true);
  expect(isLowValueToolFollowupText('The note now contains two appended lines.')).toBe(false);
});

test('getDisplayBlocks prioritizes brain tool cards and removes redundant follow-up text', () => {
  const blocks: ContentBlock[] = [
    { kind: 'text', text: 'I already have the content of notes/hello.md.' },
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_read', output: '', result: 'Brain document: notes/hello.md', done: true, success: true },
    { kind: 'text', text: 'I\'ve already searched for it.' },
  ];

  const display = getDisplayBlocks(blocks);
  expect(display).toEqual([blocks[1]]);
});

test('getDisplayBlocks keeps meaningful assistant analysis after a brain tool', () => {
  const blocks: ContentBlock[] = [
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_update', output: '', result: 'Updated brain document', done: true, success: true },
    { kind: 'text', text: 'The replace_section failed because the heading does not exist yet.' },
  ];

  const display = getDisplayBlocks(blocks);
  expect(display).toEqual(blocks);
});

test('getDisplayBlocks moves brain tool cards ahead of text blocks', () => {
  const blocks: ContentBlock[] = [
    { kind: 'text', text: 'Let me check that.' },
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_search', output: '', result: 'Found 1 brain document', done: true, success: true },
    { kind: 'text', text: 'The note exists and is readable.' },
  ];

  const display = getDisplayBlocks(blocks);
  expect(display[0]?.kind).toBe('tool_call');
  expect(display[1]?.kind).toBe('text');
  expect((display[1] as Extract<ContentBlock, { kind: 'text' }>).text).toBe('The note exists and is readable.');
});
