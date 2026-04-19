import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiErrorObj, errorMessage, setRefreshHandler, __testing } from './client';

describe('errorMessage', () => {
  it('returns ApiError message', () => {
    expect(errorMessage(new ApiErrorObj({ code: 'x', message: 'boom' }))).toBe('boom');
  });

  it('falls back to Error message', () => {
    expect(errorMessage(new Error('plain'))).toBe('plain');
  });

  it('returns default for non-errors', () => {
    expect(errorMessage('string')).toBe('Unexpected error');
    expect(errorMessage(undefined)).toBe('Unexpected error');
  });
});

// Regression: when N parallel requests race a 401, each used to fire its
// own refresh round-trip. sharedRefresh() collapses them into one upstream
// call so storage doesn't flicker between conflicting tokens.
describe('sharedRefresh', () => {
  afterEach(() => {
    __testing.resetPendingRefresh();
    setRefreshHandler(null);
  });

  it('coalesces N parallel callers into one refresh invocation', async () => {
    let resolveRefresh: (v: string) => void = () => {};
    const refresh = vi.fn(
      () =>
        new Promise<string | null>((resolve) => {
          resolveRefresh = resolve;
        }),
    );
    setRefreshHandler(refresh);

    const inflight = Promise.all([
      __testing.sharedRefresh(),
      __testing.sharedRefresh(),
      __testing.sharedRefresh(),
    ]);
    resolveRefresh('new-access');
    const results = await inflight;

    expect(refresh).toHaveBeenCalledTimes(1);
    expect(results).toEqual(['new-access', 'new-access', 'new-access']);
  });

  it('allows a fresh refresh after the previous one settles', async () => {
    const refresh = vi
      .fn<() => Promise<string | null>>()
      .mockResolvedValueOnce('t1')
      .mockResolvedValueOnce('t2');
    setRefreshHandler(refresh);

    expect(await __testing.sharedRefresh()).toBe('t1');
    expect(await __testing.sharedRefresh()).toBe('t2');
    expect(refresh).toHaveBeenCalledTimes(2);
  });

  it('returns null when no handler is installed', async () => {
    setRefreshHandler(null);
    expect(await __testing.sharedRefresh()).toBeNull();
  });
});
