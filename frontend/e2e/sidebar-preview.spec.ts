import { expect, test } from '@playwright/test';

test('ten sidebar alternatives preserve identity space and toggle access', async ({
  page
}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('preview/sidebar-collapse-solutions-preview.html');
  await expect(page.locator('.option')).toHaveCount(10);
  await page.getByLabel('测试用户名').fill('平台工程自举诊断测试管理员');
  for (const stage of await page.locator('.stage').all()) {
    const name = stage.locator('.name');
    const toggle = stage.getByRole('button', { name: /收起方案/ });
    await expect(name).toBeVisible();
    const nameBox = await name.boundingBox();
    const toggleBox = await toggle.boundingBox();
    expect(nameBox).not.toBeNull();
    expect(toggleBox).not.toBeNull();
    if (nameBox && toggleBox) {
      expect(
        nameBox.x + nameBox.width <= toggleBox.x ||
          toggleBox.x + toggleBox.width <= nameBox.x ||
          nameBox.y + nameBox.height <= toggleBox.y ||
          toggleBox.y + toggleBox.height <= nameBox.y
      ).toBeTruthy();
    }
    await toggle.click();
    await expect(stage).toHaveAttribute('data-collapsed', 'true');
    await expect(stage.locator('.user')).toHaveAttribute(
      'title',
      '平台工程自举诊断测试管理员'
    );
    const expand = stage.getByRole('button', { name: /展开方案/ });
    await expect(expand).toBeVisible();
    await expand.focus();
    await page.keyboard.press('Enter');
    await expect(stage).toHaveAttribute('data-collapsed', 'false');
    await expect(name).toBeVisible();
  }
  await page.getByRole('button', { name: '全部收起' }).click();
  await expect(page.locator('.stage[data-collapsed="true"]')).toHaveCount(10);
  await page.getByRole('button', { name: '全部展开' }).click();
  await expect(page.locator('.stage[data-collapsed="false"]')).toHaveCount(10);
  await page.getByRole('button', { name: '切换深色' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth
    )
  ).toBeTruthy();
  expect(errors).toEqual([]);
});
