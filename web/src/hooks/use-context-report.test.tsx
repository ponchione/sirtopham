import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ContextReport } from "@/types/metrics";

const { apiGetMock } = vi.hoisted(() => ({
  apiGetMock: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      ...actual.api,
      get: apiGetMock,
    },
  };
});

import { useContextReport } from "./use-context-report";

const liveReport = (turnNumber: number): ContextReport => ({
  conversation_id: "conv-1",
  turn_number: turnNumber,
  created_at: "2026-04-13T00:00:00Z",
  signal_stream: [],
});

describe("useContextReport", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    apiGetMock.mockReset();
    apiGetMock.mockResolvedValue({ stream: [] });
  });

  it("does not fetch while the inspector is disabled", () => {
    const { result } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: false, enabled: false },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(apiGetMock).not.toHaveBeenCalled();
  });

  it("does not eagerly fetch the latest turn report before the defer window elapses", () => {
    const { result } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: false, enabled: true },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    expect(apiGetMock).not.toHaveBeenCalled();
  });

  it("fetches the latest turn report after the defer window when no live report arrives", () => {
    const { result } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: false, enabled: true },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(apiGetMock).toHaveBeenNthCalledWith(1, "/api/metrics/conversation/conv-1/context/1");
    expect(apiGetMock).toHaveBeenNthCalledWith(2, "/api/metrics/conversation/conv-1/context/1/signals");
  });

  it("cancels the deferred latest-turn fetch when a live context report arrives first", () => {
    const { result } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: false, enabled: true },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    act(() => {
      result.current.setLiveReport(liveReport(1));
    });

    act(() => {
      vi.runAllTimers();
    });

    expect(apiGetMock).not.toHaveBeenCalled();
  });

  it("waits for a live latest turn to finish before starting the deferred fetch window", () => {
    const { result, rerender } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: true, enabled: true },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(apiGetMock).not.toHaveBeenCalled();

    rerender({ pending: false, enabled: true });

    act(() => {
      vi.advanceTimersByTime(499);
    });

    expect(apiGetMock).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });

    expect(apiGetMock).toHaveBeenNthCalledWith(1, "/api/metrics/conversation/conv-1/context/1");
    expect(apiGetMock).toHaveBeenNthCalledWith(2, "/api/metrics/conversation/conv-1/context/1/signals");
  });

  it("does not fetch the latest turn if the live report arrives shortly after the old defer window", () => {
    const { result } = renderHook(({ pending, enabled }) => useContextReport("conv-1", pending, enabled), {
      initialProps: { pending: true, enabled: true },
    });

    act(() => {
      result.current.setHistoryTurns(1);
    });

    act(() => {
      vi.advanceTimersByTime(250);
    });

    act(() => {
      result.current.setLiveReport(liveReport(1));
    });

    act(() => {
      vi.runAllTimers();
    });

    expect(apiGetMock).not.toHaveBeenCalled();
  });
});
