-- name: ReconstructConversationHistory :many
SELECT role, content, tool_use_id, tool_name
FROM messages
WHERE conversation_id = ? AND is_compressed = 0
ORDER BY sequence;

-- name: ListActiveMessages :many
SELECT id, conversation_id, role, content, tool_use_id, tool_name, turn_number, iteration, sequence,
       is_compressed, is_summary, compressed_turn_start, compressed_turn_end, created_at
FROM messages
WHERE conversation_id = ? AND is_compressed = 0
ORDER BY sequence;

-- name: NextMessageSequence :one
SELECT COALESCE(MAX(sequence) + 1.0, 0.0)
FROM messages
WHERE conversation_id = ?;

-- name: InsertUserMessage :exec
INSERT INTO messages (
    conversation_id,
    role,
    content,
    turn_number,
    iteration,
    sequence,
    created_at
) VALUES (
    ?,
    'user',
    ?,
    ?,
    1,
    ?,
    ?
);

-- name: TouchConversationUpdatedAt :exec
UPDATE conversations
SET updated_at = ?
WHERE id = ?;

-- name: ListConversations :many
SELECT id, title, updated_at
FROM conversations
WHERE project_id = ?
ORDER BY updated_at DESC
LIMIT ? OFFSET ?;

-- name: ListTurnMessages :many
SELECT id, role, content, tool_use_id, tool_name, turn_number, iteration, sequence
FROM messages
WHERE conversation_id = ?
ORDER BY sequence;

-- name: SearchConversations :many
SELECT c.id, c.title, c.updated_at, snippet(messages_fts, 0, '<b>', '</b>', '...', 32) AS snippet
FROM messages_fts
JOIN messages m ON m.id = messages_fts.rowid
JOIN conversations c ON c.id = m.conversation_id
WHERE messages_fts.content MATCH ?
ORDER BY rank
LIMIT 20;
