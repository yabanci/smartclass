// Per security rules tokens never go to localStorage. sessionStorage is
// scoped to the tab, cleared when it closes, and survives page refreshes
// within the session — good enough for this mobile classroom app.
const ACCESS_KEY = 'sc.access';
const REFRESH_KEY = 'sc.refresh';

function read(key: string): string | null {
  try {
    return sessionStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null) {
  try {
    if (value) sessionStorage.setItem(key, value);
    else sessionStorage.removeItem(key);
  } catch {
    /* storage unavailable (private mode) */
  }
}

export const storage = {
  getAccess: () => read(ACCESS_KEY),
  setAccess: (token: string | null) => write(ACCESS_KEY, token),
  getRefresh: () => read(REFRESH_KEY),
  setRefresh: (token: string | null) => write(REFRESH_KEY, token),
  clear: () => {
    write(ACCESS_KEY, null);
    write(REFRESH_KEY, null);
  },
};
