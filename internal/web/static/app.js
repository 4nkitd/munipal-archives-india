'use strict';

const $ = (selector, root = document) => root.querySelector(selector);
const all = (selector, root = document) => [...root.querySelectorAll(selector)];
const el = (tag, text, cls) => {
  const node = document.createElement(tag);
  if (text != null) node.textContent = text;
  if (cls) node.className = cls;
  return node;
};
const recordPath = (city, id) => `/doc/${encodeURIComponent(city)}/${encodeURIComponent(id)}`;
const link = (text, href, cls) => {
  const node = el('a', text, cls);
  node.href = href;
  return node;
};
const docLink = (title, city, id) => {
  const node = link(title, recordPath(city, id));
  node.dataset.openDoc = '';
  return node;
};
const percent = value => `${Math.round((value || 0) * 100)}%`;
const label = value => (value || '').replace(/[-_]/g, ' ').replace(/^./, c => c.toUpperCase());
const official = value => {
  try {
    const url = new URL(value);
    return /^https?:$/.test(url.protocol) ? url.href : '';
  } catch { return ''; }
};
const json = async (path, signal) => {
  const res = await fetch(path, {signal, headers:{Accept:'application/json'}});
  if (!res.ok) throw Error(`Archive returned ${res.status}`);
  return res.json();
};

all('[data-js]').forEach(node => { node.hidden = false; });
all('[data-nojs-only]').forEach(node => { node.hidden = true; });
const normalize = text => (text || '').toLowerCase().normalize('NFC').replace(/[^\p{L}\p{M}\p{N}]+/gu, ' ').trim();
const tokens = text => normalize(text).split(' ').filter(Boolean);
const formatDate = iso => {
  if (!iso) return 'Undated';
  const day = iso.split('T')[0];
  const partial = /^\d{4}-\d{2}$/.test(day);
  if (!partial && !/^\d{4}-\d{2}-\d{2}$/.test(day)) return iso;
  const full = partial ? `${day}-01` : day;
  const date = new Date(`${full}T00:00:00Z`);
  if (isNaN(date) || date.toISOString().slice(0, 10) !== full) return iso;
  const month = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][date.getUTCMonth()];
  return `${partial ? '' : `${day.slice(8)} `}${month} ${day.slice(0, 4)}`;
};

function initTheme(button) {
  const root = document.documentElement, system = matchMedia('(prefers-color-scheme: dark)');
  try {
    const saved = localStorage.getItem('nagar.theme');
    if (['dark','light'].includes(saved)) root.dataset.theme = saved;
    else delete root.dataset.theme;
  } catch {}
  const current = () => root.dataset.theme || (system.matches ? 'dark' : 'light');
  const sync = () => {
    const theme = current(), action = `Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`;
    button.dataset.themeState = theme;
    button.setAttribute('aria-label', action);
    button.title = action;
    const text = $('[data-theme-label]', button);
    if (text) text.textContent = label(theme);
  };
  button.onclick = () => {
    root.dataset.theme = current() === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem('nagar.theme', root.dataset.theme); } catch {}
    sync();
  };
  system.addEventListener('change', sync);
  sync();
}
all('[data-theme-toggle]').forEach(initTheme);

function wrapTables(root) {
  all('table', root).forEach(table => {
    if (table.parentElement.classList.contains('table-scroll')) return;
    const box = el('div', null, 'table-scroll');
    box.tabIndex = 0;
    box.setAttribute('role', 'region');
    box.setAttribute('aria-label', table.caption?.textContent || 'Record table');
    table.replaceWith(box);
    box.append(table);
  });
}
all('.prose').forEach(wrapTables);

function metadata(fields, cls, optional = false) {
  const list = el('dl', null, cls);
  for (let [name, value] of fields) {
    if (Array.isArray(value)) value = value.join(', ');
    if (optional && !value) continue;
    const pair = el('div');
    pair.append(el('dt', name), el('dd', value || 'Not recorded'));
    list.append(pair);
  }
  return list;
}
function disclosure(name, cls, body) {
  const box = el('details', null, cls);
  box.append(el('summary', name), body);
  return box;
}

function readerContents(prose, headings) {
  const targets = new Map(all('[id]', prose).map(node => [node.id, node]));
  for (const [id, node] of targets) node.id = `reader-fragment-${id}`;
  const jump = hash => {
    let id;
    try { id = decodeURIComponent(hash.slice(1)); } catch { return; }
    const node = targets.get(id);
    if (node) {
      node.tabIndex = -1;
      node.focus({preventScroll:true});
      node.scrollIntoView({block:'start'});
    }
  };
  const bind = anchor => {
    const hash = anchor.hash;
    anchor.onclick = event => { event.preventDefault(); jump(hash); };
  };
  all('a[href^="#"]', prose).forEach(bind);
  const list = el('ol');
  for (const h of headings || []) {
    if (!targets.has(h.id)) continue;
    const li = el('li'), anchor = link(h.text, `#${encodeURIComponent(h.id)}`);
    bind(anchor);
    li.append(anchor);
    list.append(li);
  }
  return {list, jump};
}
function relatedRecords(items) {
  const grid = el('ul', null, 'related-grid');
  for (const item of items || []) {
    const li = el('li'), a = docLink(null, item.municipality, item.id);
    a.className = 'related-card';
    a.append(el('strong', item.title), el('span', label(item.category)), el('span', formatDate(item.date)));
    if (Number.isFinite(item.similarity)) a.append(el('span', `${percent(item.similarity)} similarity`, 'signal-pill'));
    li.append(a);
    grid.append(li);
  }
  if (!grid.childElementCount) grid.append(el('li', 'No related records indexed.'));
  return grid;
}
const drawer = $('#reader');
const drawerBody = $('#drawer-body');
const actions = $('[data-drawer-actions]');
let opener, reading;
async function openRecord(path, trigger, hash = '') {
  if (!drawer?.showModal || !actions) { location.assign(path + hash); return; }
  reading?.abort();
  const task = reading = new AbortController();
  if (!drawer.open) opener = trigger;
  const replacing = drawer.open;
  const title = el('h1', 'Loading record…');
  title.id = 'drawer-title';
  title.tabIndex = -1;
  const full = link('Full page', path + hash, 'button');
  actions.replaceChildren(full);
  drawerBody.replaceChildren(title);
  if (!drawer.open) drawer.showModal();
  ($('.drawer-scroll', drawer) || drawer).scrollTop = 0;
  $('[data-close]').focus();
  try {
    const d = await json(path.replace(/^\/doc\//, '/api/documents/'), task.signal);
    if (task.signal.aborted) return;
    title.textContent = d.title;
    const copy = el('button', 'Copy text', 'btn-secondary');
    copy.dataset.copy = '';
    actions.replaceChildren(el('code', d.id, 'doc-id'), copy, full);
    const source = official(d.source_url);
    if (source) actions.append(link('Official source', source, 'button'));
    const core = metadata([['Collection',label(d.category)], ['Document type',d.doc_type],
      ['Date',formatDate(d.date)], ['Department',d.department]], 'meta-card');
    const more = metadata([['Wards',d.wards], ['Villages',d.villages], ['Sectors',d.sectors],
      ['Source portal',d.source_portal], ['Original filename',d.original_filename],
      ['Verified date',d.verified_at && formatDate(d.verified_at)]], 'meta-card', true);
    const prose = el('div', null, 'prose');
    prose.dataset.prose = '';
    // The document endpoint renders Markdown with Goldmark's safe renderer.
    prose.innerHTML = d.html_content || '';
    wrapTables(prose);
    const contents = readerContents(prose, d.headings);
    drawerBody.replaceChildren(title, core);
    if (more.childElementCount) drawerBody.append(disclosure('Full provenance & geography', 'provenance-more', more));
    if (contents.list.childElementCount) drawerBody.append(disclosure('Contents', 'contents', contents.list));
    drawerBody.append(prose, el('h2', 'Related records'), relatedRecords(d.related));
    if (replacing) title.focus({preventScroll:true});
    if (hash) contents.jump(hash);
  } catch {
    if (!task.signal.aborted) {
      title.textContent = 'Could not load the record';
      drawerBody.append(link('Open the full record', path));
    }
  }
}
drawer.addEventListener('close', () => {
  reading?.abort();
  drawerBody.replaceChildren();
  actions?.replaceChildren();
  if (opener?.isConnected) opener.focus();
});
$('[data-close]').onclick = () => drawer.close();
drawer.onclick = event => {
  if (event.target !== drawer) return;
  const r = drawer.getBoundingClientRect();
  if (event.clientX < r.left || event.clientX > r.right || event.clientY < r.top || event.clientY > r.bottom) drawer.close();
};
const feedback = new WeakMap();
document.addEventListener('click', async event => {
  const anchor = event.target.closest('a[data-open-doc], #drawer-body [data-prose] a[href^="/doc/"]');
  if (anchor && anchor.origin === location.origin && event.button === 0 && !(event.ctrlKey || event.metaKey || event.shiftKey || event.altKey)) {
    event.preventDefault();
    openRecord(anchor.pathname, anchor, anchor.hash);
  }
  const copy = event.target.closest('[data-copy]');
  if (copy) {
    const prose = $('[data-prose]', copy.closest('dialog, article') || document);
    if (!prose) return;
    clearTimeout(feedback.get(copy));
    copy.setAttribute('aria-live', 'polite');
    try {
      await navigator.clipboard.writeText(prose.innerText);
      copy.textContent = 'Copied';
    } catch {
      const range = new Range();
      range.selectNodeContents(prose);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      copy.textContent = 'Selected for copying';
    }
    feedback.set(copy, setTimeout(() => { copy.textContent = 'Copy text'; }, 2000));
  }
});

function syncCityLinks() {
  const current = new URL(location.href);
  const path = current.pathname.startsWith('/doc/') ? '/index' : current.pathname;
  const keys = path === '/search' ? ['q','mode','sort','category','doc_type'] :
    path === '/index' ? ['category','sort','q','doc_type'] : [];
  all('[data-city-link]').forEach(a => {
    const url = new URL(path, location.origin);
    url.searchParams.set('municipality', a.dataset.citySlug || new URL(a.href).searchParams.get('municipality'));
    for (const key of keys) if (current.searchParams.has(key)) url.searchParams.set(key, current.searchParams.get(key));
    a.href = url.href;
  });
}
function initCities(city) {
  const trigger = $('.city-trigger', city), field = $('[data-city-filter]', city);
  const items = all('[data-city-item]', city).filter(row => !row.matches('[data-city-slug="all"], .city-pinned'));
  const apply = () => {
    const terms = tokens(field.value);
    for (const row of items) row.hidden = !terms.every(term => normalize(row.dataset.cityTerms).includes(term));
    all('.city-group', city).forEach(group => { group.hidden = !group.querySelector('[data-city-item]:not([hidden])'); });
    $('[data-city-empty]', city).hidden = items.some(row => !row.hidden);
  };
  city.addEventListener('toggle', () => {
    trigger.setAttribute('aria-expanded', String(city.open));
    if (city.open) { field.value = ''; apply(); syncCityLinks(); field.focus(); }
  });
  field.oninput = apply;
  field.onkeydown = e => {
    if (e.key !== 'Enter' || e.isComposing) return;
    e.preventDefault();
    const matches = items.filter(row => !row.hidden);
    if (matches.length === 1) (matches[0].closest('a') || $('a', matches[0]))?.click();
  };
  document.addEventListener('click', e => {
    if (city.open && !city.contains(e.target)) { city.open = false; if (city.contains(document.activeElement)) trigger.focus(); }
  });
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape' && city.open && !drawer.open) { city.open = false; trigger.focus(); }
  });
  syncCityLinks();
}
all('[data-city-switcher]').forEach(initCities);
all('[data-shortcut]').forEach(node => {
  node.textContent = /Mac|iPhone|iPad/.test(navigator.userAgentData?.platform || navigator.platform) ? '⌘K' : 'Ctrl K';
});
document.addEventListener('keydown', e => {
  if (drawer.open) return;
  if (e.target.closest('input, textarea, select, [contenteditable]:not([contenteditable=false])')) return;
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    const input = $('[data-focus]');
    if (input) { e.preventDefault(); input.focus(); }
  }
});

const query = $('#archive-query');
if (query) {
  const form = query.form, list = $('#suggestions');
  const clear = $('[data-clear-query]', form);
  if (clear) {
    const sync = () => { clear.hidden = !query.value; };
    clear.onclick = () => {
      query.value = '';
      query.dispatchEvent(new Event('input', {bubbles:true}));
      query.focus();
    };
    query.addEventListener('input', sync);
    sync();
  }
  const syncPrompts = () => {
    all('[data-prompt-link]').forEach(a => {
      const url = new URL(a.href);
      for (const key of ['mode','sort']) if (form.elements[key]?.value) url.searchParams.set(key, form.elements[key].value);
      a.href = url.href;
    });
  };
  form.addEventListener('change', syncPrompts);
  syncPrompts();
  let timer, pending, items = [], active = -1;
  const hide = () => {
    clearTimeout(timer);
    pending?.abort();
    list.hidden = true;
    query.setAttribute('aria-expanded', 'false');
    query.removeAttribute('aria-activedescendant');
    active = -1;
  };
  const choose = index => {
    const item = items[index];
    if (!item) return;
    hide();
    const city = item.municipality || form.elements.municipality.value;
    if (item.doc_id && city && city !== 'all') openRecord(recordPath(city, item.doc_id), query);
    else {
      query.value = item.query || item.label;
      form.requestSubmit();
    }
  };
  query.addEventListener('input', () => {
    hide();
    const q = query.value.trim();
    if (!q) return;
    timer = setTimeout(async () => {
      const task = pending = new AbortController();
      try {
        const result = await json(`/api/suggest?${new URLSearchParams({q, municipality:form.elements.municipality.value})}`, task.signal);
        if (task.signal.aborted || document.activeElement !== query) return;
        items = result.suggestions || [];
        list.replaceChildren(...items.map((item, i) => {
          const option = el('div');
          const kind = ['ward','village','sector','officer','prompt'].includes(item.type) ? label(item.type) : 'Record';
          option.append(el('span', kind, 'suggest-kind'), el('span', item.label));
          option.id = `suggestion-${i}`;
          option.setAttribute('role', 'option');
          option.setAttribute('aria-selected', 'false');
          if (item.sublabel) option.append(el('small', item.sublabel));
          option.onmousedown = e => e.preventDefault();
          option.onclick = () => choose(i);
          return option;
        }));
        list.hidden = !items.length;
        query.setAttribute('aria-expanded', String(!list.hidden));
      } catch { if (!task.signal.aborted) hide(); }
    }, 140);
  });
  query.onblur = hide;
  query.addEventListener('keydown', e => {
    if (list.hidden || e.isComposing) return;
    if (e.key === 'Escape') { e.preventDefault(); hide(); return; }
    if (e.key === 'Enter' && active !== -1) { e.preventDefault(); choose(active); return; }
    if (!['ArrowUp','ArrowDown'].includes(e.key)) return;
    e.preventDefault();
    active = active < 0 ? (e.key === 'ArrowDown' ? 0 : items.length - 1) :
      (active + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
    [...list.children].forEach((node, i) => node.setAttribute('aria-selected', String(i === active)));
    const option = list.children[active];
    option.scrollIntoView({block:'nearest'});
    query.setAttribute('aria-activedescendant', option.id);
  });
  form.addEventListener('submit', hide);
  if ($('.dock')) {
    const preview = el('section', null, 'brief');
    preview.hidden = true;
    preview.setAttribute('aria-label', 'Search preview');
    $('.stream').prepend(preview);
    let clock, request;
    const refresh = () => {
      clearTimeout(clock);
      request?.abort();
      preview.hidden = true;
      if (query.value.trim().length < 3 || query.value === query.defaultValue) return;
      clock = setTimeout(async () => {
        const job = request = new AbortController();
        const params = new URLSearchParams(new FormData(form));
        try {
          const result = await json(`/api/ask?${params}`, job.signal);
          if (job.signal.aborted || !result.brief?.headline) return;
          const brief = result.brief;
          preview.replaceChildren(el('span', 'Search preview', 'small-title'), el('h2', brief.headline));
          const byIdx = new Map((brief.citations || []).map(c => [String(c.index), c]));
          const citeNode = c => {
            const text = `[${c.index}]`;
            const source = c.source_url?.trim() || '';
            const href = /^\/(?![\\/])/.test(source) && !/[\\\x00-\x1F\x7F]/.test(source) ? source : official(source);
            let node;
            if (c.doc_id && c.municipality) {
              node = docLink(text, c.municipality, c.doc_id);
            } else if (href) {
              node = link(text, href);
            } else {
              return el('span', text, 'cite');
            }
            node.className = 'cite';
            node.title = c.title;
            node.setAttribute('aria-label', `Source ${c.index}: ${c.title}`);
            return node;
          };
          for (const paragraph of brief.answer_paragraphs || []) {
            const p = el('p', null, 'answer');
            for (const part of paragraph.split(/(\[\d+\])/)) {
              const m = part.match(/^\[(\d+)\]$/);
              if (m && byIdx.has(m[1])) p.append(citeNode(byIdx.get(m[1])));
              else p.append(document.createTextNode(part));
            }
            preview.append(p);
          }
          const sources = el('div', null, 'buttons');
          for (const c of brief.citations || []) sources.append(citeNode(c));
          sources.append(link(`View ${result.total} matches`, `/search?${params}`));
          preview.append(sources);
          preview.hidden = false;
        } catch { /* Submitting the form still opens the server-rendered results. */ }
      }, 550);
    };
    query.addEventListener('input', refresh);
    form.addEventListener('change', refresh);
  }
}

for (const rail of all('[data-rail-drawer]')) {
  const wide = matchMedia('(min-width: 920px)');
  const sync = () => {
    const p = new URLSearchParams(location.search);
    rail.open = wide.matches || ['category','doc_type','ward'].some(k => !!p.get(k)) ||
      (p.has('sort') && p.get('sort') !== 'relevance');
  };
  sync();
  wide.addEventListener('change', sync);
}
const matchesWords = (words, terms) => terms.every(term =>
  words.some(word => /^\d+$/.test(term) ? word === term : word.includes(term)));
function initWards(list) {
  const field = $('[data-filter]'), tabs = all('[data-tab]', list);
  const panels = all('[data-tab-panel]').map(panel => ({
    panel, name:panel.dataset.tabPanel,
    rows:all('[data-row]', panel).map(node => ({node, words:tokens(node.dataset.terms),
      wards:tokens(`${node.dataset.wardNum || ''} ${node.dataset.wards || ''}`), sectors:tokens(node.dataset.sectors)}))
  }));
  for (const {panel, name} of panels) {
    panel.setAttribute('role', 'tabpanel');
    panel.setAttribute('aria-labelledby', `tab-${name}`);
    panel.tabIndex = 0;
  }
  let active = 'wards';
  const filter = () => {
    const q = normalize(field.value), terms = tokens(q);
    const ward = q.match(/^(?:ward\s*)?(\d+)$/)?.[1], sector = q.match(/^sector\s*(\d+)$/)?.[1];
    for (const {panel, name, rows} of panels) {
      panel.hidden = name !== active;
      let count = 0;
      for (const row of rows) {
        row.node.hidden = !(ward ? row.wards.includes(ward) : sector ? row.sectors.includes(sector) : matchesWords(row.words, terms));
        if (!row.node.hidden) count++;
      }
      $(`[data-count-${name}]`).textContent = count;
      $(`[data-empty-${name}]`).hidden = count > 0;
      if (!panel.hidden) $('[data-filter-status]').textContent = `${count} of ${rows.length} ${name}`;
    }
    for (const button of all('[data-clear-filter]')) button.hidden = !field.value;
  };
  const activate = tab => {
    active = tab.dataset.tab;
    for (const item of tabs) {
      item.setAttribute('aria-selected', String(item === tab));
      item.tabIndex = item === tab ? 0 : -1;
    }
    filter();
  };
  tabs.forEach((tab, i) => {
    tab.onclick = e => { e.preventDefault(); activate(tab); };
    tab.onkeydown = e => {
      const pos = {ArrowLeft:(i + tabs.length - 1) % tabs.length, ArrowRight:(i + 1) % tabs.length, Home:0, End:tabs.length-1}[e.key];
      if (pos === undefined) return;
      e.preventDefault();
      activate(tabs[pos]);
      tabs[pos].focus();
    };
  });
  for (const button of all('[data-clear-filter]')) button.onclick = () => { field.value = ''; filter(); field.focus(); };
  field.addEventListener('input', filter);
  activate(tabs[0]);
}
all('[data-tab-list]').forEach(initWards);

function initIndex(sort) {
  const field = $('[data-filter]'), category = $('[data-collection]');
  const clear = $('[data-clear-filter]'), indicator = $('[data-sort-indicator]');
  const body = $('[data-index-rows]');
  const other = $('[data-index-other]');
  const nodes = [...all('tr[data-row]', body), ...(other ? all('tr[data-row]', other.content) : [])];
  const rows = nodes.map(node => {
    const d = node.dataset;
    const date = /^\d{4}-\d{2}-\d{2}$/.test(d.date) ? d.date : '';
    return {node, words:d.terms.split(' '), date, precision:date ? 0 : d.undated === 'true' ? 2 : 1,
      title:d.title, muni:d.muni, id:d.id};
  });
  const cmp = (a,b) => a === b ? 0 : a < b ? -1 : 1;
  const reorder = () => {
    if (indicator) indicator.textContent = sort.selectedOptions[0].textContent;
    rows.sort((a,b) => {
      if (sort.value !== 'title') {
        const ordered = cmp(a.precision,b.precision) || cmp(a.date,b.date) * (sort.value === 'date_asc' ? 1 : -1);
        if (ordered) return ordered;
      }
      return cmp(a.title,b.title) || cmp(a.muni,b.muni) || cmp(a.id,b.id);
    });
    const fragment = document.createDocumentFragment();
    rows.forEach(({node}) => fragment.append(node));
    body.append(fragment);
  };
  const filter = () => {
    const terms = tokens(field.value);
    let count = 0;
    for (const {node, words} of rows) {
      node.hidden = !!(category.value && category.value !== node.dataset.category) || !matchesWords(words, terms);
      if (!node.hidden) count++;
    }
    $('[data-filter-status]').textContent = `${count} ${count === 1 ? 'record' : 'records'}`;
    $('[data-empty-index]').hidden = count > 0;
    if (clear) clear.hidden = !field.value;
  };
  const save = method => {
    const url = new URL(location.href);
    for (const [key, value] of [['category',category.value], ['sort',sort.value], ['q',field.value]]) {
      if (value) url.searchParams.set(key, value);
      else url.searchParams.delete(key);
    }
    if (url.href !== location.href) history[method](null, '', url);
    syncCityLinks();
  };
  const restore = () => {
    const p = new URLSearchParams(location.search);
    sort.value = ['title','date_asc'].includes(p.get('sort')) ? p.get('sort') : 'date_desc';
    category.value = p.get('category') || '';
    field.value = p.get('q') || '';
    reorder();
    filter();
    syncCityLinks();
  };
  category.onchange = () => { filter(); save('pushState'); };
  sort.onchange = () => { reorder(); save('pushState'); };
  field.oninput = () => { filter(); save('replaceState'); };
  if (clear) clear.onclick = () => { field.value = ''; field.oninput(); field.focus(); };
  window.addEventListener('popstate', restore);
  restore();
}
all('[data-index-sort]').forEach(initIndex);

function highlightSnippets() {
  if (!query) return;
  const stop = new Set('a an are as at and by can do does for from how i in is it me my of on or the to what when where which who with not near'.split(' '));
  const terms = new Set(tokens(query.value).filter(term => !stop.has(term)));
  if (!terms.size) return;
  all('[data-snippet]').forEach(node => {
    const text = node.textContent;
    node.replaceChildren(...text.split(/([\p{L}\p{M}\p{N}]+)/u).map((part, i) =>
      i % 2 && terms.has(normalize(part)) ? el('mark', part) : document.createTextNode(part)));
  });
}
highlightSnippets();
