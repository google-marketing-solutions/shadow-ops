/**
 * @fileoverview Resolves DOM element target descriptor and locators from (x, y) coordinates.
 */

/**
 * Resolves element from coordinates and returns rich target descriptor.
 * @param {{x: number, y: number}} point Coordinates object.
 * @return {?Object} Target element descriptor or null.
 */
window.__shadowopsGetElementFromPoint = function({x, y}) {
  const start = document.elementFromPoint(x, y);
  if (!start) {
    return null;
  }

  const testAttrs = ['data-testid', 'data-test-id', 'data-qa', 'data-cy'];
  const interactiveRoles = new Set([
    'button',
    'link',
    'checkbox',
    'radio',
    'switch',
    'tab',
    'menuitem',
    'option',
  ]);

  /**
   * Cleans and normalizes text whitespace.
   * @param {?string} value
   * @return {string}
   */
  function cleanText(value) {
    return (value || '').replace(/\s+/g, ' ').trim();
  }

  /**
   * Safely retrieves attribute value from an element.
   * @param {?Element} el
   * @param {string} name
   * @return {string}
   */
  function attr(el, name) {
    return (el && el.getAttribute) ? (el.getAttribute(name) || '') : '';
  }

  /**
   * Extracts text from an element.
   * @param {?Element} el
   * @return {string}
   */
  function textOf(el) {
    return cleanText(el?.innerText || el?.textContent || '');
  }

  /**
   * Tests if an ID is stable (non-dynamic).
   * @param {string} id
   * @return {boolean}
   */
  function isStableId(id) {
    return Boolean(id && !/^\d+$/.test(id) && !/^mat-input-\d+$/.test(id));
  }

  /**
   * Returns filtered stable CSS classes for an element.
   * @param {?Element} el
   * @return {!Array<string>}
   */
  function stableClasses(el) {
    const raw = (el?.className && typeof el.className === 'string')
        ? el.className.split(/\s+/).filter(Boolean)
        : [];
    return raw
        .filter((c) => {
          if (c.length > 48) {
            return false;
          }
          if (/^_?ng(host|content)-/.test(c)) {
            return false;
          }
          if (/^ng-/.test(c)) {
            return false;
          }
          if (/^cdk-/.test(c)) {
            return false;
          }
          if (/^css-[a-z0-9_-]+$/i.test(c)) {
            return false;
          }
          if (/^[a-f0-9]{8,}$/i.test(c)) {
            return false;
          }
          return true;
        })
        .slice(0, 3);
  }

  /**
   * Computes implicit ARIA role for native HTML elements.
   * @param {?Element} el
   * @return {string}
   */
  function implicitRole(el) {
    const tag = (el?.tagName || '').toLowerCase();
    const type = attr(el, 'type');
    if (tag === 'button') {
      return 'button';
    }
    if (tag === 'a' && attr(el, 'href')) {
      return 'link';
    }
    if (tag === 'input' && ['button', 'submit', 'reset'].includes(type)) {
      return 'button';
    }
    if (tag === 'input' && type === 'checkbox') {
      return 'checkbox';
    }
    if (tag === 'input' && type === 'radio') {
      return 'radio';
    }
    return '';
  }

  /**
   * Resolves accessible name for element.
   * @param {?Element} el
   * @return {string}
   */
  function accessibleName(el) {
    if (!el) {
      return '';
    }
    const aria = attr(el, 'aria-label');
    if (aria) {
      return cleanText(aria);
    }
    const labelledBy = attr(el, 'aria-labelledby');
    if (labelledBy) {
      const t = labelledBy
          .split(/\s+/)
          .map((id) => document.getElementById(id))
          .filter(Boolean)
          .map((n) => textOf(n))
          .join(' ');
      if (t) {
        return cleanText(t);
      }
    }
    if (el.labels && el.labels.length) {
      const t = Array.from(el.labels)
          .map((n) => textOf(n))
          .join(' ');
      if (t) {
        return cleanText(t);
      }
    }
    for (const key of ['title', 'alt', 'placeholder', 'name']) {
      const v = attr(el, key);
      if (v) {
        return cleanText(v);
      }
    }
    return textOf(el);
  }

  /**
   * Generates a CSS selector for an element.
   * @param {?Element} el
   * @return {string}
   */
  function cssFor(el) {
    if (!el) {
      return '';
    }
    const id = el.id || '';
    if (isStableId(id)) {
      return `#${id}`;
    }
    for (const a of testAttrs) {
      const v = attr(el, a);
      if (v) {
        return `[${a}="${v.replace(/"/g, '')}"]`;
      }
    }
    const tag = (el.tagName || '').toLowerCase() || 'div';
    const name = attr(el, 'name');
    const type = attr(el, 'type');
    const classes = stableClasses(el);
    let selector = tag;
    if (classes.length) {
      selector += '.' + classes.join('.');
    }
    if (name) {
      selector += `[name="${name.replace(/"/g, '')}"]`;
    }
    if (type && ['input', 'button'].includes(tag)) {
      selector += `[type="${type.replace(/"/g, '')}"]`;
    }
    return selector;
  }

  /**
   * Generates a hierarchical CSS path for an element.
   * @param {?Element} el
   * @return {string}
   */
  function cssPath(el) {
    const parts = [];
    let node = el;
    let depth = 0;
    while (node && node.nodeType === 1 && node !== document.body && depth < 6) {
      const tag = (node.tagName || '').toLowerCase();
      if (!tag) {
        break;
      }
      if (isStableId(node.id || '')) {
        parts.unshift(`#${node.id}`);
        break;
      }
      const classes = stableClasses(node);
      let part = tag;
      if (classes.length) {
        part += '.' + classes.slice(0, 2).join('.');
      } else {
        let nth = 1;
        let sib = node;
        while ((sib = sib.previousElementSibling)) {
          if ((sib.tagName || '').toLowerCase() === tag) {
            nth += 1;
          }
        }
        part += `:nth-of-type(${nth})`;
      }
      parts.unshift(part);
      node = node.parentElement;
      depth += 1;
    }
    return parts.join(' > ');
  }

  /**
   * Checks if an element is interactive.
   * @param {?Element} el
   * @return {boolean}
   */
  function isInteractiveElement(el) {
    if (!el || el.nodeType !== 1) {
      return false;
    }
    const tag = (el.tagName || '').toLowerCase();
    if (['button', 'a', 'input', 'textarea', 'select', 'option', 'label', 'summary'].includes(tag)) {
      return true;
    }
    const role = attr(el, 'role');
    if (role && interactiveRoles.has(role)) {
      return true;
    }
    if (el.tabIndex >= 0) {
      return true;
    }
    if (attr(el, 'onclick')) {
      return true;
    }
    return false;
  }

  /**
   * Checks if an element contains meaningful text or interaction targets.
   * @param {?Element} el
   * @return {boolean}
   */
  function isMeaningful(el) {
    if (!el || el.nodeType !== 1) {
      return false;
    }
    if (isInteractiveElement(el)) {
      return true;
    }
    for (const a of testAttrs) {
      if (attr(el, a)) {
        return true;
      }
    }
    const txt = textOf(el);
    return Boolean(txt && txt.length <= 80);
  }

  /**
   * Scores action target suitability.
   * @param {?Element} el
   * @param {number} depth
   * @return {number}
   */
  function scoreActionTarget(el, depth) {
    if (!el || el.nodeType !== 1) {
      return -9999;
    }
    const tag = (el.tagName || '').toLowerCase();
    const role = attr(el, 'role') || implicitRole(el);
    const txt = textOf(el);
    let score = 0;
    if (tag === 'button') {
      score += 140;
    }
    if (tag === 'a' && attr(el, 'href')) {
      score += 135;
    }
    if (['input', 'select', 'textarea'].includes(tag)) {
      score += 125;
    }
    if (role && interactiveRoles.has(role)) {
      score += 110;
    }
    if (attr(el, 'href')) {
      score += 35;
    }
    if (el.tabIndex >= 0) {
      score += 20;
    }
    if (attr(el, 'onclick')) {
      score += 20;
    }
    for (const a of testAttrs) {
      if (attr(el, a)) {
        score += 30;
      }
    }
    if (txt) {
      score += Math.min(28, 6 + Math.floor(txt.length / 10));
    }
    if (tag === 'div' || tag === 'span') {
      score -= 10;
    }
    score -= Math.min(20, depth * 2);
    return score;
  }

  let best = null;
  let bestScore = -9999;
  let node = start;
  let depth = 0;
  while (node && node.nodeType === 1 && node !== document.body && depth < 8) {
    if (isMeaningful(node)) {
      const score = scoreActionTarget(node, depth);
      if (score > bestScore) {
        best = node;
        bestScore = score;
      }
    }
    node = node.parentElement;
    depth += 1;
  }

  const target = best || start;
  const tag = (target.tagName || '').toLowerCase() || 'element';
  const role = attr(target, 'role') || implicitRole(target);
  const label = accessibleName(target).slice(0, 160);
  const text = textOf(target).slice(0, 160);
  const name = attr(target, 'name');
  const id = target.id || '';
  const locatorCandidates = [];

  for (const a of testAttrs) {
    const v = attr(target, a);
    if (v) {
      locatorCandidates.push({
        kind: 'css',
        value: `[${a}="${v.replace(/"/g, '')}"]`,
      });
    }
  }

  if (role && label) {
    locatorCandidates.push({
      kind: 'role',
      role,
      name: label,
    });
  }
  if (label) {
    locatorCandidates.push({
      kind: 'label',
      value: label,
    });
  }
  if (name) {
    locatorCandidates.push({
      kind: 'css',
      value: `${tag}[name="${name.replace(/"/g, '')}"]`,
    });
  }
  if (isStableId(id)) {
    locatorCandidates.push({
      kind: 'css',
      value: `#${id}`,
    });
  }

  const css = cssFor(target);
  if (css) {
    locatorCandidates.push({
      kind: 'css',
      value: css,
    });
  }

  const pathCss = cssPath(target);
  if (pathCss && pathCss !== css) {
    locatorCandidates.push({
      kind: 'css',
      value: pathCss,
    });
  }

  if (text && text.length <= 80) {
    locatorCandidates.push({
      kind: 'text',
      value: text,
    });
  }

  const displayText = label || text || name || id || tag || 'element';
  return {
    displayText,
    point: {
      x: Math.round(x),
      y: Math.round(y),
    },
    locatorCandidates,
  };
};
