export function installDiagnosisDocumentListeners(options: {
  copyCode: (encoded: string) => void | Promise<boolean>;
}) {
  const handleMarkdownClick = (event: MouseEvent) => {
    const target = event.target as HTMLElement | null;
    const button = target?.closest<HTMLButtonElement>('[data-code-copy]');
    if (!button) return;
    event.preventDefault();
    const result = options.copyCode(button.dataset.codeCopy ?? '');
    if (result instanceof Promise) {
      void result.then((copied) => {
        if (!copied) return;
        button.dataset.copied = 'true';
        button.dataset.tooltip = '已复制';
        window.setTimeout(() => {
          if (!button.isConnected) return;
          delete button.dataset.copied;
          button.dataset.tooltip = '复制代码';
        }, 1500);
      });
    }
  };
  document.addEventListener('click', handleMarkdownClick);
  return () => {
    document.removeEventListener('click', handleMarkdownClick);
  };
}

export function copyDiagnosisText(
  content: string,
  onCopied: () => void,
  onError: (message: string) => void
) {
  const clipboard = navigator.clipboard;
  if (!clipboard?.writeText) {
    onError('当前浏览器不允许直接复制，请手动选择文本。');
    return;
  }
  void clipboard.writeText(content).then(
    onCopied,
    () => onError('当前浏览器不允许直接复制，请手动选择文本。')
  );
}

export function startDiagnosisPanelResize(
  side: 'history' | 'context',
  event: PointerEvent,
  getWidths: () => { history: number; context: number },
  setWidth: (side: 'history' | 'context', value: number) => void
) {
  if (window.innerWidth <= 850) return;
  const startX = event.clientX;
  const startWidth =
    side === 'history' ? getWidths().history : getWidths().context;
  const move = (moveEvent: PointerEvent) => {
    const delta = moveEvent.clientX - startX;
    const next =
      side === 'history'
        ? Math.max(180, Math.min(360, startWidth + delta))
        : Math.max(220, Math.min(380, startWidth - delta));
    setWidth(side, next);
  };
  const stop = () => {
    window.removeEventListener('pointermove', move);
    window.removeEventListener('pointerup', stop);
  };
  window.addEventListener('pointermove', move);
  window.addEventListener('pointerup', stop, { once: true });
}
