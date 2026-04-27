import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";

export function useApiResource<T>(
  path: string,
  fallback: T,
  normalize?: (value: T | null | undefined) => T,
) {
  const [data, setData] = useState<T>(fallback);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const mounted = useRef(true);

  const normalizeResponse = useCallback(
    (value: T | null | undefined) => normalize?.(value) ?? value ?? fallback,
    [fallback, normalize],
  );

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.get<T>(path);
      if (mounted.current) setData(normalizeResponse(response));
    } catch (err) {
      if (mounted.current) setError(err instanceof Error ? err.message : "Failed");
    } finally {
      if (mounted.current) setLoading(false);
    }
  }, [normalizeResponse, path]);

  useEffect(() => {
    mounted.current = true;
    refresh();
    return () => {
      mounted.current = false;
    };
  }, [refresh]);

  return { data, loading, error, refresh };
}
