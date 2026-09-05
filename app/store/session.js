import { defineStore } from "pinia";
import { useUsersStore } from "~~/store/users";

import { createSessionCleanup } from "../utils/session_cleanup.js";

const SYNC_THROTTLE_MS = 30000;

export const useSessionStore = defineStore(
  "session-store",
  () => {
    const session = ref(false);
    const lastComponent = ref("");
    const activeQuizTitle = ref("");
    const playSession = ref(null);
    const activeSessions = ref([]);
    const sessionsSyncedAt = ref(0);
    const sessionsLoading = ref(false);

    const getSession = () => {
      return session.value;
    };

    const setSession = (data) => {
      session.value = data;
    };

    const getLastComponent = () => {
      return lastComponent.value;
    };

    const setLastComponent = (data) => {
      lastComponent.value = data;
    };

    const getActiveQuizTitle = () => activeQuizTitle.value;

    const setActiveQuizTitle = (title) => {
      if (typeof title === "string" && title.trim()) {
        activeQuizTitle.value = title.trim();
      }
    };

    const setPlaySession = (data) => {
      playSession.value = { ...data, code: String(data.code) };
    };

    const getPlaySessionFor = (code) =>
      playSession.value?.code === String(code) ? playSession.value : null;

    const clearPlaySession = () => {
      playSession.value = null;
    };

    const getActiveSessions = () => activeSessions.value;

    const removeActiveSession = (id) => {
      activeSessions.value = activeSessions.value.filter((s) => s.id !== id);
      if (session.value === id) {
        session.value = null;
      }
    };

    const cleanup = createSessionCleanup(async () => {
      sessionsLoading.value = true;
      try {
        const { apiUrl } = useRuntimeConfig().public;
        const res = await $fetch(`${apiUrl}/quiz/sessions/active`, {
          method: "GET", credentials: "include", timeout: 1000, retry: 0,
        });
        if (!Array.isArray(res?.data)) throw new Error("Invalid active sessions response");
        return res.data;
      } finally { sessionsLoading.value = false; }
    }, (sessions) => {
      activeSessions.value = sessions;
      sessionsSyncedAt.value = Date.now();
    });

    const finishHostSession = (id) => {
      const quizId = activeSessions.value.find((s) => s.id === id)?.quiz_id;
      removeActiveSession(id);
      return cleanup.finish(id, quizId);
    };
    const canStartQuiz = (quizId) => cleanup.ready(quizId);

    const syncActiveSessions = async ({ force = false } = {}) => {
      if (!import.meta.client) return;
      const usersStore = useUsersStore();
      if (!usersStore.getUserData() || !usersStore.authConfirmed) {
        activeSessions.value = [];
        return;
      }
      if (!force && Date.now() - sessionsSyncedAt.value < SYNC_THROTTLE_MS) return;
      try { await cleanup.read(force); }
      catch (error) {
        const status = error?.response?.status || error?.statusCode;
        if (status === 401 || status === 403) {
          usersStore.invalidateAuth();
          activeSessions.value = [];
          sessionsSyncedAt.value = Date.now();
        }
      }
    };

    return {
      session,
      getSession,
      setSession,
      lastComponent,
      getLastComponent,
      setLastComponent,
      activeQuizTitle,
      getActiveQuizTitle,
      setActiveQuizTitle,
      playSession,
      setPlaySession,
      getPlaySessionFor,
      clearPlaySession,
      activeSessions,
      sessionsLoading,
      getActiveSessions,
      removeActiveSession,
      syncActiveSessions,
      finishHostSession,
      canStartQuiz,
    };
  },
  {
    persist: [
      { key: "session-store", pick: ["activeQuizTitle"] },
      {
        key: "session-store-play",
        pick: ["playSession"],
        storage: import.meta.client ? window.sessionStorage : undefined,
      },
    ],
  }
);
