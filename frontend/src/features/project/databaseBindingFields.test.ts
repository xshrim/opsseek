import { expect, it } from 'vitest';
import { databaseBindingFields } from './databaseBindingFields';

it('uses engine-specific database concepts without uncertain session selectors', () => {
  for (const [kind, mode, keys] of [
    ['PostgreSQL', '', ['database', 'schema', 'username']],
    ['MySQL', '', ['database', 'username']],
    ['Oracle', '', ['service_name', 'sid', 'schema', 'username']],
    ['OceanBase', 'mysql', ['database', 'username']],
    ['OceanBase', 'oracle', ['service_name', 'schema', 'username']],
    ['OceanBase', '', ['database', 'schema', 'username']]
  ] as const) {
    expect(databaseBindingFields(kind, mode).map((field) => field.key)).toEqual(
      keys
    );
  }
});
