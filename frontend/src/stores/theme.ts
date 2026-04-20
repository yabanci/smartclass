import { create } from 'zustand';

type Mode = 'light' | 'dark';

const KEY = 'sc-theme';

function read(): Mode {
  try {
    return localStorage.getItem(KEY) === 'dark' ? 'dark' : 'light';
  } catch {
    return 'light';
  }
}

function apply(mode: Mode) {
  const root = document.documentElement;
  if (mode === 'dark') root.classList.add('dark');
  else root.classList.remove('dark');
  try {
    localStorage.setItem(KEY, mode);
  } catch {
    /* ignore */
  }
}

interface ThemeState {
  mode: Mode;
  toggle: () => void;
  set: (mode: Mode) => void;
}

const initial = read();
apply(initial);

export const useTheme = create<ThemeState>((set, get) => ({
  mode: initial,
  toggle: () => {
    const next: Mode = get().mode === 'dark' ? 'light' : 'dark';
    apply(next);
    set({ mode: next });
  },
  set: (mode) => {
    apply(mode);
    set({ mode });
  },
}));
