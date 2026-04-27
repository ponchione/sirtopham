import { useCallback } from "react";
import { useApiResource } from "@/hooks/use-api-resource";
import type { ProviderStatus } from "@/types/metrics";

const emptyProviders: ProviderStatus[] = [];

export function useProviders() {
  const normalize = useCallback((value: ProviderStatus[] | null | undefined) => {
    return value ?? emptyProviders;
  }, []);
  const { data: providers, loading, error, refresh } = useApiResource<ProviderStatus[]>(
    "/api/providers",
    emptyProviders,
    normalize,
  );
  return { providers, loading, error, refresh };
}
