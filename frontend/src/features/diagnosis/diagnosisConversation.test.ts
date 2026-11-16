import { describe, expect, it } from 'vitest';
import {
  diagnosisDayKey,
  diagnosisDayLabel,
  diagnosisHasOlderConversation
} from './diagnosisConversation';

describe('diagnosis conversation day grouping', () => {
  it('uses the local calendar day as the grouping key', () => {
    const value = '2026-03-08T12:00:00.000Z';
    const date = new Date(value);
    const expected = [date.getFullYear(), date.getMonth() + 1, date.getDate()]
      .map((part, index) =>
        index === 0 ? String(part) : String(part).padStart(2, '0')
      )
      .join('-');
    expect(diagnosisDayKey(value)).toBe(expected);
    expect(diagnosisDayKey('2026-03-08T12:00:00.000Z')).not.toBe(
      diagnosisDayKey('2026-03-10T12:00:00.000Z')
    );
  });

  it('returns a readable date label and tolerates invalid timestamps', () => {
    expect(diagnosisDayLabel('not-a-date')).toBe('not-a-date');
    expect(diagnosisDayLabel('2026-03-08T12:00:00.000Z')).toMatch(
      /2026|3月|3月8日/
    );
  });

  it('hides day separators for conversations younger than one day', () => {
    const now = Date.parse('2026-03-10T12:00:00.000Z');
    expect(diagnosisHasOlderConversation('2026-03-09T12:00:01.000Z', now)).toBe(
      false
    );
    expect(diagnosisHasOlderConversation('2026-03-09T12:00:00.000Z', now)).toBe(
      true
    );
    expect(diagnosisHasOlderConversation('not-a-date', now)).toBe(false);
  });
});
