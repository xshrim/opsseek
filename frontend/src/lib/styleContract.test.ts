// @vitest-environment node
import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import postcss from 'postcss';
import { describe, expect, it } from 'vitest';

const sourceRoot = fileURLToPath(new URL('..', import.meta.url));
const sources = readdirSync(sourceRoot, { recursive: true, encoding: 'utf8' })
  .filter((file) => /\.(css|svelte)$/.test(file))
  .map((file) => ({
    file,
    source: readFileSync(resolve(sourceRoot, file), 'utf8')
  }));

const styles = sources.flatMap(({ file, source }) =>
  (file.endsWith('.css')
    ? [source]
    : [...source.matchAll(/<style(?:\s[^>]*)?>([\s\S]*?)<\/style>/g)].map(
        (match) => match[1]
      )
  ).map((css) => ({ file, root: postcss.parse(css) }))
);

describe('shared style contract', () => {
  it('does not leave Svelte-only selectors in plain CSS', () => {
    const issues: string[] = [];
    for (const { file, root } of styles) {
      if (!file.endsWith('.css')) continue;
      root.walkRules((rule) => {
        if (rule.selector.includes(':global(')) {
          issues.push(`${file}: ${rule.selector}`);
        }
      });
    }
    expect(issues).toEqual([]);
  });

  it('keeps action states, confirmation dialogs and table columns out of page-level priority patches', () => {
    const issues: string[] = [];
    for (const { file, root } of styles) {
      root.walkRules((rule) => {
        if (
          file.startsWith('features/') &&
          rule.selector.includes('.confirm-dialog')
        ) {
          issues.push(`${file}: shared confirmation dialog override`);
        }
        if (
          /\.(?:icon-button|edit-action|delete-action)\b|col:nth-child/.test(
            rule.selector
          )
        ) {
          rule.walkDecls((declaration) => {
            if (
              declaration.important &&
              /^(?:color|background|border|width)/.test(declaration.prop)
            ) {
              issues.push(
                `${file}: ${rule.selector}: ${declaration.toString()}`
              );
            }
          });
        }
      });
    }
    expect(issues).toEqual([]);
  });

  it('keeps literal colors in global tokens and forbids component color mixing', () => {
    const issues: string[] = [];
    for (const { file, root } of styles) {
      root.walkDecls((declaration) => {
        const globalToken =
          file === 'app.css' && declaration.prop.startsWith('--');
        const literal =
          /#[\da-f]{3,8}\b|\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(/i;
        const named =
          /\b(?:white|black|red|green|blue|orange|purple|gray|grey)\b/i;
        const value = declaration.value.replace(/var\(--[\w-]+\)/g, '');
        if (
          /color-mix\(/i.test(value) ||
          (!globalToken && (literal.test(value) || named.test(value)))
        ) {
          issues.push(`${file}: ${declaration.toString()}`);
        }
      });
    }
    expect(issues).toEqual([]);
  });

  it('resolves every required theme color reference', () => {
    const source = sources.map((item) => item.source).join('\n');
    const definitions = new Set(
      [...source.matchAll(/(--[\w-]+)\s*:/g)].map((match) => match[1])
    );
    const references = new Set(
      [
        ...source.matchAll(
          /var\((--(?:theme|color|resource-catalog)-[\w-]+)\s*\)/g
        )
      ].map((match) => match[1])
    );
    expect([...references].filter((name) => !definitions.has(name))).toEqual(
      []
    );
  });

  it('preserves separate subtle table state colors for both themes', () => {
    const root = styles.find((item) => item.file === 'app.css')!.root;
    const values: string[] = [];
    root.walkDecls('--theme-bg-table-row-selected', (declaration) => {
      values.push(declaration.value);
    });
    expect(values).toEqual(['#fff9f4', '#2a2926']);
    const skill = styles.find(
      (item) => item.file === 'features/skill/skill.css'
    )!.root;
    skill.walkRules((rule) => {
      if (
        rule.selector.includes('.skill-row:hover') ||
        rule.selector.includes('.skill-row.selected')
      ) {
        rule.walkDecls('background', (declaration) => {
          expect(declaration.value).toBe('var(--theme-bg-table-row-selected)');
        });
      }
    });
    const table = styles.find(
      (item) => item.file === 'components/ExpandableTable.svelte'
    )!.root;
    for (const state of [
      ':hover',
      'tr.selected',
      'tr.expandable-table-details-row'
    ]) {
      const backgrounds: string[] = [];
      table.walkRules((rule) => {
        if (!rule.selector.includes('tbody') || !rule.selector.includes(state))
          return;
        rule.walkDecls('background', (declaration) => {
          backgrounds.push(declaration.value);
        });
      });
      expect(backgrounds.length).toBeGreaterThan(0);
      expect(new Set(backgrounds)).toEqual(
        new Set(['var(--theme-bg-table-row-selected)'])
      );
    }
  });
});
