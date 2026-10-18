import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { api, ApiError } from "../../lib/api";
import { TodayDiary } from "./TodayDiary";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("switches the displayed diary at midnight in JST", async () => {
  const NativeDate = Date;
  let now = "2026-10-18T14:59:00Z";
  vi.stubGlobal("Date", class extends NativeDate {
    constructor(value?: string | number) { super(value ?? now); }
    static now() { return new NativeDate(now).getTime(); }
  });
  const file = vi.spyOn(api, "file").mockRejectedValue(new ApiError(404, "not found"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  try {
    render(<QueryClientProvider client={client}><MemoryRouter><TodayDiary /></MemoryRouter></QueryClientProvider>);
    await waitFor(() => expect(file).toHaveBeenCalledWith("daily/2026/10/18.md"));
    now = "2026-10-18T15:01:00Z";
    fireEvent(window, new Event("focus"));
    await waitFor(() => expect(file).toHaveBeenCalledWith("daily/2026/10/19.md"));
    expect(screen.queryByRole("textbox")).toBeNull();
  } finally {
    client.clear();
    vi.unstubAllGlobals();
  }
});
