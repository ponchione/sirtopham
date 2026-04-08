import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import type { ConversationSummary, SearchResult } from "@/types/api";

export interface UseConversationListReturn {
  conversations: ConversationSummary[];
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  searchResults: SearchResult[];
  searching: boolean;
  searchError: string | null;
  showingSearchResults: boolean;
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
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mounted = useRef(true);
  const searchDebounce = useRef<number | null>(null);

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
      if (searchDebounce.current !== null) {
        window.clearTimeout(searchDebounce.current);
      }
      mounted.current = false;
    };
  }, [fetchList]);

  const normalizedSearchQuery = useMemo(() => searchQuery.trim(), [searchQuery]);

  useEffect(() => {
    if (searchDebounce.current !== null) {
      window.clearTimeout(searchDebounce.current);
      searchDebounce.current = null;
    }

    if (normalizedSearchQuery === "") {
      setSearchResults([]);
      setSearchError(null);
      setSearching(false);
      return;
    }

    setSearching(true);
    setSearchError(null);
    searchDebounce.current = window.setTimeout(async () => {
      try {
        const data = await api.get<SearchResult[]>(
          `/api/conversations/search?q=${encodeURIComponent(normalizedSearchQuery)}`,
        );
        if (mounted.current) {
          setSearchResults(data ?? []);
        }
      } catch (err) {
        if (mounted.current) {
          setSearchError(err instanceof Error ? err.message : "Failed to search conversations");
          setSearchResults([]);
        }
      } finally {
        if (mounted.current) {
          setSearching(false);
        }
      }
    }, 250);

    return () => {
      if (searchDebounce.current !== null) {
        window.clearTimeout(searchDebounce.current);
        searchDebounce.current = null;
      }
    };
  }, [normalizedSearchQuery]);

  const deleteConversation = useCallback(
    async (id: string) => {
      await api.delete(`/api/conversations/${id}`);
      setConversations((prev) => prev.filter((c) => c.id !== id));
      setSearchResults((prev) => prev.filter((c) => c.id !== id));
    },
    [],
  );

  return {
    conversations,
    searchQuery,
    setSearchQuery,
    searchResults,
    searching,
    searchError,
    showingSearchResults: normalizedSearchQuery !== "",
    loading,
    error,
    refresh: fetchList,
    deleteConversation,
  };
}
