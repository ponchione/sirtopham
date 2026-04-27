import { useCallback } from "react";
import { useApiResource } from "@/hooks/use-api-resource";
import type { ProjectInfo } from "@/types/metrics";

export function useProjectInfo() {
  const normalize = useCallback((value: ProjectInfo | null | undefined) => {
    return value ?? null;
  }, []);
  const { data: project, loading, error, refresh } = useApiResource<ProjectInfo | null>(
    "/api/project",
    null,
    normalize,
  );
  return { project, loading, error, refresh };
}
