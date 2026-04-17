import { create } from 'zustand';
import { authApi, userApi } from '@/api/endpoints';
import { setRefreshHandler, setOnUnauthorized } from '@/api/client';
import { storage } from '@/lib/storage';
import type { User } from '@/api/types';

interface AuthState {
  user: User | null;
  refreshToken: string | null;
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
  refreshToken: null,
  status: 'bootstrapping',

  bootstrap: async () => {
    if (!storage.getAccess()) {
      set({ status: 'anonymous' });
      return;
    }
    try {
      const user = await userApi.me();
      set({ user, status: 'authenticated' });
    } catch {
      storage.setAccess(null);
      set({ status: 'anonymous' });
    }
  },

  login: async (email, password) => {
    const res = await authApi.login({ email, password });
    storage.setAccess(res.tokens.accessToken);
    set({ user: res.user, refreshToken: res.tokens.refreshToken, status: 'authenticated' });
  },

  register: async (args) => {
    const res = await authApi.register(args);
    storage.setAccess(res.tokens.accessToken);
    set({ user: res.user, refreshToken: res.tokens.refreshToken, status: 'authenticated' });
  },

  logout: () => {
    storage.setAccess(null);
    set({ user: null, refreshToken: null, status: 'anonymous' });
  },

  refresh: async () => {
    const rt = get().refreshToken;
    if (!rt) return null;
    try {
      const res = await authApi.refresh(rt);
      storage.setAccess(res.tokens.accessToken);
      set({ user: res.user, refreshToken: res.tokens.refreshToken, status: 'authenticated' });
      return res.tokens.accessToken;
    } catch {
      storage.setAccess(null);
      set({ user: null, refreshToken: null, status: 'anonymous' });
      return null;
    }
  },

  setUser: (u) => set({ user: u }),
}));

setRefreshHandler(() => useAuth.getState().refresh());
setOnUnauthorized(() => useAuth.getState().logout());
