import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { ConversationSummary } from "@/types/api";

export interface UseConversationListReturn {
  conversations: ConversationSummary[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
  deleteConversation: (id: string) => Promise<void>;
}

/**
 * Fetches and manages the conversation list from the REST API.
 * Auto-fetches on mount and provides refresh/delete capabilities.
 */
export function useConversationList(): UseConversationListReturn {
  const [conversations, setConversations] = useState<ConversationSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mounted = useRef(true);

  const fetchList = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.get<ConversationSummary[]>("/api/conversations?limit=50");
      if (mounted.current) {
        setConversations(data ?? []);
      }
    } catch (err) {
      if (mounted.current) {
        setError(err instanceof Error ? err.message : "Failed to load conversations");
      }
    } finally {
      if (mounted.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    fetchList();
    return () => {
      mounted.current = false;
    };
  }, [fetchList]);

  const deleteConversation = useCallback(
    async (id: string) => {
      await api.delete(`/api/conversations/${id}`);
      setConversations((prev) => prev.filter((c) => c.id !== id));
    },
    [],
  );

  return {
    conversations,
    loading,
    error,
    refresh: fetchList,
    deleteConversation,
  };
}
