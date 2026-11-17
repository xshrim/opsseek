import hljs from 'highlight.js/lib/common';
import { Marked, type RendererObject } from 'marked';

export type DiagnosisMarkdownOptions = {
  renderMermaid?: boolean;
};

function escapeHTML(text: string) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Keep the provider text byte-for-byte intact. Markdown is parsed only; the
 * client must not infer headings, insert line breaks, or close delimiters.
 * This same identity function is used for live and persisted messages.
 */
export function normalizeDiagnosisMarkdown(value: string) {
  return String(value ?? '');
}

function renderCode(
  source: string,
  language = '',
  renderMermaid = false
): string {
  const normalized = language
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_+-]/g, '');
  if (normalized === 'mermaid' && renderMermaid) {
    const encoded = escapeHTML(encodeURIComponent(source));
    return `<div class="diagnosis-mermaid" data-mermaid-source="${encoded}" aria-label="Mermaid 图"><div class="diagnosis-mermaid-fallback">${renderCode(source, '')}</div></div>`;
  }
  let rendered = escapeHTML(source);
  if (normalized && normalized !== 'mermaid') {
    try {
      rendered = hljs.highlight(source, {
        language: normalized,
        ignoreIllegals: true
      }).value;
    } catch {
      // Unknown language: keep a safe plain-text block.
    }
  }
  const encoded = encodeURIComponent(source);
  return `<div class="diagnosis-code-wrap"><button class="diagnosis-code-copy" type="button" data-code-copy="${encoded}" aria-label="复制代码" data-tooltip="复制代码"><span class="diagnosis-code-copy-icon" aria-hidden="true">⧉</span><span class="diagnosis-code-copied-icon" aria-hidden="true">✓</span></button><pre class="diagnosis-code-block"><code${normalized ? ` class="language-${normalized} hljs"` : ' class="hljs"'}>${rendered}</code></pre></div>`;
}

export function renderDiagnosisMarkdown(
  value: string,
  options: DiagnosisMarkdownOptions = {}
) {
  const renderer: RendererObject = {
    code: ({ text, lang }) => {
      const source = String(text ?? '');
      return source.trim()
        ? renderCode(source, lang ?? '', options.renderMermaid === true)
        : '';
    },
    html: ({ text }) => escapeHTML(text),
    link({ href, title, tokens }) {
      const safeHref = /^(?:https?:|mailto:)/i.test(href) ? href : '#';
      const label = this.parser.parseInline(tokens);
      const titleAttr = title ? ` data-tooltip="${escapeHTML(title)}"` : '';
      return `<a href="${escapeHTML(safeHref)}"${titleAttr} target="_blank" rel="noreferrer">${label}</a>`;
    }
  };
  const parser = new Marked({ gfm: true, breaks: true, renderer });
  try {
    return parser.parse(normalizeDiagnosisMarkdown(value), {
      async: false
    }) as string;
  } catch {
    return `<p>${escapeHTML(String(value ?? ''))}</p>`;
  }
}
