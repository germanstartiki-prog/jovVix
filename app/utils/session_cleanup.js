// Serializes active-session reads and keeps terminal sessions out of the UI.
export function createSessionCleanup(fetchSessions, publish, { attempts = 10, delay = 200 } = {}) {
  let request = null;
  const terminal = new Map();
  const read = (force = false) => {
    if (request && !force) return request;
    const run = async () => {
      const sessions = await fetchSessions();
      for (const [id, state] of terminal) {
        const found = sessions.find((s) => s.id === id);
        if (found) state.quizId = found.quiz_id;
      }
      publish(sessions.filter((s) => !terminal.has(s.id)));
      return sessions;
    };
    const next = request ? request.then(run, run) : Promise.resolve().then(run);
    request = next;
    const clear = () => { if (request === next) request = null; };
    next.then(clear, clear);
    return next;
  };
  const finish = (id, quizId) => {
    if (terminal.has(id)) return terminal.get(id).completion;
    const state = { quizId, completion: null };
    terminal.set(id, state);
    const deadline = Date.now() + 4000;
    state.completion = (async () => {
      try {
        for (let i = 0; i < attempts; i++) {
          const remaining = deadline - Date.now();
          if (remaining <= 0) return false;
          let timer;
          let sessions;
          try {
            sessions = await Promise.race([
              read(true),
              new Promise((_, reject) => { timer = setTimeout(() => reject(new Error("Cleanup timed out")), remaining); }),
            ]);
          } finally { clearTimeout(timer); }
          if (!sessions.some((s) => s.id === id)) {
            terminal.delete(id); // Serialized reads cannot restore an older result.
            return true;
          }
          if (i + 1 < attempts) await new Promise((resolve) => setTimeout(resolve, delay));
        }
      } catch { /* Keep this session blocked on failure. */ }
      return false;
    })();
    return state.completion;
  };
  const ready = async (quizId) => {
    for (const state of terminal.values()) {
      if ((!state.quizId || state.quizId === quizId) && !await state.completion) return false;
    }
    return true;
  };
  return { read, finish, ready };
}
