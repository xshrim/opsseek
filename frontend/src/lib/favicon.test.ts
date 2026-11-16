import { afterEach, describe, expect, it } from 'vitest';
import { syncFavicon } from './favicon';

afterEach(() => {
  document.head.innerHTML = '';
  document.body.innerHTML = '';
  document.documentElement.style.removeProperty('--theme-logo-fg');
});

describe('favicon theme synchronization', () => {
  it('uses the actual logo color and updates when it changes', () => {
    document.head.innerHTML = '<link rel="icon" href="/favicon.svg">';
    document.body.innerHTML =
      '<svg class="opsseek-logo" style="color:rgb(36, 93, 80)"></svg>';
    syncFavicon();
    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')!;
    const svg = () =>
      decodeURIComponent(
        link.getAttribute('href')!.split(',').slice(1).join(',')
      );
    expect(svg()).toContain('color:rgb(36, 93, 80)');
    expect(svg()).not.toContain('prefers-color-scheme');
    document.querySelector<SVGElement>('svg')!.style.color =
      'rgb(159, 228, 207)';
    syncFavicon();
    expect(svg()).toContain('color:rgb(159, 228, 207)');
  });

  it('uses the theme token before the logo mounts and tolerates a missing icon link', () => {
    expect(() => syncFavicon()).not.toThrow();
    document.head.innerHTML = '<link rel="icon" href="/favicon.svg">';
    document.documentElement.style.setProperty('--theme-logo-fg', '#e77b50');
    syncFavicon();
    expect(
      decodeURIComponent(document.querySelector('link')!.getAttribute('href')!)
    ).toContain('color:#e77b50');
  });
});
