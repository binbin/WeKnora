/**
 * Minimal Markdown → HTML for IE9 (subset). Escapes HTML first.
 */
(function (global) {
  'use strict';

  function escapeHtml(text) {
    return String(text == null ? '' : text)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function renderInline(text) {
    var html = escapeHtml(text);
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    html = html.replace(
      /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>'
    );
    html = html.replace(
      /(^|[\s(])(https?:\/\/[^\s<]+)/g,
      '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>'
    );
    return html;
  }

  function renderMarkdown(source) {
    var text = String(source == null ? '' : source).replace(/\r\n/g, '\n');
    var lines = text.split('\n');
    var out = [];
    var inCode = false;
    var codeBuf = [];
    var index;
    for (index = 0; index < lines.length; index += 1) {
      var line = lines[index];
      if (line.indexOf('```') === 0) {
        if (inCode) {
          out.push('<pre><code>' + escapeHtml(codeBuf.join('\n')) + '</code></pre>');
          codeBuf = [];
          inCode = false;
        } else {
          inCode = true;
        }
        continue;
      }
      if (inCode) {
        codeBuf.push(line);
        continue;
      }
      if (/^###\s+/.test(line)) {
        out.push('<h3>' + renderInline(line.replace(/^###\s+/, '')) + '</h3>');
      } else if (/^##\s+/.test(line)) {
        out.push('<h2>' + renderInline(line.replace(/^##\s+/, '')) + '</h2>');
      } else if (/^#\s+/.test(line)) {
        out.push('<h1>' + renderInline(line.replace(/^#\s+/, '')) + '</h1>');
      } else if (/^[-*]\s+/.test(line)) {
        out.push('<div>• ' + renderInline(line.replace(/^[-*]\s+/, '')) + '</div>');
      } else if (line === '') {
        out.push('<br/>');
      } else {
        out.push('<div>' + renderInline(line) + '</div>');
      }
    }
    if (inCode) {
      out.push('<pre><code>' + escapeHtml(codeBuf.join('\n')) + '</code></pre>');
    }
    return out.join('');
  }

  function renderPlain(source) {
    return escapeHtml(source).replace(/\n/g, '<br/>');
  }

  global.LegacyEmbedMarkdown = {
    render: renderMarkdown,
    renderPlain: renderPlain,
    escapeHtml: escapeHtml,
  };
})(typeof window !== 'undefined' ? window : this);
