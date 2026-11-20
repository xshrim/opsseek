import { readFileSync } from 'node:fs';
import { expect, test } from '@playwright/test';

// Preserve the real entry's stylesheet order without starting the application's API calls.
const imports = [
  ...readFileSync('src/main.ts', 'utf8').matchAll(/import '(\.\/[^']+\.css)'/g)
]
  .map((match) => `import '/opsseek/src/${match[1].slice(2)}';`)
  .join('\n');

for (const theme of ['light', 'dark']) {
  test(`shared style cascade remains stable (${theme})`, async ({ page }) => {
    await page.route('**/style-cascade.html', (route) =>
      route.fulfill({
        contentType: 'text/html',
        body: `<html data-theme="${theme}"><body><main id="fixture"></main><script type="module">
        ${imports}
        import '/opsseek/e2e/fixtures/style-cascade.ts';
      </script></body></html>`
      })
    );
    await page.goto('style-cascade.html');
    const stateColor =
      theme === 'light' ? 'rgb(255, 249, 244)' : 'rgb(42, 41, 38)';
    for (const skin of [
      'access-member-table',
      'resource-expandable-table',
      'application-table'
    ]) {
      const table = page.getByRole('region', { name: skin });
      await expect(table.locator('tr.selected > td').first()).toHaveCSS(
        'background-color',
        stateColor
      );
      await expect(
        table.locator('col.expandable-table-select-column')
      ).toHaveCSS('width', '24px');
      await table.getByRole('button', { name: '展开 应用二' }).click();
      await expect(
        table.locator('.expandable-table-details-row > td')
      ).toHaveCSS('background-color', stateColor);
      await expect(table.locator('.readonly-table tbody td')).toHaveCSS(
        'background-color',
        stateColor
      );
      await table
        .locator('tbody > tr')
        .filter({ hasText: '应用二' })
        .first()
        .hover();
      await expect(
        table
          .locator('tbody > tr')
          .filter({ hasText: '应用二' })
          .first()
          .locator('td')
          .first()
      ).toHaveCSS('background-color', stateColor);
    }
    const tokens = await page.evaluate(() => {
      const styles = getComputedStyle(document.documentElement);
      const color = (name: string) => {
        const probe = document.createElement('span');
        probe.style.color = styles.getPropertyValue(name);
        document.body.append(probe);
        const value = getComputedStyle(probe).color;
        probe.remove();
        return value;
      };
      return {
        danger: color('--theme-danger'),
        dangerBg: color('--theme-bg-danger-soft'),
        editBg: color('--theme-bg-edit-hover'),
        selectedBg: color('--theme-bg-selected'),
        edit: color('--theme-icon-fg')
      };
    });
    for (const skin of [
      'resource-row-actions',
      'application-table-actions',
      'access-row-actions',
      'diagnosis-session-actions'
    ]) {
      const actions = page.locator(`[aria-label="${skin}"]`);
      await actions.locator('..').hover();
      for (const [name, foreground, background] of [
        [
          '编辑',
          tokens.edit,
          skin === 'resource-row-actions' && theme === 'light'
            ? tokens.editBg
            : tokens.selectedBg
        ],
        ['删除', tokens.danger, tokens.dangerBg]
      ]) {
        const button = actions.getByRole('button', { name });
        await button.hover();
        await expect(button).toHaveCSS('color', foreground);
        await expect(button).toHaveCSS('background-color', background);
        await expect(button).toHaveCSS('border-top-width', '0px');
        await page.mouse.down();
        await expect(button).toHaveCSS('background-color', background);
        await page.mouse.up();
      }
    }
    await page.getByLabel('技能行').hover();
    await expect(page.getByLabel('技能行')).toHaveCSS(
      'background-color',
      stateColor
    );
    await expect(page.getByLabel('应用依赖表格').locator('th')).toHaveCSS(
      'font-size',
      '12px'
    );
    const input = page.getByRole('textbox', { name: '错误字段' });
    await input.focus();
    await expect(input).toHaveCSS('border-top-color', tokens.danger);
    await expect(page.getByRole('alert')).toHaveCSS('position', 'absolute');
    await page.getByRole('button', { name: '打开确认窗口' }).click();
    const dialog = page.getByRole('dialog', { name: '确认操作' });
    await expect(dialog).toBeVisible();
    await expect(dialog).toHaveCSS('width', '480px');
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(dialog).toHaveCSS('width', '350px');
  });
}
