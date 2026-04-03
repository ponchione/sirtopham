import test from 'node:test';
import assert from 'node:assert/strict';

import { getDisplayBlocks, isLowValueToolFollowupText, isBrainToolName } from '../src/lib/tool-transcript.ts';
import type { ContentBlock } from '../src/hooks/use-conversation.ts';

test('isBrainToolName matches brain tools', () => {
  assert.equal(isBrainToolName('brain_read'), true);
  assert.equal(isBrainToolName('brain_search'), true);
  assert.equal(isBrainToolName('shell'), false);
});

test('isLowValueToolFollowupText suppresses redundant brain narration', () => {
  assert.equal(isLowValueToolFollowupText('I already have the content of notes/hello.md.'), true);
  assert.equal(isLowValueToolFollowupText('I\'ve already searched for that phrase and the results are the same.'), true);
  assert.equal(isLowValueToolFollowupText('Done.'), true);
  assert.equal(isLowValueToolFollowupText('The note now contains two appended lines.'), false);
});

test('getDisplayBlocks prioritizes brain tool cards and removes redundant follow-up text', () => {
  const blocks: ContentBlock[] = [
    { kind: 'text', text: 'I already have the content of notes/hello.md.' },
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_read', output: '', result: 'Brain document: notes/hello.md', done: true, success: true },
    { kind: 'text', text: 'I\'ve already searched for it.' },
  ];

  const display = getDisplayBlocks(blocks);
  assert.deepEqual(display, [blocks[1]]);
});

test('getDisplayBlocks keeps meaningful assistant analysis after a brain tool', () => {
  const blocks: ContentBlock[] = [
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_update', output: '', result: 'Updated brain document', done: true, success: true },
    { kind: 'text', text: 'The replace_section failed because the heading does not exist yet.' },
  ];

  const display = getDisplayBlocks(blocks);
  assert.deepEqual(display, blocks);
});

test('getDisplayBlocks moves brain tool cards ahead of text blocks', () => {
  const blocks: ContentBlock[] = [
    { kind: 'text', text: 'Let me check that.' },
    { kind: 'tool_call', toolCallId: '1', toolName: 'brain_search', output: '', result: 'Found 1 brain document', done: true, success: true },
    { kind: 'text', text: 'The note exists and is readable.' },
  ];

  const display = getDisplayBlocks(blocks);
  assert.equal(display[0]?.kind, 'tool_call');
  assert.equal(display[1]?.kind, 'text');
  assert.equal((display[1] as Extract<ContentBlock, { kind: 'text' }>).text, 'The note exists and is readable.');
});
