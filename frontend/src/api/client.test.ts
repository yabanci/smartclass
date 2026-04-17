import { describe, expect, it } from 'vitest';
import { ApiErrorObj, errorMessage } from './client';

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
