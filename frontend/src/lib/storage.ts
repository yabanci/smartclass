// Per security rules we don't put JWT in localStorage. sessionStorage is
// sufficient for this mobile-scoped app; refresh-token stays in memory only.
const ACCESS_KEY = 'sc.access';

export const storage = {
  getAccess(): string | null {
    try {
      return sessionStorage.getItem(ACCESS_KEY);
    } catch {
      return null;
    }
  },
  setAccess(token: string | null) {
    try {
      if (token) sessionStorage.setItem(ACCESS_KEY, token);
      else sessionStorage.removeItem(ACCESS_KEY);
    } catch {
      /* storage unavailable (private mode) */
    }
  },
};
