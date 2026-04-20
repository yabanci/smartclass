import axios, { AxiosError, AxiosInstance, InternalAxiosRequestConfig } from 'axios';
import { storage } from '@/lib/storage';

export interface ApiError {
  code: string;
  message: string;
  details?: unknown;
}

export interface Envelope<T> {
  data?: T;
  error?: ApiError;
}

let refreshHandler: (() => Promise<string | null>) | null = null;
let onUnauthorized: (() => void) | null = null;

export function setRefreshHandler(fn: (() => Promise<string | null>) | null) {
  refreshHandler = fn;
}

export function setOnUnauthorized(fn: (() => void) | null) {
  onUnauthorized = fn;
}

export const client: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
});

client.interceptors.request.use((cfg: InternalAxiosRequestConfig) => {
  const token = storage.getAccess();
  if (token && !cfg.headers.Authorization) {
    cfg.headers.Authorization = `Bearer ${token}`;
  }
  const lang = localStorage.getItem('sc.lang');
  if (lang) cfg.headers['Accept-Language'] = lang;
  return cfg;
});

// When several requests fire in parallel and the access token has just
// expired, every one of them will hit 401. Without coordination each would
// call refreshHandler() — which POSTs /auth/refresh — producing N refresh
// round-trips and racing to write the new access token to storage. We gate
// with a single in-flight promise: the first 401 starts the refresh, the
// rest await that same promise and reuse the resulting token.
let pendingRefresh: Promise<string | null> | null = null;

async function sharedRefresh(): Promise<string | null> {
  if (!refreshHandler) return null;
  if (!pendingRefresh) {
    pendingRefresh = refreshHandler().finally(() => {
      pendingRefresh = null;
    });
  }
  return pendingRefresh;
}

client.interceptors.response.use(
  (r) => r,
  async (err: AxiosError<Envelope<unknown>>) => {
    const original = err.config as InternalAxiosRequestConfig & { _retry?: boolean };
    // Auth endpoints (/auth/login, /auth/register, /auth/refresh) must never
    // trigger the refresh-and-retry path: a 401 from /auth/refresh means the
    // refresh token itself is invalid, so retrying would call refresh again
    // and await the same in-flight promise — a deadlock that freezes the app
    // on the splash screen. Fail fast for anything under /auth.
    const url = original?.url ?? '';
    const isAuthEndpoint = url.startsWith('/auth/') || url.startsWith('auth/');
    if (
      err.response?.status === 401 &&
      !original?._retry &&
      !isAuthEndpoint &&
      refreshHandler
    ) {
      original._retry = true;
      const fresh = await sharedRefresh();
      if (fresh) {
        original.headers = original.headers ?? {};
        original.headers.Authorization = `Bearer ${fresh}`;
        return client.request(original);
      }
      onUnauthorized?.();
    }
    return Promise.reject(err);
  },
);

// Exported for tests.
export const __testing = {
  sharedRefresh,
  resetPendingRefresh: () => {
    pendingRefresh = null;
  },
};

export function extract<T>(env: Envelope<T>): T {
  if (env.error) throw new ApiErrorObj(env.error);
  return env.data as T;
}

export class ApiErrorObj extends Error {
  code: string;
  details?: unknown;
  constructor(err: ApiError) {
    super(err.message);
    this.code = err.code;
    this.details = err.details;
  }
}

export function errorMessage(err: unknown): string {
  if (err instanceof ApiErrorObj) return err.message;
  if (axios.isAxiosError(err)) {
    const e = err.response?.data?.error;
    if (e?.message) return e.message;
    return err.message;
  }
  if (err instanceof Error) return err.message;
  return 'Unexpected error';
}
