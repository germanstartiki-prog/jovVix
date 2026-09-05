import { defineStore } from "pinia";
export const useUsersStore = defineStore(
  "users-store",
  () => {
    const userData = ref(null);
    const authConfirmed = ref(false);

    const setUserData = (data) => {
      userData.value = data;
      authConfirmed.value = Boolean(data);
    };

    const getUserData = () => {
      return userData.value;
    };

    const confirmAuth = () => {
      authConfirmed.value = true;
    };

    const invalidateAuth = () => {
      authConfirmed.value = false;
    };

    return {
      userData,
      authConfirmed,
      setUserData,
      getUserData,
      confirmAuth,
      invalidateAuth,
    };
  },
  {
    persist: { pick: ["userData"] },
  }
);
