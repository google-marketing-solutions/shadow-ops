/**
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * @fileoverview DOM event recorder script for user interaction capture.
 */
(() => {
  if (window.__shadowopsDomRecorderInstalled) {
    return;
  }
  window.__shadowopsDomRecorderInstalled = true;

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
   * @param {?string} v
   * @return {string}
   */
  function cleanText(v) {
    return (v || '').replace(/\s+/g, ' ').trim();
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
        .slice(0, 4);
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
   * Resolves accessible name for an element.
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
   * Returns text lines of element.
   * @param {?Element} el
   * @return {!Array<string>}
   */
  function textLinesOf(el) {
    return String(el?.innerText || el?.textContent || '')
        .split(/\n+/)
        .map((v) => cleanText(v))
        .filter(Boolean)
        .slice(0, 6);
  }

  /**
   * Tests if string contains volatile/timestamp text.
   * @param {?string} v
   * @return {boolean}
   */
  function looksVolatileText(v) {
    const s = cleanText(v).toLowerCase();
    if (!s) {
      return false;
    }
    if (/\b\d{1,2}:\d{2}\s?(am|pm)\b/.test(s)) {
      return true;
    }
    if (/\b(today|yesterday|just now|minutes? ago|hours? ago)\b/.test(s)) {
      return true;
    }
    if (/\b(jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)\b/.test(s)) {
      return true;
    }
    return false;
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
   * Checks if an element contains meaningful interaction targets.
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
    if (looksVolatileText(txt)) {
      score -= 14;
    }
    if (tag === 'div' || tag === 'span') {
      score -= 10;
    }
    score -= Math.min(20, depth * 2);
    return score;
  }

  /**
   * Selects best interactive target from event path.
   * @param {!Array<!Node>} path
   * @param {?EventTarget} eventTarget
   * @return {?Element}
   */
  function chooseTargetFromPath(path, eventTarget) {
    const els = (Array.isArray(path) ? path : [])
        .filter((n) => n && n.nodeType === 1);
    let best = null;
    let bestScore = -9999;
    els.slice(0, 12).forEach((el, depth) => {
      if (!isMeaningful(el)) {
        return;
      }
      const score = scoreActionTarget(el, depth);
      if (score > bestScore) {
        best = el;
        bestScore = score;
      }
    });
    if (best) {
      return best;
    }
    return (eventTarget && eventTarget.nodeType === 1)
        ? eventTarget
        : (els[0] || null);
  }

  /**
   * Computes sibling signature for grouping context.
   * @param {?Element} el
   * @return {string}
   */
  function siblingSignature(el) {
    if (!el || el.nodeType !== 1) {
      return '';
    }
    const tag = (el.tagName || '').toLowerCase();
    const role = attr(el, 'role') || implicitRole(el) || '';
    const classes = stableClasses(el).slice(0, 2).join('.');
    const hrefFlag = attr(el, 'href') ? 'href' : '';
    return `${tag}|${role}|${classes}|${hrefFlag}`;
  }

  /**
   * Computes nth-of-type index.
   * @param {?Element} el
   * @return {number}
   */
  function nthOfType(el) {
    if (!el || el.nodeType !== 1 || !el.parentElement) {
      return 1;
    }
    const tag = (el.tagName || '').toLowerCase();
    let idx = 1;
    let sib = el;
    while ((sib = sib.previousElementSibling)) {
      if ((sib.tagName || '').toLowerCase() === tag) {
        idx += 1;
      }
    }
    return idx;
  }

  /**
   * Computes 1-based child index.
   * @param {?Element} el
   * @return {number}
   */
  function childIndex(el) {
    if (!el || el.nodeType !== 1 || !el.parentElement) {
      return 1;
    }
    let idx = 1;
    let sib = el;
    while ((sib = sib.previousElementSibling)) {
      idx += 1;
    }
    return idx;
  }

  /**
   * Builds descriptor for single hierarchy node.
   * @param {?Element} el
   * @param {number} depth
   * @return {?Object}
   */
  function hierarchyNodeDescriptor(el, depth) {
    if (!el || el.nodeType !== 1) {
      return null;
    }
    const tag = (el.tagName || '').toLowerCase();
    const role = attr(el, 'role') || implicitRole(el) || undefined;
    const lines = textLinesOf(el);
    const primary = (lines[0] || '').slice(0, 100);
    const secondary = (lines[1] || '').slice(0, 100);
    const aria = attr(el, 'aria-label') || undefined;
    const name = attr(el, 'name') || undefined;
    const href = attr(el, 'href') || undefined;
    const id = isStableId(el.id || '') ? (el.id || undefined) : undefined;
    const classes = stableClasses(el);
    const testId = testAttrs
        .map((a) => ({a, v: attr(el, a)}))
        .find((x) => Boolean(x.v));
    const segment = (() => {
      if (id) {
        return `#${id}`;
      }
      let s = tag || 'div';
      if (classes.length) {
        s += '.' + classes.slice(0, 2).join('.');
      }
      if (name) {
        s += `[name="${String(name).replace(/"/g, '')}"]`;
      }
      if (role) {
        s += `[role="${String(role).replace(/"/g, '')}"]`;
      }
      return s;
    })();
    const label = accessibleName(el);
    return {
      depth,
      tag: tag || undefined,
      role,
      segment,
      id,
      classes: classes.length ? classes : undefined,
      testIdAttr: testId ? testId.a : undefined,
      testId: testId ? testId.v : undefined,
      label: label ? label.slice(0, 120) : undefined,
      text: primary || undefined,
      secondaryText: secondary || undefined,
      volatileText: (looksVolatileText(primary) || looksVolatileText(label)) ||
          undefined,
      href,
      ariaLabel: aria,
      name,
      interactive: isInteractiveElement(el) || undefined,
      nthOfType: nthOfType(el),
      childIndex: childIndex(el),
    };
  }

  /**
   * Builds hierarchy path descriptors up to 8 levels.
   * @param {!Element} target
   * @return {!Array<!Object>}
   */
  function buildHierarchy(target) {
    const out = [];
    let node = target;
    let depth = 0;
    while (node && node.nodeType === 1 && node !== document.body && depth < 8) {
      const d = hierarchyNodeDescriptor(node, depth);
      if (d) {
        out.push(d);
      }
      node = node.parentElement;
      depth += 1;
    }
    if (document.body && out.length < 9) {
      const bodyDesc = hierarchyNodeDescriptor(document.body, depth);
      if (bodyDesc) {
        out.push(bodyDesc);
      }
    }
    return out;
  }

  /**
   * Finds nearby heading text.
   * @param {?Element} node
   * @return {string}
   */
  function findNearbyHeadingText(node) {
    let cursor = node;
    for (let depth = 0; cursor && cursor !== document.body && depth < 4; depth += 1) {
      let prev = cursor.previousElementSibling;
      while (prev) {
        const h = (prev.matches && prev.matches('h1,h2,h3,h4,h5,h6,[role="heading"]'))
            ? prev
            : (prev.querySelector ? prev.querySelector('h1,h2,h3,h4,h5,h6,[role="heading"]') : null);
        const t = textOf(h);
        if (t) {
          return t.slice(0, 120);
        }
        prev = prev.previousElementSibling;
      }
      const aria = attr(cursor, 'aria-label');
      if (aria) {
        return cleanText(aria).slice(0, 120);
      }
      cursor = cursor.parentElement;
    }
    return '';
  }

  /**
   * Resolves list or grid collection context.
   * @param {!Element} el
   * @return {?Object}
   */
  function findCollectionContext(el) {
    let node = el;
    for (let depth = 0; node && node !== document.body && depth < 6; depth += 1) {
      const parent = node.parentElement;
      if (!parent) {
        break;
      }
      const children = Array.from(parent.children || [])
          .filter((c) => c && c.nodeType === 1);
      if (children.length >= 2) {
        const sig = siblingSignature(node);
        const matches = children.filter(
            (c) => siblingSignature(c) === sig && (isInteractiveElement(c) || Boolean(textOf(c))));
        if (sig && matches.length >= 2) {
          const idx = matches.indexOf(node);
          if (idx >= 0) {
            return {
              itemIndex: idx + 1,
              itemCount: matches.length,
              itemTag: (node.tagName || '').toLowerCase(),
              itemRole: attr(node, 'role') || implicitRole(node) || undefined,
              parentTag: (parent.tagName || '').toLowerCase(),
              parentRole: attr(parent, 'role') || implicitRole(parent) || undefined,
              headingText: findNearbyHeadingText(parent) || undefined,
              itemSignature: sig,
            };
          }
        }
      }
      node = parent;
    }
    return null;
  }

  /**
   * Constructs detailed target descriptor for DOM events.
   * @param {!Event} event
   * @return {?Object}
   */
  function buildTargetDescriptor(event) {
    const path = typeof event.composedPath === 'function' ? event.composedPath() : [];
    const target = chooseTargetFromPath(path, event.target);
    const start = (event.target && event.target.nodeType === 1) ? event.target : target;
    if (!target) {
      return null;
    }
    const tag = (target.tagName || '').toLowerCase() || 'element';
    const role = attr(target, 'role') || implicitRole(target);
    const rawLines = textLinesOf(target);
    const primaryLine = (rawLines[0] || '').slice(0, 160);
    const secondaryLine = (rawLines[1] || '').slice(0, 160);
    const label = accessibleName(target).slice(0, 160);
    const text = textOf(target).slice(0, 160);
    const name = attr(target, 'name');
    const id = target.id || '';
    const href = attr(target, 'href');
    const value = typeof target.value === 'string' ? String(target.value).slice(0, 200) : '';
    const collection = findCollectionContext(target);
    const hierarchy = buildHierarchy(target);
    const volatileText = looksVolatileText(text) ||
        looksVolatileText(primaryLine) ||
        looksVolatileText(label);
    const rect = (() => {
      try {
        const r = target.getBoundingClientRect();
        return {
          x: Math.round(r.x),
          y: Math.round(r.y),
          width: Math.round(r.width),
          height: Math.round(r.height),
        };
      } catch (ignored) {
        // Ignore bounding client rect retrieval error if detached.
        return null;
      }
    })();
    const locatorCandidates = [];
    const seen = new Set();
    function push(candidate) {
      if (!candidate) {
        return;
      }
      const k = JSON.stringify(candidate);
      if (seen.has(k)) {
        return;
      }
      seen.add(k);
      locatorCandidates.push(candidate);
    }
    for (const a of testAttrs) {
      const v = attr(target, a);
      if (v) {
        push({kind: 'css', value: `[${a}="${v.replace(/"/g, '')}"]`});
      }
    }
    if (role && label && !looksVolatileText(label)) {
      push({kind: 'role', role, name: label});
    }
    if (role && primaryLine && !looksVolatileText(primaryLine)) {
      push({kind: 'role', role, name: primaryLine});
    }
    if (label && !looksVolatileText(label)) {
      push({kind: 'label', value: label});
    }
    if (primaryLine && primaryLine !== label && !looksVolatileText(primaryLine)) {
      push({kind: 'text', value: primaryLine});
    }
    if (name) {
      push({kind: 'css', value: `${tag}[name="${name.replace(/"/g, '')}"]`});
    }
    if (isStableId(id)) {
      push({kind: 'css', value: `#${id}`});
    }
    const css = cssFor(target);
    if (css) {
      push({kind: 'css', value: css});
    }
    const pathCss = cssPath(target);
    if (pathCss && pathCss !== css) {
      push({kind: 'css', value: pathCss});
    }
    if (collection && (collection.itemIndex || 0) > 0) {
      push({
        kind: 'collection_index',
        index: collection.itemIndex,
        itemCount: collection.itemCount,
        itemRole: collection.itemRole,
        itemTag: collection.itemTag,
        headingText: collection.headingText,
      });
    }
    if (text && text.length <= 80 && !looksVolatileText(text)) {
      push({kind: 'text', value: text});
    }

    const displayText = (() => {
      if (primaryLine && !looksVolatileText(primaryLine)) {
        return primaryLine;
      }
      if (label && !looksVolatileText(label)) {
        return label;
      }
      if (primaryLine) {
        return primaryLine;
      }
      if (label) {
        return label;
      }
      return text || name || id || tag || 'element';
    })();
    return {
      displayText,
      tag,
      role: role || undefined,
      text: text || undefined,
      titleText: primaryLine || undefined,
      secondaryText: secondaryLine || undefined,
      label: label || undefined,
      href: href || undefined,
      valuePreview: value || undefined,
      volatileText: volatileText || undefined,
      collection: collection || undefined,
      hierarchy: hierarchy.length ? hierarchy : undefined,
      point: {
        x: Math.round(event.clientX || 0),
        y: Math.round(event.clientY || 0),
      },
      rect: rect || undefined,
      locatorCandidates,
      debug: {
        startTag: start && start.tagName
            ? String(start.tagName).toLowerCase()
            : undefined,
        startText: start ? textOf(start).slice(0, 120) : undefined,
        chosenTargetTag: tag,
        pathTags: (Array.isArray(path) ? path : [])
            .filter((n) => n && n.nodeType === 1)
            .slice(0, 8)
            .map((n) => String(n.tagName || '').toLowerCase()),
      },
    };
  }

  /**
   * Dispatches capture payload to host window hook.
   * @param {!Object} payload
   */
  function send(payload) {
    try {
      if (typeof window.__shadowopsCaptureEvent === 'function') {
        window.__shadowopsCaptureEvent(payload);
      }
    } catch (ignored) {
      // Ignore errors when dispatching capture event.
    }
  }

  document.addEventListener('click', (event) => {
    if (!event.isTrusted) {
      return;
    }
    send({
      kind: 'dom.click',
      button: event.button,
      clickCount: event.detail || 1,
      modifiers: {
        alt: Boolean(event.altKey),
        ctrl: Boolean(event.ctrlKey),
        meta: Boolean(event.metaKey),
        shift: Boolean(event.shiftKey),
      },
      target: buildTargetDescriptor(event),
      pageUrl: location.href,
      ts: Date.now(),
    });
  }, true);

  document.addEventListener('change', (event) => {
    if (!event.isTrusted) {
      return;
    }
    const t = event.target;
    if (!(t instanceof HTMLElement)) {
      return;
    }
    const tag = (t.tagName || '').toLowerCase();
    if (!['input', 'textarea', 'select'].includes(tag)) {
      return;
    }
    const descriptor = buildTargetDescriptor(event);
    send({
      kind: 'dom.change',
      target: descriptor,
      value: typeof t.value === 'string' ? t.value : undefined,
      checked: typeof t.checked === 'boolean' ? t.checked : undefined,
      pageUrl: location.href,
      ts: Date.now(),
    });
  }, true);
})();
