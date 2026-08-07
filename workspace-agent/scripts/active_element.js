/**
 * @fileoverview Evaluates and returns metadata for the active focused element.
 */

/**
 * Evaluates active focused element and returns its selector and label info.
 * @return {?Object} Active element metadata, or null if not input-like.
 */
window.__shadowopsGetActiveElement = function() {
  const el = document.activeElement;
  if (!el) {
    return null;
  }

  const tag = (el.tagName || '').toLowerCase();
  const isInputLike =
      tag === 'input' || tag === 'textarea' || el.isContentEditable;
  if (!isInputLike) {
    return null;
  }

  const text = (el.innerText || '').trim();
  const value = typeof el.value === 'string' ? el.value : text;
  const aria = el.getAttribute('aria-label') || '';
  const placeholder = el.getAttribute('placeholder') || '';
  const name = el.getAttribute('name') || '';
  const type = el.getAttribute('type') || '';
  const id = el.id || '';

  const labelText = (el.labels && el.labels.length)
      ? Array.from(el.labels)
            .map((n) => (n.innerText || n.textContent || '').trim())
            .join(' ')
            .replace(/\s+/g, ' ')
            .trim()
      : '';

  const testAttrs = ['data-testid', 'data-test-id', 'data-qa', 'data-cy'];

  /**
   * Returns filtered stable CSS classes for a node.
   * @param {!Element} node
   * @return {!Array<string>}
   */
  function stableClasses(node) {
    const raw = (node.className && typeof node.className === 'string')
        ? node.className.split(/\s+/).filter(Boolean)
        : [];
    return raw
        .filter(
            (c) => !/^_?ng(host|content)-/.test(c) &&
                !/^ng-/.test(c) &&
                !/^cdk-/.test(c) &&
                !/^[a-f0-9]{8,}$/i.test(c))
        .slice(0, 3);
  }

  let css = '';
  if (id && !/^\d+$/.test(id)) {
    css = `#${id}`;
  } else {
    for (const a of testAttrs) {
      const v = el.getAttribute(a) || '';
      if (v) {
        css = `[${a}="${v.replace(/"/g, '')}"]`;
        break;
      }
    }
    if (!css) {
      let part = tag || 'div';
      const classes = stableClasses(el);
      if (classes.length) {
        part += '.' + classes.join('.');
      }
      if (name) {
        part += `[name="${name.replace(/"/g, '')}"]`;
      }
      if (type) {
        part += `[type="${type.replace(/"/g, '')}"]`;
      }
      css = part;
    }
  }

  const displayText =
      aria || labelText || placeholder || name || (id ? id : tag);

  const locators = [];
  for (const a of testAttrs) {
    const v = el.getAttribute(a) || '';
    if (v) {
      locators.push({
        kind: 'css',
        value: `[${a}="${v.replace(/"/g, '')}"]`,
      });
    }
  }

  if ((aria || labelText) && (tag === 'input' || tag === 'textarea')) {
    locators.push({
      kind: 'label',
      value: aria || labelText,
    });
  }
  if (aria) {
    locators.push({
      kind: 'label',
      value: aria,
    });
  }
  if (labelText) {
    locators.push({
      kind: 'label',
      value: labelText,
    });
  }
  if (placeholder) {
    locators.push({
      kind: 'label',
      value: placeholder,
    });
  }
  if (type) {
    locators.push({
      kind: 'css',
      value: `input[type="${type}"]`,
    });
  }
  if (css) {
    locators.push({
      kind: 'css',
      value: css,
    });
  }

  return {
    displayText,
    placeholder,
    label: aria || labelText,
    css,
    value,
    locatorCandidates: locators,
  };
};
