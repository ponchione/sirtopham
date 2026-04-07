-- name: GetConversationTokenUsage :one
SELECT
    CAST(COALESCE(SUM(tokens_in), 0) AS INTEGER) AS total_in,
    CAST(COALESCE(SUM(tokens_out), 0) AS INTEGER) AS total_out,
    CAST(COALESCE(SUM(cache_read_tokens), 0) AS INTEGER) AS total_cache_hits,
    COUNT(*) AS total_calls,
    CAST(COALESCE(SUM(latency_ms), 0) AS INTEGER) AS total_latency_ms
FROM sub_calls
WHERE conversation_id = ? AND purpose = 'chat';

-- name: GetConversationCacheHitRate :one
SELECT
    CAST(COALESCE(SUM(cache_read_tokens) * 100.0 / NULLIF(SUM(tokens_in), 0), 0.0) AS REAL) AS cache_hit_pct
FROM sub_calls
WHERE conversation_id = ? AND purpose = 'chat';

-- name: GetConversationToolUsage :many
SELECT
    tool_name,
    COUNT(*) AS call_count,
    AVG(duration_ms) AS avg_duration,
    SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS failure_count
FROM tool_executions
WHERE conversation_id = ?
GROUP BY tool_name;

-- name: InsertToolExecution :exec
INSERT INTO tool_executions (
    conversation_id, turn_number, iteration,
    tool_use_id, tool_name, input,
    output_size, normalized_size, error, success,
    duration_ms, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetConversationContextQuality :one
SELECT
    COUNT(*) AS total_turns,
    SUM(agent_used_search_tool) AS reactive_search_turns,
    AVG(context_hit_rate) AS avg_hit_rate,
    AVG(CAST(budget_used AS REAL) * 100.0 / NULLIF(budget_total, 0)) AS avg_budget_used
FROM context_reports
WHERE conversation_id = ?;

-- name: GetConversationLastTurnUsage :one
-- Returns the latest chat turn's aggregated sub_calls usage. Used to populate
-- the per-conversation turn-usage chip on page reload (B3).
WITH latest_turn AS (
    SELECT MAX(sc.turn_number) AS n
    FROM sub_calls sc
    WHERE sc.conversation_id = ?
      AND sc.purpose = 'chat'
      AND sc.turn_number IS NOT NULL
)
SELECT
    sc.turn_number,
    CAST(COALESCE(MAX(sc.iteration), 1) AS INTEGER) AS iteration_count,
    CAST(COALESCE(SUM(sc.tokens_in), 0) AS INTEGER) AS tokens_in,
    CAST(COALESCE(SUM(sc.tokens_out), 0) AS INTEGER) AS tokens_out,
    CAST(COALESCE(SUM(sc.latency_ms), 0) AS INTEGER) AS latency_ms
FROM sub_calls sc, latest_turn lt
WHERE sc.conversation_id = ?
  AND sc.purpose = 'chat'
  AND sc.turn_number = lt.n
GROUP BY sc.turn_number;
