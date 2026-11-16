function validDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

const DAY_IN_MILLISECONDS = 24 * 60 * 60 * 1000;

export function diagnosisDayKey(value: string) {
  const date = validDate(value);
  if (!date) return value.slice(0, 10) || value;
  return [date.getFullYear(), date.getMonth() + 1, date.getDate()]
    .map((part, index) =>
      index === 0 ? String(part) : String(part).padStart(2, '0')
    )
    .join('-');
}

export function diagnosisDayLabel(value: string) {
  const date = validDate(value);
  if (!date) return value;
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'short'
  }).format(date);
}

export function diagnosisHasOlderConversation(value: string, now = Date.now()) {
  const date = validDate(value);
  return Boolean(date && now - date.getTime() >= DAY_IN_MILLISECONDS);
}
