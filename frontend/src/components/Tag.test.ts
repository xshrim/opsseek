import { afterEach, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import Tag from './Tag.svelte';

let component: ReturnType<typeof mount>;
let resize: ResizeObserverCallback | undefined;

afterEach(async () => {
  if (component) await unmount(component);
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  resize = undefined;
});

it('only exposes a tooltip when ellipsis text is truncated', () => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(callback: ResizeObserverCallback) {
        resize = callback;
      }
      observe() {}
      disconnect() {}
      unobserve() {}
    }
  );
  component = mount(Tag, {
    target: document.body,
    props: { value: '完整标签文本', maxWidth: '50px' }
  });
  flushSync();

  const tag = document.querySelector<HTMLSpanElement>('.tag')!;
  expect(tag.dataset.tooltip).toBeUndefined();

  Object.defineProperties(tag, {
    clientWidth: { configurable: true, value: 50 },
    scrollWidth: { configurable: true, value: 120 }
  });
  resize?.([], {} as ResizeObserver);
  flushSync();

  expect(tag.dataset.tooltip).toBe('完整标签文本');
});
