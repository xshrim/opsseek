import { describe, expect, it } from 'vitest';
import { randomId } from './randomId';

describe('randomId', () => {
  it('returns a non-empty identifier', () => {
    const value = randomId();
    expect(value).toMatch(/^(.{8}-.{4}-.{4}-.{4}-.{12}|id-.+)$/);
  });
});
