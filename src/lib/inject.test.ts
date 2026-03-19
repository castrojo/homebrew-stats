import { describe, it, expect } from 'vitest';
import { safeJson } from './inject.js';

describe('safeJson', () => {
  it('returns valid JSON for normal data', () => {
    const input = { name: 'test', count: 42, active: true };
    const output = safeJson(input);
    expect(JSON.parse(output)).toEqual(input);
  });

  it('escapes </script> so output contains \\u003c not <', () => {
    const input = { payload: '</script><script>alert(1)</script>' };
    const output = safeJson(input);
    expect(output).not.toContain('<');
    expect(output).toContain('\\u003c');
    // Must still round-trip correctly
    expect(JSON.parse(output)).toEqual(input);
  });

  it('escapes < inside nested objects and arrays', () => {
    const input = {
      items: ['<b>bold</b>', '</script>'],
      meta: { tag: '<div>' },
    };
    const output = safeJson(input);
    expect(output).not.toContain('<');
    expect(JSON.parse(output)).toEqual(input);
  });

  it('handles empty object', () => {
    expect(JSON.parse(safeJson({}))).toEqual({});
  });

  it('handles arrays at the root level', () => {
    const input = [1, 'two', null, false];
    expect(JSON.parse(safeJson(input))).toEqual(input);
  });

  it('handles null', () => {
    expect(JSON.parse(safeJson(null))).toBeNull();
  });

  it('handles strings with no special characters unchanged', () => {
    const output = safeJson('hello world');
    expect(output).toBe('"hello world"');
  });
});
