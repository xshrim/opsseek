import { expect, test } from '@playwright/test';

for (const theme of ['light', 'dark']) {
  test(`administrator initialization stays in place (${theme})`, async ({
    page
  }) => {
    await page.route('**/api/v1/**', (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ required: true })
      })
    );
    await page.emulateMedia({ colorScheme: theme as 'light' | 'dark' });
    await page.goto('./');
    const dialog = page.getByRole('dialog', { name: '初始化管理员' });
    await expect(dialog).toBeVisible();
    await expect
      .poll(async () => {
        const box = (await dialog.boundingBox())!;
        return Math.abs(box.x + box.width / 2 - page.viewportSize()!.width / 2);
      })
      .toBeLessThan(2);
    const initial = (await dialog.boundingBox())!;
    // Repeated content resizes must not accumulate native dialog offsets.
    for (const padding of ['48px', '24px', '40px', '24px']) {
      await dialog.locator('.bootstrap-panel').evaluate((element, padding) => {
        (element as HTMLElement).style.padding = padding;
      }, padding);
      await page.waitForTimeout(100);
      const box = (await dialog.boundingBox())!;
      expect(Math.abs(box.x - initial.x)).toBeLessThan(2);
      expect(Math.abs(box.y - initial.y)).toBeLessThan(2);
      expect(box.y + box.height).toBeLessThan(page.viewportSize()!.height);
    }
    const brand = page.getByLabel('OpsSeek 智能诊断平台');
    await expect(brand).toBeVisible();
    await expect(brand).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
    await expect(brand).toHaveCSS('box-shadow', 'none');
    const brandColor = await brand.evaluate(
      (element) => getComputedStyle(element).color
    );
    expect(brandColor).not.toBe(
      theme === 'light' ? 'rgb(231, 96, 27)' : 'rgb(24, 162, 125)'
    );
    await expect(brand.locator('small')).toHaveCSS('color', brandColor);
    await expect(brand).toHaveCSS('color', brandColor);
    await expect(brand.locator('svg')).toHaveCSS('color', brandColor);
    const faviconColor = () =>
      page.evaluate(() => {
        const href =
          document.querySelector<HTMLLinkElement>('link[rel="icon"]')!.href;
        return decodeURIComponent(href.slice(href.indexOf(',') + 1));
      });
    await expect.poll(faviconColor).toContain(`color:${brandColor}`);
    await page.emulateMedia({
      colorScheme: theme === 'light' ? 'dark' : 'light'
    });
    await expect
      .poll(() => brand.evaluate((element) => getComputedStyle(element).color))
      .not.toBe(brandColor);
    const nextColor = await brand.evaluate(
      (element) => getComputedStyle(element).color
    );
    await expect.poll(faviconColor).toContain(`color:${nextColor}`);
  });
}
