// @vitest-environment node
import { describe, it, expect, vi } from "vitest";
import { createSessionCleanup } from "../../utils/session_cleanup.js";

const old = { id: "old", quiz_id: "quiz" };
describe("terminal host sessions", () => {
  it("forces a fresh read after an in-flight GET and hides terminal results", async () => {
    let resolve;
    const fetch = vi.fn().mockImplementationOnce(() => new Promise((r) => { resolve = r; }))
      .mockResolvedValueOnce([old]).mockResolvedValue([]);
    const published = [];
    const cleanup = createSessionCleanup(fetch, (s) => published.push(s), { delay: 0 });
    const first = cleanup.read();
    await Promise.resolve();
    const finished = cleanup.finish(old.id, old.quiz_id);
    resolve([old]);
    await first;
    expect(await finished).toBe(true);
    expect(published.every((s) => s.length === 0)).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(3);
  });
  it("cancelled leave retains the banner; confirmed leave hides it", async () => {
    let banner;
    const cleanup = createSessionCleanup(async () => [old], (s) => { banner = s; }, { attempts: 1 });
    await cleanup.read(); // Cancelling the guard does not call finish.
    expect(banner).toEqual([old]);
    expect(await cleanup.finish(old.id, old.quiz_id)).toBe(false);
    expect(banner).toEqual([]);
    expect(await cleanup.ready("quiz")).toBe(false);
    expect(await cleanup.ready("other")).toBe(true);
  });
  it("waits for cleanup before the first start creates a new session", async () => {
    let active = old;
    const cleanup = createSessionCleanup(async () => active ? [active] : [], () => {}, { delay: 0 });
    const finished = cleanup.finish(old.id, old.quiz_id);
    const start = async () => {
      if (!await cleanup.ready("quiz")) throw new Error("blocked");
      return active?.id || "new"; // demo_session reuses only an active session.
    };
    active = null;
    expect(await start()).toBe("new");
    expect(await finished).toBe(true);
  });
  it("fails closed on backend errors without retrying forever", async () => {
    const fetch = vi.fn().mockRejectedValue(new Error("DB unavailable"));
    const cleanup = createSessionCleanup(fetch, () => {});
    expect(await cleanup.finish(old.id, old.quiz_id)).toBe(false);
    expect(await cleanup.ready("quiz")).toBe(false);
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
