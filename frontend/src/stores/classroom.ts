import { create } from 'zustand';

interface ActiveClassroom {
  id: string | null;
  set: (id: string | null) => void;
}

const KEY = 'sc.activeClassroom';

export const useActiveClassroom = create<ActiveClassroom>((set) => ({
  id: typeof window !== 'undefined' ? localStorage.getItem(KEY) : null,
  set: (id) => {
    try {
      if (id) localStorage.setItem(KEY, id);
      else localStorage.removeItem(KEY);
    } catch {
      /* ignore */
    }
    set({ id });
  },
}));
