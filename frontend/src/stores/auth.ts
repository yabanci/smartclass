import { create } from 'zustand';
import { authApi, userApi } from '@/api/endpoints';
import { setRefreshHandler, setOnUnauthorized } from '@/api/client';
import { storage } from '@/lib/storage';
import type { User } from '@/api/types';

interface AuthState {
  user: User | null;
  status: 'bootstrapping' | 'authenticated' | 'anonymous';
  bootstrap: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (args: { email: string; password: string; fullName: string; role: string; language?: string }) => Promise<void>;
  logout: () => void;
  refresh: () => Promise<string | null>;
  setUser: (u: User) => void;
}

export const useAuth = create<AuthState>((set, get) => ({
  user: null,
  status: 'bootstrapping',

  bootstrap: async () => {
    // bootstrap must not override state the user has already moved past
    // (e.g. submitted a login form before /me finished resolving).
    const stillBootstrapping = () => get().status === 'bootstrapping';

    if (!storage.getAccess() && !storage.getRefresh()) {
      if (stillBootstrapping()) set({ status: 'anonymous' });
      return;
    }
    try {
      const user = await userApi.me();
      if (stillBootstrapping()) set({ user, status: 'authenticated' });
      return;
    } catch {
      // access token missing or expired — try refresh
    }
    if (!stillBootstrapping()) return;
    const fresh = await get().refresh();
    if (fresh) {
      try {
        const user = await userApi.me();
        if (stillBootstrapping()) set({ user, status: 'authenticated' });
        return;
      } catch {
        /* fall through */
      }
    }
    if (stillBootstrapping()) {
      storage.clear();
      set({ status: 'anonymous' });
    }
  },

  login: async (email, password) => {
    const res = await authApi.login({ email, password });
    storage.setAccess(res.tokens.accessToken);
    storage.setRefresh(res.tokens.refreshToken);
    set({ user: res.user, status: 'authenticated' });
  },

  register: async (args) => {
    const res = await authApi.register(args);
    storage.setAccess(res.tokens.accessToken);
    storage.setRefresh(res.tokens.refreshToken);
    set({ user: res.user, status: 'authenticated' });
  },

  logout: () => {
    storage.clear();
    set({ user: null, status: 'anonymous' });
  },

  refresh: async () => {
    const rt = storage.getRefresh();
    if (!rt) return null;
    try {
      const res = await authApi.refresh(rt);
      storage.setAccess(res.tokens.accessToken);
      storage.setRefresh(res.tokens.refreshToken);
      set({ user: res.user, status: 'authenticated' });
      return res.tokens.accessToken;
    } catch {
      storage.clear();
      set({ user: null, status: 'anonymous' });
      return null;
    }
  },

  setUser: (u) => set({ user: u }),
}));

setRefreshHandler(() => useAuth.getState().refresh());
setOnUnauthorized(() => useAuth.getState().logout());
