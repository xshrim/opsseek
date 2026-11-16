import favicon from '../assets/favicon.svg?raw';

export function syncFavicon() {
  const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
  if (!link) return;
  const logo = document.querySelector<SVGElement>('.opsseek-logo');
  const color = logo
    ? getComputedStyle(logo).color
    : getComputedStyle(document.documentElement)
        .getPropertyValue('--theme-logo-fg')
        .trim();
  if (!color) return;
  const svg = favicon
    .replace(/<style>[\s\S]*?<\/style>/, '')
    .replace('<svg ', `<svg style="color:${color}" `);
  const href = `data:image/svg+xml,${encodeURIComponent(svg)}`;
  if (link.getAttribute('href') !== href) link.href = href;
}
