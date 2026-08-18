"use strict";
// Dependency-free SPA for the thAImaturgy server. Talks to the REST API under
// /api and tails the session timeline over SSE. Phase 1 (issue #66) brings the
// session experience to parity with the desktop GUI: an adventure browser, a
// detail pane with inline images, context actions, the Oracle↔Virtual-DM mode
// toggle with Begin/Rest, a dice roller and a multiline prompt.

const $ = (sel) => document.querySelector(sel);
const el = (tag, cls, text) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
};

const tokenInput = $("#token");
tokenInput.value = sessionStorage.getItem("thaim_token") || "";
tokenInput.addEventListener("change", () => sessionStorage.setItem("thaim_token", tokenInput.value.trim()));
const token = () => tokenInput.value.trim();

function status(msg, isErr) {
  const s = $("#status");
  s.textContent = msg;
  s.classList.toggle("err", !!isErr);
  s.classList.add("show");
  clearTimeout(status._t);
  status._t = setTimeout(() => s.classList.remove("show"), isErr ? 6000 : 2500);
}

async function api(method, path, body) {
  const headers = {};
  if (token()) headers["Authorization"] = "Bearer " + token();
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const resp = await fetch("/api" + path, {
    method, headers, body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await resp.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { /* non-JSON */ }
  if (!resp.ok) {
    const msg = (data && data.error) || resp.statusText || ("HTTP " + resp.status);
    throw new Error(msg);
  }
  return data;
}

// Module images are behind the same auth as the API, and an <img> tag can't send
// the bearer header, so we fetch them as authenticated blobs and cache the object
// URLs for the life of the open session.
const assetCache = new Map();
async function assetURL(advId, relPath) {
  const key = advId + "|" + relPath;
  if (assetCache.has(key)) return assetCache.get(key);
  const headers = {};
  if (token()) headers["Authorization"] = "Bearer " + token();
  const resp = await fetch("/api/adventures/" + encodeURIComponent(advId) +
    "/asset?path=" + encodeURIComponent(relPath), { headers });
  if (!resp.ok) throw new Error("image " + resp.status);
  const url = URL.createObjectURL(await resp.blob());
  assetCache.set(key, url);
  return url;
}
function clearAssets() {
  for (const url of assetCache.values()) URL.revokeObjectURL(url);
  assetCache.clear();
}

// --- View switching ------------------------------------------------------

function show(view) {
  leaveSession(); // tear down any live session stream before switching
  document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
  $("#view-" + view).classList.remove("hidden");
  document.querySelectorAll(".nav").forEach((b) => b.classList.toggle("active", b.dataset.view === view));
  if (view === "library") loadLibrary();
  if (view === "roster") loadRoster();
  if (view === "settings") loadSettings();
}
document.querySelectorAll(".nav").forEach((b) => b.addEventListener("click", () => show(b.dataset.view)));

// --- Library -------------------------------------------------------------

async function loadLibrary() {
  const advs = $("#adventures"); advs.innerHTML = "";
  const sess = $("#sessions"); sess.innerHTML = "";
  try {
    for (const a of (await api("GET", "/adventures")) || []) {
      const c = el("div", "card");
      c.append(el("span", "title", a.title || a.id));
      c.append(el("span", "muted", a.system || ""));
      c.append(el("span", "spacer"));
      const play = el("button", null, "New session");
      play.onclick = async () => {
        try { const r = await api("POST", "/sessions", { adventure_id: a.id }); openSession(r.name); }
        catch (e) { status(e.message, true); }
      };
      const edit = el("button", "ghost", "Edit");
      edit.onclick = () => openEditor(a.id);
      const del = el("button", "ghost", "Delete");
      del.onclick = async () => {
        if (!confirm("Delete adventure “" + (a.title || a.id) + "” and its assets?")) return;
        try { await api("DELETE", "/adventures/" + encodeURIComponent(a.id)); loadLibrary(); status("Adventure deleted."); }
        catch (e) { status(e.message, true); }
      };
      c.append(play, edit, del);
      advs.append(c);
    }
    for (const s of (await api("GET", "/sessions")) || []) {
      const c = el("div", "card");
      c.append(el("span", "title", s.name));
      const sub = [s.adventure_title || s.adventure_id || "", fmtTime(s.modified_at)].filter(Boolean).join(" · ");
      c.append(el("span", "muted", sub));
      c.append(el("span", "spacer"));
      const open = el("button", null, "Open");
      open.onclick = () => openSession(s.name);
      const ren = el("button", "ghost", "Rename");
      ren.onclick = async () => {
        const nn = prompt("Rename session", s.name);
        if (!nn || nn.trim() === s.name) return;
        try { await api("POST", "/sessions/" + encodeURIComponent(s.name) + "/rename", { new_name: nn.trim() }); loadLibrary(); }
        catch (e) { status(e.message, true); }
      };
      const del = el("button", "ghost", "Delete");
      del.onclick = async () => {
        if (!confirm("Delete session " + s.name + "?")) return;
        try { await api("DELETE", "/sessions/" + encodeURIComponent(s.name)); loadLibrary(); }
        catch (e) { status(e.message, true); }
      };
      c.append(open, ren, del);
      sess.append(c);
    }
  } catch (e) { status(e.message, true); }
}

// Import a module (.tar.gz) via multipart upload. FormData sets its own
// Content-Type boundary, so we only add the bearer header when a token is set.
$("#import-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const f = $("#import-file").files[0];
  if (!f) { status("Choose a .tar.gz module first.", true); return; }
  const fd = new FormData();
  fd.append("module", f);
  // X-Thaim-CSRF makes this a non-simple request, forcing a CORS preflight that a
  // cross-origin page can't satisfy — CSRF protection for the safelisted upload.
  const headers = { "X-Thaim-CSRF": "1" };
  if (token()) headers["Authorization"] = "Bearer " + token();
  try {
    const resp = await fetch("/api/adventures/import", { method: "POST", headers, body: fd });
    const data = await resp.json().catch(() => null);
    if (!resp.ok) throw new Error((data && data.error) || ("HTTP " + resp.status));
    $("#import-file").value = "";
    status("Imported “" + (data.title || data.id) + "”.");
    loadLibrary();
  } catch (err) { status(err.message, true); }
});

// AI import (PDF or images) — starts an async job and polls its progress.
$("#ai-kind").addEventListener("change", () => {
  const kind = $("#ai-kind").value;
  const f = $("#ai-file");
  if (kind === "images") { f.setAttribute("multiple", "multiple"); f.setAttribute("accept", "image/*"); }
  else { f.removeAttribute("multiple"); f.setAttribute("accept", "application/pdf,.pdf"); }
});
$("#aiimport-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const kind = $("#ai-kind").value;
  const files = $("#ai-file").files;
  if (!files.length) { status("Choose " + (kind === "images" ? "one or more images" : "a PDF") + " first.", true); return; }
  const fd = new FormData();
  fd.append("kind", kind);
  fd.append("title", $("#ai-title").value.trim());
  if (kind === "images") { for (const f of files) fd.append("files", f); }
  else { fd.append("file", files[0]); }
  const headers = { "X-Thaim-CSRF": "1" };
  if (token()) headers["Authorization"] = "Bearer " + token();
  $("#ai-progress").textContent = "Starting AI import…";
  try {
    const resp = await fetch("/api/import-jobs", { method: "POST", headers, body: fd });
    const data = await resp.json().catch(() => null);
    if (!resp.ok) throw new Error((data && data.error) || ("HTTP " + resp.status));
    $("#ai-file").value = "";
    pollImportJob(data.id);
  } catch (err) { $("#ai-progress").textContent = ""; status(err.message, true); }
});

async function pollImportJob(id, fails) {
  fails = fails || 0;
  try {
    const j = await api("GET", "/import-jobs/" + encodeURIComponent(id));
    if (j.status === "running") {
      $("#ai-progress").textContent = "AI import: " + (j.stage || "working…");
      setTimeout(() => pollImportJob(id, 0), 2500);
    } else if (j.status === "done") {
      $("#ai-progress").textContent = "";
      status("Imported “" + (j.adventure_title || j.adventure_id) + "”.");
      loadLibrary();
    } else {
      $("#ai-progress").textContent = "";
      status("AI import failed: " + (j.error || "unknown error"), true);
    }
  } catch (e) {
    // A transient poll error shouldn't strand a running import: retry with bounded
    // backoff, surfacing the job id if it finally gives up.
    if (fails < 5) {
      $("#ai-progress").textContent = "AI import: reconnecting…";
      setTimeout(() => pollImportJob(id, fails + 1), Math.min(2500 * (fails + 1), 15000));
    } else {
      $("#ai-progress").textContent = "";
      status("Lost contact with import job " + id + " (" + e.message + "). It may still finish; reload later.", true);
    }
  }
}

function fmtTime(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d)) return "";
  return d.toLocaleString();
}

// --- Session -------------------------------------------------------------

let current = null;      // session name
let adv = null;          // loaded adventure content
let sess = null;         // last-known session state
let evtSource = null;    // EventSource
let openGen = 0;         // bumped on each open/leave; stale async work checks it
let selectedKey = null;  // selected browser node, e.g. "room:r1"
let lastZone = null;     // for auto zone-art on zone change

function appendLine(cls, text) {
  const t = $("#transcript");
  t.append(el("div", cls, text));
  t.scrollTop = t.scrollHeight;
}

function leaveSession() {
  openGen++;
  current = null; adv = null; sess = null; selectedKey = null; lastZone = null;
  if (evtSource) { evtSource.close(); evtSource = null; }
  clearAssets();
}

async function openSession(name) {
  const gen = ++openGen;
  if (evtSource) { evtSource.close(); evtSource = null; }
  clearAssets();
  try {
    const st = await api("GET", "/sessions/" + encodeURIComponent(name));
    if (gen !== openGen) return;
    let a = null;
    if (st.adventure_id) { try { a = await api("GET", "/adventures/" + encodeURIComponent(st.adventure_id)); } catch { /* keep going */ } }
    if (gen !== openGen) return;
    current = name; sess = st; adv = a;
    $("#session-name").textContent = name;
    // Show the session view.
    document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
    $("#view-session").classList.remove("hidden");
    // Transcript replay.
    $("#transcript").innerHTML = "";
    const conv = (st.conversation && st.conversation.messages) || [];
    for (const m of conv) appendLine(m.role === "assistant" ? "a" : "u", (m.role === "assistant" ? "" : "» ") + m.content);
    const t = $("#transcript"); t.scrollTop = t.scrollHeight;
    // Panels.
    renderBrowser();
    renderParty();
    renderLog();
    hosting = false;
    tgReady = false; // toggle stays disabled until refreshTelegram() confirms status
    tgBusy = false;
    applyModeUI();
    detailPlaceholder();
    subscribeEvents(name, gen);
    refreshTelegram(); // reflect a host already running for this session
  } catch (e) { if (gen === openGen) status(e.message, true); }
}

// refreshState re-fetches the session state after a mutation and re-renders the
// state-dependent panels (browser markers, party, location, mode UI).
async function refreshState() {
  if (!current) return;
  try {
    const st = await api("GET", "/sessions/" + encodeURIComponent(current));
    sess = st;
    renderBrowser();
    renderParty();
    applyModeUI();
    maybeAutoZoneArt();
  } catch (e) { status(e.message, true); }
}

// --- Adventure browser ---------------------------------------------------

function imagesFor(node) {
  // Collect module-relative image paths from a direct field plus image_ids.
  const out = [];
  if (node.image) out.push(node.image);
  if (node.map_image) out.push(node.map_image);
  const byId = {};
  for (const img of (adv && adv.images) || []) byId[img.id] = img.path;
  for (const id of node.image_ids || []) if (byId[id]) out.push(byId[id]);
  return [...new Set(out)];
}

function roomsIndex() {
  const idx = {};
  for (const z of (adv && adv.zones) || []) for (const r of z.rooms || []) idx[r.id] = { room: r, zone: z };
  return idx;
}

function renderBrowser() {
  const box = $("#browser"); box.innerHTML = "";
  if (!adv) { box.append(el("div", "muted", "No adventure content.")); return; }
  const visited = (sess && sess.visited_rooms) || {};
  const known = (sess && sess.known_npcs) || {};
  const triggered = (sess && sess.triggered_events) || {};
  const here = sess && sess.current_room;

  const group = (label, nodes) => {
    if (!nodes.length) return;
    const g = el("div", "group");
    const head = el("div", "row-label", label);
    const kids = el("div", "children");
    head.onclick = () => kids.classList.toggle("hidden");
    for (const n of nodes) kids.append(n);
    g.append(head, kids);
    box.append(g);
  };
  const node = (key, label, opts = {}) => {
    const n = el("div", "node");
    if (opts.mark) n.append(el("span", "mark", "✓"));
    if (opts.here) n.append(el("span", "here", "●"));
    n.append(document.createTextNode(label));
    if (key === selectedKey) n.classList.add("active");
    n.onclick = () => selectNode(key);
    return n;
  };

  // Adventure root.
  box.append(node("adventure:", adv.title || adv.id));

  // Zones → rooms.
  const zoneNodes = [];
  for (const z of adv.zones || []) {
    const zn = node("zone:" + z.id, z.name || z.id);
    zoneNodes.push(zn);
    const roomNodes = (z.rooms || []).map((r) =>
      node("room:" + r.id, r.name || r.id, { mark: !!visited[r.id], here: here === r.id }));
    if (roomNodes.length) {
      const kids = el("div", "children");
      for (const rn of roomNodes) kids.append(rn);
      zoneNodes.push(kids);
    }
  }
  group("Zones", zoneNodes);
  group("NPCs", (adv.npcs || []).map((n) =>
    node("npc:" + n.id, n.name || n.id, { mark: !!(known[n.id] && known[n.id].met) })));
  group("Events", (adv.events || []).map((e) =>
    node("event:" + e.id, e.name || e.id, { mark: !!triggered[e.id] })));
  group("Items", (adv.items || []).map((i) => node("item:" + i.id, i.name || i.id)));
  group("Tables", (adv.tables || []).map((tb) => node("table:" + tb.id, tb.name || tb.id)));
}

function selectNode(key) {
  selectedKey = key;
  renderBrowser();
  renderDetail(key);
}

// --- Detail pane ---------------------------------------------------------

function detailPlaceholder() {
  $("#detail-title").textContent = "Detail";
  $("#detail").innerHTML = '<p class="muted">Select something in the browser.</p>';
}

function para(parent, label, value, cls) {
  if (!value) return;
  if (label) parent.append(el("div", "label", label));
  parent.append(el("p", cls || null, value));
}
function list(parent, label, items) {
  if (!items || !items.length) return;
  parent.append(el("div", "label", label));
  const ul = el("ul");
  for (const it of items) ul.append(el("li", null, it));
  parent.append(ul);
}
function navlinks(parent, label, links) {
  if (!links || !links.length) return;
  parent.append(el("div", "label", label));
  const p = el("p");
  links.forEach((lk, i) => {
    if (i) p.append(document.createTextNode(" · "));
    const a = el("a", "navlink", lk.label);
    a.onclick = () => selectNode(lk.key);
    p.append(a);
  });
  parent.append(p);
}

async function addImages(parent, advId, paths) {
  for (const p of paths) {
    try { const url = await assetURL(advId, p); const img = el("img"); img.src = url; parent.append(img); }
    catch { /* skip broken image */ }
  }
}

function actionBar(parent, buttons) {
  const bar = el("div", "actions");
  for (const b of buttons) {
    const btn = el("button", "small", b.label);
    btn.onclick = b.onclick;
    bar.append(btn);
  }
  parent.append(bar);
}

function renderDetail(key) {
  const [kind, id] = splitKey(key);
  const box = $("#detail"); box.innerHTML = "";
  const title = $("#detail-title");
  if (!adv) { detailPlaceholder(); return; }

  if (kind === "adventure") {
    title.textContent = "Adventure";
    box.append(el("h4", null, adv.title || adv.id));
    para(box, "Summary", adv.summary);
    para(box, "Background", adv.background);
    para(box, "Introduction", adv.introduction);
    list(box, "Hooks", adv.hooks);
    return;
  }
  if (kind === "zone") {
    const z = (adv.zones || []).find((x) => x.id === id); if (!z) return;
    title.textContent = "Zone";
    box.append(el("h4", null, z.name || z.id));
    para(box, "Overview", z.overview);
    para(box, "Description", z.description);
    addImages(box, adv.id, imagesFor(z));
    return;
  }
  if (kind === "room") {
    const r = roomsIndex()[id] && roomsIndex()[id].room; if (!r) return;
    title.textContent = "Room";
    box.append(el("h4", null, r.name || r.id));
    para(box, "Read aloud", r.read_aloud, "readaloud");
    para(box, "DM notes", r.dm_notes);
    // Context actions.
    const acts = [{ label: "Move party here", onclick: () => runCommand("/goto " + r.id) }];
    box.append(el("div"));
    actionBar(box, acts);
    // Exits + linked NPCs/events as nav links.
    if (r.exits && r.exits.length) {
      navlinks(box, "Exits", r.exits.filter((e) => roomsIndex()[e.to]).map((e) =>
        ({ label: (e.direction ? e.direction + " → " : "") + (roomsIndex()[e.to].room.name || e.to), key: "room:" + e.to })));
    }
    navlinks(box, "NPCs here", (r.npc_ids || []).filter((n) => npcById(n)).map((n) => ({ label: npcById(n).name, key: "npc:" + n })));
    navlinks(box, "Events", (r.event_ids || []).filter((e) => eventById(e)).map((e) => ({ label: eventById(e).name, key: "event:" + e })));
    list(box, "Treasure", r.treasure);
    if (r.features) for (const f of r.features) para(box, f.name + (f.dc ? " (DC " + f.dc + " " + (f.skill || "") + ")" : ""), f.description);
    addImages(box, adv.id, imagesFor(r));
    return;
  }
  if (kind === "npc") {
    const n = npcById(id); if (!n) return;
    title.textContent = "NPC";
    box.append(el("h4", null, n.name || n.id));
    para(box, "Role", n.role);
    const met = sess && sess.known_npcs && sess.known_npcs[n.id] && sess.known_npcs[n.id].met;
    actionBar(box, [{ label: met ? "✓ Met" : "Mark as met", onclick: () => runCommand("/met " + n.id) }]);
    para(box, "Appearance", n.appearance);
    para(box, "Personality", n.personality);
    para(box, "Motivations", n.motivations);
    para(box, "Secrets", n.secrets);
    para(box, "Voice", n.voice);
    para(box, "Disposition", n.disposition);
    list(box, "Knowledge", n.knowledge);
    list(box, "Sample dialogue", n.sample_dialogue);
    if (n.stat_block) statBlock(box, n.stat_block);
    addImages(box, adv.id, imagesFor(n));
    return;
  }
  if (kind === "event") {
    const e = eventById(id); if (!e) return;
    title.textContent = "Event";
    box.append(el("h4", null, e.name || e.id));
    const trg = sess && sess.triggered_events && sess.triggered_events[e.id];
    actionBar(box, [{ label: trg ? "✓ Triggered" : "Mark triggered", onclick: () => runCommand("/trigger " + e.id) }]);
    para(box, "Trigger", e.trigger);
    para(box, "Description", e.description);
    para(box, "Read aloud", e.read_aloud, "readaloud");
    para(box, "DM notes", e.dm_notes);
    para(box, "Consequences", e.consequences);
    if (e.outcomes) for (const o of e.outcomes) para(box, o.condition, o.result);
    return;
  }
  if (kind === "item") {
    const it = (adv.items || []).find((x) => x.id === id); if (!it) return;
    title.textContent = "Item";
    box.append(el("h4", null, it.name || it.id));
    para(box, "Rarity", it.rarity);
    para(box, "Description", it.description);
    para(box, "Mechanics", it.mechanics);
    addImages(box, adv.id, imagesFor(it));
    return;
  }
  if (kind === "table") {
    const tb = (adv.tables || []).find((x) => x.id === id); if (!tb) return;
    title.textContent = "Table";
    box.append(el("h4", null, tb.name || tb.id));
    para(box, "Description", tb.description);
    actionBar(box, [{ label: "🎲 Roll on table", onclick: () => runCommand("/table " + tb.id) }]);
    if (tb.rows) {
      const ul = el("ul");
      for (const row of tb.rows) ul.append(el("li", null, (row.roll ? row.roll + ": " : "") + (row.cells || []).join(" · ")));
      box.append(ul);
    }
    return;
  }
}

function showImageDetail(relPath) {
  const box = $("#detail"); box.innerHTML = "";
  $("#detail-title").textContent = "Image";
  addImages(box, adv.id, [relPath]);
}

function splitKey(key) { const i = (key || "").indexOf(":"); return [key.slice(0, i), key.slice(i + 1)]; }
function npcById(id) { return (adv.npcs || []).find((n) => n.id === id); }
function eventById(id) { return (adv.events || []).find((e) => e.id === id); }

function statBlock(parent, s) {
  parent.append(el("div", "label", "Stat block"));
  const bits = [];
  if (s.ac) bits.push("AC " + s.ac);
  if (s.max_hp) bits.push("HP " + s.max_hp + (s.hit_dice ? " (" + s.hit_dice + ")" : ""));
  if (s.speed) bits.push(s.speed);
  if (s.cr) bits.push("CR " + s.cr);
  parent.append(el("p", null, bits.join(" · ")));
  const ab = s.abilities;
  if (ab) parent.append(el("p", null, ["STR", "DEX", "CON", "INT", "WIS", "CHA"]
    .map((k) => k + " " + (ab[k.toLowerCase()] ?? "—")).join("  ")));
  if (s.actions) for (const a of s.actions) para(parent, a.name, a.description + (a.to_hit ? "  (" + a.to_hit + (a.damage ? ", " + a.damage : "") + ")" : ""));
}

function maybeAutoZoneArt() {
  const zid = sess && sess.current_zone;
  if (!zid || zid === lastZone) return;
  lastZone = zid;
  const z = (adv.zones || []).find((x) => x.id === zid);
  if (!z) return;
  const imgs = imagesFor(z);
  if (imgs.length) showImageDetail(imgs[0]);
}

// --- Party (virtual-DM mode) --------------------------------------------

function renderParty() {
  const p = $("#party"); p.innerHTML = "";
  if (!hasLegacyDND5E()) {
    p.append(el("div", "muted", "Character mechanics are managed by the loaded rules package; the legacy D&D 5e sheet is unavailable."));
    return;
  }
  const party = (sess && sess.characters) || [];
  if (!party.length) { p.append(el("div", "muted", "No party. Use “Party…” to add characters.")); return; }
  for (const c of party) {
    const card = el("div", "card small");
    const line = `${c.name} — L${c.level || 1} ${c.race || ""} ${c.class || ""} (HP ${c.current_hp ?? "?"}/${c.max_hp ?? "?"}, AC ${c.ac ?? "?"})`;
    card.append(el("span", null, line));
    card.append(el("span", "spacer"));
    const edit = el("button", "ghost small", "Edit sheet");
    edit.onclick = () => openSheetEditor(c);
    card.append(edit);
    p.append(card);
  }
}

$("#party-edit").onclick = () => openPartyEditor();

// --- Session log ---------------------------------------------------------

function logEntry(entry) {
  const row = el("div", "entry");
  const ts = entry.timestamp ? new Date(entry.timestamp) : null;
  if (ts && !isNaN(ts)) row.append(el("span", "ts", ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })));
  row.append(document.createTextNode(entry.message || ""));
  return row;
}
function renderLog() {
  const box = $("#log"); box.innerHTML = "";
  const entries = (sess && sess.log && sess.log.entries) || [];
  for (const e of entries.slice(-80)) box.append(logEntry(e));
  box.scrollTop = box.scrollHeight;
}

// --- Mode / Begin / Rest -------------------------------------------------

function effectiveMode() { return (sess && sess.mode) || "assistant"; }
function hasLegacyDND5E() { return !!(sess && sess.rules_capabilities && sess.rules_capabilities.legacy_dnd5e); }

function applyModeUI() {
  const dm = effectiveMode() === "dm";
  const legacyDND5E = hasLegacyDND5E();
  $("#browser-wrap").classList.toggle("hidden", dm);
  $("#party-wrap").classList.toggle("hidden", !dm);
  $("#detail-wrap").classList.toggle("hidden", dm);
  $("#mode-toggle").textContent = dm ? "Mode: Virtual DM" : "Mode: Oracle";
  $("#transcript-title").textContent = dm ? "Virtual DM" : "Oracle";
  const started = !!(sess && sess.started);
  $("#begin").classList.toggle("hidden", !(dm && !started));
  $("#rest").classList.toggle("hidden", !(dm && legacyDND5E && started));
  $("#dice").classList.toggle("hidden", !legacyDND5E);
  $("#party-edit").classList.toggle("hidden", !legacyDND5E);
  $("#ask-input").placeholder = dm
    ? "Describe what your character does…  (Enter sends · Shift/Ctrl+Enter = newline)"
    : "Ask the oracle, or type a /command.  (Enter sends · Shift/Ctrl+Enter = newline)";
  applyTelegramUI();
}

$("#mode-toggle").onclick = () => runCommand("/mode");
$("#begin").onclick = () => runCommand("/begin");

// --- Host on Telegram (server-side bot) ---------------------------------
// The SERVER runs the Telegram bot bound to this session (token from Settings).
// While hosting, the bot is the sole driver of the game, so the server rejects
// turns from the web — the turn controls are disabled until hosting stops.
let hosting = false;
let tgReady = false; // initial Telegram status fetched for the current session?
let tgBusy = false;  // a start/stop request is in flight?

function applyTelegramUI() {
  const btn = $("#telegram");
  if (!btn) return;
  const dm = effectiveMode() === "dm";
  const legacyDND5E = hasLegacyDND5E();
  btn.classList.toggle("hidden", !((dm && legacyDND5E) || hosting));
  btn.textContent = hosting ? "Hosting — stop" : "Host: Telegram";
  btn.classList.toggle("active", hosting);
  // Not usable until the initial status is known (so a toggle can't race the
  // initial refresh and get overwritten by its late reply), nor mid-request.
  btn.disabled = (!legacyDND5E && !hosting) || !tgReady || tgBusy;
  $("#ask-input").disabled = hosting;
  $("#mode-toggle").disabled = hosting;
  $("#begin").disabled = hosting;
  $("#rest").disabled = hosting;
  if (hosting) {
    $("#ask-input").placeholder = "Hosting on Telegram — players drive the game there.";
  }
}

async function refreshTelegram() {
  if (!current) { hosting = false; tgReady = true; applyTelegramUI(); return; }
  if (!hasLegacyDND5E()) { hosting = false; tgReady = true; applyTelegramUI(); return; }
  // Capture the session generation: if the user switches sessions while this
  // request is in flight, the response is for the OLD session and must not touch
  // the (now different) current session's hosting state.
  const gen = openGen;
  try {
    const r = await api("GET", "/sessions/" + encodeURIComponent(current) + "/telegram");
    if (gen !== openGen) return;
    hosting = !!r.hosting;
  } catch { if (gen === openGen) hosting = false; }
  if (gen === openGen) { tgReady = true; applyTelegramUI(); } // enable the toggle only now
}

$("#telegram").onclick = async () => {
  if (!current || tgBusy || !tgReady) return; // wait for the initial status; one request at a time
  const gen = openGen;
  const verb = hosting ? "stop" : "start";
  tgBusy = true;
  applyTelegramUI(); // disables the button while the request is in flight
  try {
    const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/telegram/" + verb);
    if (gen !== openGen) return; // session switched under us — ignore stale reply
    hosting = !!r.hosting;
    if (hosting) {
      appendLine("log", "Hosting on Telegram" + (r.username ? " as @" + r.username : "") +
        ". Players drive the game from Telegram; turns here are paused until you stop hosting.");
    } else {
      appendLine("log", "Stopped hosting on Telegram.");
    }
  } catch (e) { if (gen === openGen) appendLine("err", "⚠ " + e.message); }
  finally {
    // Only clear busy / refresh for the CURRENT session: a stale reply (the user
    // switched away and may have started a toggle on the new session) must not
    // re-enable the new session's in-flight toggle. openSession resets tgBusy on
    // switch, so the obsolete generation leaks nothing.
    if (gen === openGen) { tgBusy = false; applyModeUI(); }
  }
};

$("#rest").onclick = () => {
  if (!hasLegacyDND5E()) { status("Rest requires the exact built-in D&D 5e rules package.", true); return; }
  const kind = prompt("Rest type: short or long?", "short");
  if (!kind) return;
  const k = kind.trim().toLowerCase();
  if (k !== "short" && k !== "long") { status("Type 'short' or 'long'.", true); return; }
  runCommand("/rest " + k);
};

// --- Novel editor -------------------------------------------------------
// A modal editor over the session's associated novelization: generate it from
// the play session, edit the prose by hand, adjust it with AI (whole text or a
// selection), save (optimistic concurrency) and export to Markdown/PDF.

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
let novelVersion = ""; // version tag of the last loaded/saved novel
let novelDirty = false;
let novelBusy = false;

function novelSetBusy(b) {
  novelBusy = b;
  ["#novel-generate", "#novel-save", "#novel-export-md", "#novel-export-pdf", "#novel-adjust", "#novel-instruction", "#novel-text"]
    .forEach((sel) => { const e = $(sel); if (e) e.disabled = b; });
}
function novelSetState(msg) { $("#novel-state").textContent = msg; }
function novelMarkDirty() { novelDirty = true; novelSetState("unsaved changes"); }
$("#novel-text").addEventListener("input", novelMarkDirty);

function closeNovelEditor() {
  if (novelBusy) return; // don't abandon an in-flight AI job
  if (novelDirty && !confirm("Discard unsaved changes to the novel?")) return;
  $("#novel-modal").classList.add("hidden");
}
$("#novel").onclick = async () => {
  if (!current) return;
  // Reset editor state up front, so a failed/slow load can't leave the previous
  // session's text + version in the modal (which Save would then write here).
  $("#novel-text").value = "";
  novelVersion = "";
  novelDirty = false;
  $("#novel-modal").classList.remove("hidden");
  novelSetBusy(true);
  try {
    const r = await api("GET", "/sessions/" + encodeURIComponent(current) + "/novel");
    $("#novel-text").value = r.text || "";
    novelVersion = r.version || "";
    novelDirty = false;
    novelSetState(r.exists ? "loaded" : "no novel yet — Generate to start");
    novelSetBusy(false); // re-enable editing only after a successful load
  } catch (e) {
    status(e.message, true);
    novelSetState("could not load — close and reopen to retry");
    // Leave the mutation controls DISABLED so nothing can be saved over the wrong
    // session, but clear the busy flag so the modal can still be closed.
    novelBusy = false;
  }
};
$("#novel-close").onclick = closeNovelEditor;
$("#novel-modal").addEventListener("click", (e) => { if (e.target === $("#novel-modal")) closeNovelEditor(); });

// awaitNovelJob polls a novel job (generate or adjust) to completion with bounded
// backoff, returning the finished job or throwing on failure / lost contact.
async function awaitNovelJob(id) {
  let fails = 0;
  for (;;) {
    let j;
    try { j = await api("GET", "/novel-jobs/" + encodeURIComponent(id)); fails = 0; }
    catch (e) {
      if (++fails > 5) throw new Error("lost contact with novel job " + id + " (" + e.message + ")");
      await sleep(Math.min(3000 * fails, 15000));
      continue;
    }
    if (j.status === "running") { await sleep(3000); continue; }
    if (j.status === "done") return j;
    throw new Error(j.error || "the AI job failed");
  }
}

async function saveNovel() {
  const r = await api("PUT", "/sessions/" + encodeURIComponent(current) + "/novel",
    { text: $("#novel-text").value, base_version: novelVersion });
  novelVersion = r.version || "";
  novelDirty = false;
}

$("#novel-generate").onclick = async () => {
  if (!current || novelBusy) return;
  if ($("#novel-text").value.trim() && !confirm("Generate a new novel from the session? This replaces the current text (unsaved edits are lost).")) return;
  novelSetBusy(true);
  novelSetState("writing the novel… this can take a minute");
  try {
    const j = await api("POST", "/sessions/" + encodeURIComponent(current) + "/novel");
    await awaitNovelJob(j.id);
    // The generate job persisted the result server-side; reload the saved text.
    const r = await api("GET", "/sessions/" + encodeURIComponent(current) + "/novel");
    $("#novel-text").value = r.text || "";
    novelVersion = r.version || "";
    novelDirty = false;
    novelSetState("generated & saved");
  } catch (e) { status(e.message, true); novelSetState("generation failed"); }
  finally { novelSetBusy(false); }
};

$("#novel-adjust-form").onsubmit = async (e) => {
  e.preventDefault();
  if (!current || novelBusy) return;
  const instruction = $("#novel-instruction").value.trim();
  if (!instruction) { status("Type an adjustment instruction first.", true); return; }
  const ta = $("#novel-text");
  if (!ta.value.trim()) { status("Generate or write a novel first.", true); return; }
  const selStart = ta.selectionStart, selEnd = ta.selectionEnd;
  const selection = selStart !== selEnd ? ta.value.substring(selStart, selEnd) : "";
  novelSetBusy(true);
  novelSetState(selection ? "revising the selection…" : "revising the whole novel…");
  try {
    const j = await api("POST", "/sessions/" + encodeURIComponent(current) + "/novel/adjust",
      { instruction, selection, text: ta.value });
    const done = await awaitNovelJob(j.id);
    const res = await api("GET", "/novel-jobs/" + encodeURIComponent(done.id) + "/result");
    if (selection) {
      ta.value = ta.value.slice(0, selStart) + (res.text || "") + ta.value.slice(selEnd);
    } else {
      ta.value = res.text || "";
    }
    novelMarkDirty();
    novelSetState("adjusted — review and Save");
    $("#novel-instruction").value = "";
  } catch (err) { status(err.message, true); novelSetState("adjustment failed"); }
  finally { novelSetBusy(false); }
};

$("#novel-save").onclick = async () => {
  if (!current || novelBusy) return;
  novelSetBusy(true);
  try { await saveNovel(); novelSetState("saved"); status("Novel saved."); }
  catch (e) { status(e.message, true); novelSetState("save failed — reload if it changed elsewhere"); }
  finally { novelSetBusy(false); }
};

async function novelExport(fmt) {
  if (!current || novelBusy) return;
  // Export streams the SAVED novel, so persist pending edits first.
  if (novelDirty) {
    if (!confirm("Save your edits before exporting? Export uses the saved novel.")) return;
    novelSetBusy(true);
    try { await saveNovel(); novelSetState("saved"); }
    catch (e) { status(e.message, true); novelSetBusy(false); return; }
    novelSetBusy(false);
  }
  downloadAuthed("/sessions/" + encodeURIComponent(current) + "/novel/download?format=" + fmt, current + "-novel." + fmt);
}
$("#novel-export-md").onclick = () => novelExport("md");
$("#novel-export-pdf").onclick = () => novelExport("pdf");

$("#back").onclick = () => show("library");
$("#save").onclick = async () => {
  try { await api("POST", "/sessions/" + encodeURIComponent(current) + "/save"); status("Saved."); }
  catch (e) { status(e.message, true); }
};
$("#close").onclick = async () => {
  const name = current;
  try { await api("POST", "/sessions/" + encodeURIComponent(name) + "/close"); show("library"); status("Closed."); }
  catch (e) { status(e.message, true); }
};

// --- Command / oracle submit --------------------------------------------

// runCommand posts a slash command and reacts to the result, including UI actions
// (oracle kickoff, inline image, save, mode change), then refreshes state so the
// browser markers and panels reflect any mutation.
async function runCommand(input) {
  if (!current) return;
  try {
    const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/command", { input });
    if (r.message) appendLine("log", r.message);
    if (r.response) appendLine("a", r.response);
    if (r.ui_action === "oracle" && r.ui_arg) { await askOracle(r.ui_arg); }
    else if (r.ui_action === "image" && r.ui_arg) { showImageDetail(r.ui_arg); }
    else if (r.ui_action === "save") { await api("POST", "/sessions/" + encodeURIComponent(current) + "/save"); status("Saved."); }
    await refreshState();
    renderLog();
    if (r.ui_action === "mode") { detailPlaceholder(); }
  } catch (e) { appendLine("err", "⚠ " + e.message); }
}

async function askOracle(input) {
  appendLine("u", "» " + input);
  appendLine("log", "…thinking…");
  try {
    const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/oracle", { input });
    if (r.error) appendLine("err", "⚠ " + r.error);
    else appendLine("a", r.answer || "(no answer)");
  } catch (e) { appendLine("err", "⚠ " + e.message); }
  await refreshState();
  renderLog();
}

async function submitInput(text) {
  text = text.trim();
  if (!text) return;
  if (hosting) { status("Hosting on Telegram — stop hosting to take a turn here.", true); return; }
  if (text.startsWith("/")) { appendLine("u", "» " + text); await runCommand(text); }
  else { await askOracle(text); }
}

$("#ask").addEventListener("submit", (e) => {
  e.preventDefault();
  const input = $("#ask-input");
  const text = input.value;
  input.value = "";
  submitInput(text);
});
// Enter submits; Shift+Enter and Ctrl/Cmd+Enter insert a newline.
$("#ask-input").addEventListener("keydown", (e) => {
  if (e.key !== "Enter") return;
  if (e.shiftKey || e.ctrlKey || e.metaKey) return; // allow newline
  e.preventDefault();
  const input = $("#ask-input");
  const text = input.value;
  input.value = "";
  submitInput(text);
});

// --- Live log stream (SSE) ----------------------------------------------

async function subscribeEvents(name, gen) {
  if (evtSource) { evtSource.close(); evtSource = null; }
  let url = "/api/sessions/" + encodeURIComponent(name) + "/events";
  if (token()) {
    try {
      const t = await api("POST", "/sse-ticket");
      if (gen !== openGen) return;
      url += "?ticket=" + encodeURIComponent(t.ticket);
    } catch (e) { if (gen === openGen) status("live updates unavailable: " + e.message, true); return; }
  }
  if (gen !== openGen) return;
  evtSource = new EventSource(url);
  evtSource.addEventListener("log", (ev) => {
    try {
      const entry = JSON.parse(ev.data);
      $("#log").append(logEntry(entry));
      $("#log").scrollTop = $("#log").scrollHeight;
    } catch { /* ignore */ }
  });
  evtSource.onerror = () => { /* EventSource auto-reconnects within the ticket window */ };
}

// --- Dice roller ---------------------------------------------------------

const STD_DICE = ["d4", "d6", "d8", "d10", "d12", "d20", "d100"];
function buildDice() {
  const q = $("#dice-quick"); q.innerHTML = "";
  for (const d of STD_DICE) {
    const b = el("button", "ghost small", d);
    b.onclick = () => rollDice("1" + d);
    q.append(b);
  }
}
async function rollDice(notation) {
  if (!current) { status("Open a session first.", true); return; }
  if (!hasLegacyDND5E()) { status("The legacy dice roller requires the exact built-in D&D 5e rules package.", true); return; }
  try {
    const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/command", { input: "/roll " + notation });
    const msg = r.message || r.response || "(no result)";
    $("#dice-result").textContent = msg;
    appendLine("log", msg);
    renderLog();
  } catch (e) { $("#dice-result").textContent = e.message; }
}
$("#dice").onclick = () => {
  if (!hasLegacyDND5E()) { status("The legacy dice roller requires the exact built-in D&D 5e rules package.", true); return; }
  buildDice(); $("#dice-result").textContent = ""; $("#dice-modal").classList.remove("hidden");
};
$("#dice-close").onclick = () => $("#dice-modal").classList.add("hidden");
$("#dice-modal").addEventListener("click", (e) => { if (e.target === $("#dice-modal")) $("#dice-modal").classList.add("hidden"); });
$("#dice-form").addEventListener("submit", (e) => {
  e.preventDefault();
  const n = $("#dice-notation").value.trim() || "1d20";
  rollDice(n);
});

// --- Roster --------------------------------------------------------------

async function loadRoster() {
  const box = $("#roster"); box.innerHTML = "";
  try {
    for (const c of (await api("GET", "/roster")) || []) {
      const card = el("div", "card");
      card.append(el("span", "title", c.name));
      card.append(el("span", "muted", `L${c.level || 1} ${c.race || ""} ${c.class || ""}`));
      card.append(el("span", "spacer"));
      const del = el("button", "ghost", "Delete");
      del.onclick = async () => {
        try { await api("DELETE", "/roster/" + encodeURIComponent(c.id)); loadRoster(); }
        catch (e) { status(e.message, true); }
      };
      card.append(del);
      box.append(card);
    }
  } catch (e) { status(e.message, true); }
}

$("#roster-new-full").onclick = () => openCharacterCreator(null, () => loadRoster(), "roster");

$("#roster-add").addEventListener("submit", async (e) => {
  e.preventDefault();
  const c = {
    name: $("#rc-name").value.trim(),
    race: $("#rc-race").value.trim() || "Human",
    class: $("#rc-class").value.trim() || "Fighter",
    level: 1, max_hp: 10, current_hp: 10, ac: 10,
  };
  if (!c.name) return;
  try { await api("POST", "/roster", c); $("#rc-name").value = ""; loadRoster(); status("Saved to roster."); }
  catch (err) { status(err.message, true); }
});

// --- Settings ------------------------------------------------------------

let cfgCache = null;
let settingsRefs = null;

function checkbox(value) { const c = el("input"); c.type = "checkbox"; c.checked = !!value; return c; }
function passwordInput() { return input("", { type: "password", autocomplete: "new-password", placeholder: "leave blank to keep current" }); }

async function loadSettings() {
  try { cfgCache = await api("GET", "/config"); } catch (e) { status(e.message, true); return; }
  const c = cfgCache || {};
  const f = $("#settings-form"); f.innerHTML = "";
  const g = (label, node) => { f.append(field(label, node)); return node; };
  const sec = (t) => f.append(el("div", "label", t));

  sec("Provider & models");
  const provider = g("Provider", selectFrom(["openai", "anthropic", "gemini", "claude-cli"], c.provider || "openai"));
  const model = g("Model", input(c.model || ""));
  const runModel = g("Run model (oracle)", input(c.run_model || ""));
  const editModel = g("Edit model (import)", input(c.edit_model || ""));

  sec("Language");
  const lang = g("UI language", selectFrom(["en", "es"], c.language || "en"));
  const importLang = g("Import language", input(c.import_language || ""));

  sec("Tunables");
  const temp = g("Temperature", numInput(c.temperature ?? 0.7)); temp.step = "0.1";
  const maxTokens = g("Max tokens", numInput(c.max_tokens || 0));
  const importMax = g("Import max output tokens", numInput(c.import_max_output_tokens || 0));
  const oracleIters = g("Oracle max tool iterations", numInput(c.oracle_max_tool_iterations || 0));
  const timeout = g("Request timeout (s)", numInput(c.request_timeout_seconds || 0));
  const autosave = g("Auto-save sessions", checkbox(c.auto_save));
  const autosaveInt = g("Auto-save interval (s)", numInput(c.auto_save_interval || 0));

  sec("Text-to-speech");
  const ttsEnabled = g("TTS enabled", checkbox(c.tts && c.tts.enabled));
  const ttsVoice = g("TTS voice", selectFrom(["alloy", "echo", "fable", "onyx", "nova", "shimmer"], (c.tts && c.tts.voice) || "alloy"));

  sec("Spoiler guard (Virtual DM)");
  const sgEnabled = g("Review DM narration for spoilers", checkbox(c.spoiler_guard && c.spoiler_guard.enabled));
  const sgModel = g("Review model (optional; blank = oracle model)", input((c.spoiler_guard && c.spoiler_guard.model) || ""));

  sec("API keys (write-only)");
  const kOpenAI = g("OpenAI API key", passwordInput());
  const kAnthropic = g("Anthropic API key", passwordInput());
  const kGemini = g("Gemini API key", passwordInput());

  sec("Telegram");
  const tgToken = g("Bot token (write-only)", passwordInput());
  const tgChat = g("Chat id", numInput(c.telegram_chat_id || 0));
  const tgUsers = g("Allowed users (one numeric id per line)", textarea((c.telegram_allowed_users || []).join("\n"), 3));

  const save = el("button", null, "Save settings"); save.type = "submit"; f.append(save);
  settingsRefs = { provider, model, runModel, editModel, lang, importLang, temp, maxTokens, importMax, oracleIters, timeout, autosave, autosaveInt, ttsEnabled, ttsVoice, sgEnabled, sgModel, kOpenAI, kAnthropic, kGemini, tgToken, tgChat, tgUsers };
}

$("#settings-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const r = settingsRefs; if (!r) return;
  const cfg = Object.assign({}, cfgCache || {});
  cfg.provider = r.provider.value;
  cfg.model = r.model.value.trim();
  cfg.run_model = r.runModel.value.trim();
  cfg.edit_model = r.editModel.value.trim();
  cfg.language = r.lang.value;
  cfg.import_language = r.importLang.value.trim();
  cfg.temperature = parseFloat(r.temp.value) || 0;
  cfg.max_tokens = parseInt(r.maxTokens.value, 10) || 0;
  cfg.import_max_output_tokens = parseInt(r.importMax.value, 10) || 0;
  cfg.oracle_max_tool_iterations = parseInt(r.oracleIters.value, 10) || 0;
  cfg.request_timeout_seconds = parseInt(r.timeout.value, 10) || 0;
  cfg.auto_save = r.autosave.checked;
  cfg.auto_save_interval = parseInt(r.autosaveInt.value, 10) || 0;
  cfg.tts = Object.assign({}, cfgCache.tts || {}, { enabled: r.ttsEnabled.checked, voice: r.ttsVoice.value });
  cfg.spoiler_guard = Object.assign({}, cfgCache.spoiler_guard || {}, { enabled: r.sgEnabled.checked, model: r.sgModel.value.trim() });
  cfg.telegram_chat_id = parseInt(r.tgChat.value, 10) || 0;
  cfg.telegram_allowed_users = r.tgUsers.value.split("\n").map((s) => s.trim()).filter(Boolean);
  // Secrets are write-only: send what was typed (empty = keep current, per the server).
  cfg.openai_api_key = r.kOpenAI.value.trim();
  cfg.anthropic_api_key = r.kAnthropic.value.trim();
  cfg.gemini_api_key = r.kGemini.value.trim();
  cfg.telegram_token = r.tgToken.value.trim();
  try { await api("PUT", "/config", cfg); status("Settings saved."); loadSettings(); }
  catch (err) { status(err.message, true); }
});

// --- Party & character sheets (phase 2, issue #67) -----------------------

const ABILITIES = ["STR", "DEX", "CON", "INT", "WIS", "CHA"];
const CONDITIONS = ["Blinded", "Charmed", "Deafened", "Exhausted", "Frightened",
  "Grappled", "Incapacitated", "Invisible", "Paralyzed", "Petrified", "Poisoned",
  "Prone", "Restrained", "Stunned", "Unconscious"];
let chargenOpts = null;

async function getChargenOpts() {
  if (chargenOpts) return chargenOpts;
  try { chargenOpts = await api("GET", "/chargen/options"); } catch { chargenOpts = { races: [], classes: [] }; }
  return chargenOpts;
}

// Generic modal with a title, a body node, and auto-cleanup.
function openModal(title, bodyNode, opts = {}) {
  const overlay = el("div", "modal");
  const card = el("div", "modal-card" + (opts.wide ? " wide" : ""));
  const head = el("div", "modal-head");
  head.append(el("span", null, title));
  const x = el("button", "ghost", "✕");
  x.onclick = () => overlay.remove();
  head.append(x);
  card.append(head, bodyNode);
  overlay.append(card);
  overlay.addEventListener("click", (e) => { if (e.target === overlay) overlay.remove(); });
  document.body.append(overlay);
  return overlay;
}

function field(label, input) {
  const f = el("label", "field", label);
  f.append(input);
  return f;
}
function input(value, attrs = {}) {
  const i = el("input");
  if (value != null) i.value = value;
  for (const [k, v] of Object.entries(attrs)) i.setAttribute(k, v);
  return i;
}
function numInput(value) { return input(value ?? 0, { type: "number" }); }
function selectFrom(options, value) {
  const s = el("select");
  for (const o of options) { const op = el("option", null, o); if (o === value) op.selected = true; s.append(op); }
  return s;
}
function textarea(value, rows) {
  const t = el("textarea"); t.rows = rows || 4; if (value != null) t.value = value; return t;
}

// --- Party editor --------------------------------------------------------

async function openPartyEditor() {
  if (!current) return;
  if (!hasLegacyDND5E()) { status("Party editing requires the exact built-in D&D 5e rules package.", true); return; }
  const opts = await getChargenOpts();
  const body = el("div", "form");

  const list = el("div", "list small");
  const renderList = () => {
    list.innerHTML = "";
    for (const c of (sess && sess.characters) || [])
      list.append(el("div", "card small", `${c.name} — L${c.level || 1} ${c.race || ""} ${c.class || ""}`));
    if (!list.children.length) list.append(el("div", "muted", "Empty party."));
  };
  renderList();

  body.append(el("div", "label", "Current party"));
  body.append(list);

  const prompt = textarea("", 3); prompt.placeholder = "Describe the party you want (e.g. 'a sneaky trio: a rogue, a bard and a ranger')";
  body.append(field("Generate with AI", prompt));

  const actions = el("div", "actions");
  const gen = el("button", null, "Generate with AI");
  gen.onclick = async () => {
    const txt = prompt.value.trim();
    if (!txt) { status("Describe the party first.", true); return; }
    gen.disabled = true; gen.textContent = "Generating…";
    try {
      await api("POST", "/sessions/" + encodeURIComponent(current) + "/party/plan", { prompt: txt });
      await refreshState(); renderList(); status("Party generated.");
    } catch (e) { status(e.message, true); }
    finally { gen.disabled = false; gen.textContent = "Generate with AI"; }
  };
  const def = el("button", "ghost", "Default party");
  def.onclick = async () => {
    try { await api("POST", "/sessions/" + encodeURIComponent(current) + "/party/default");
      await refreshState(); renderList(); status("Default party set."); }
    catch (e) { status(e.message, true); }
  };
  const neu = el("button", "ghost", "New character…");
  neu.onclick = () => openCharacterCreator(opts, () => renderList());
  const fromRoster = el("button", "ghost", "From roster…");
  fromRoster.onclick = () => openRosterPicker(() => renderList());
  const toRoster = el("button", "ghost", "Save party → roster");
  toRoster.onclick = async () => {
    try { await api("POST", "/sessions/" + encodeURIComponent(current) + "/party/save-to-roster");
      status("Party saved to roster."); }
    catch (e) { status(e.message, true); }
  };
  actions.append(gen, def, neu, fromRoster, toRoster);
  body.append(actions);
  openModal("Party editor", body, { wide: false });
}

// openRosterPicker lists campaign-roster characters and adds a chosen one to the
// open session's party.
async function openRosterPicker(onDone) {
  const body = el("div", "form");
  let roster = [];
  try { roster = (await api("GET", "/roster")) || []; } catch (e) { status(e.message, true); }
  if (!roster.length) { body.append(el("div", "muted", "The roster is empty.")); }
  for (const rc of roster) {
    const card = el("div", "card small");
    card.append(el("span", null, `${rc.name} — L${rc.level || 1} ${rc.race || ""} ${rc.class || ""}`));
    card.append(el("span", "spacer"));
    const add = el("button", "ghost small", "Add to party");
    add.onclick = async () => {
      try {
        const party = (await api("GET", "/sessions/" + encodeURIComponent(current) + "/party")) || [];
        party.push(rc);
        await api("PUT", "/sessions/" + encodeURIComponent(current) + "/party", party);
        await refreshState(); status(rc.name + " added to the party."); if (onDone) onDone();
      } catch (e) { status(e.message, true); }
    };
    card.append(add);
    body.append(card);
  }
  openModal("Add from roster", body, { wide: false });
}

// --- Character creator ---------------------------------------------------

async function openCharacterCreator(opts, onDone, target) {
  target = target || "party"; // "party" (add to session) | "roster" (save to roster only)
  opts = opts || (await getChargenOpts());
  const body = el("div", "form");
  const name = input("", { placeholder: "Name" });
  const race = selectFrom(opts.races || [], "Human");
  const cls = selectFrom(opts.classes || [], "Fighter");
  const level = numInput(1);
  const bg = input("", { placeholder: "Background" });
  const align = input("", { placeholder: "Alignment" });
  const grid = el("div", "fgrid");
  grid.append(field("Name", name), field("Race", race), field("Class", cls),
    field("Level", level), field("Background", bg), field("Alignment", align));
  body.append(grid);

  // Abilities.
  body.append(el("div", "label", "Abilities"));
  const abilRow = el("div", "abilrow");
  const abInputs = {};
  const std = (opts.standard_array || [15, 14, 13, 12, 10, 8]);
  ABILITIES.forEach((a, i) => { const inp = numInput(std[i] ?? 10); abInputs[a] = inp; abilRow.append(field(a, inp)); });
  body.append(abilRow);
  const abilActions = el("div", "actions");
  const arr = el("button", "ghost small", "Standard array");
  arr.onclick = () => ABILITIES.forEach((a, i) => abInputs[a].value = std[i] ?? 10);
  const roll = el("button", "ghost small", "Roll 4d6 (drop lowest)");
  roll.onclick = () => ABILITIES.forEach((a) => abInputs[a].value = roll4d6());
  abilActions.append(arr, roll);
  body.append(abilActions);

  const toRoster = el("input"); toRoster.type = "checkbox";
  const rosterLbl = el("label", "field"); rosterLbl.append(toRoster, document.createTextNode(" Also save to roster"));
  if (target === "party") body.append(rosterLbl); // in roster mode it always saves to the roster

  const create = el("button", null, "Create");
  create.onclick = async () => {
    if (!name.value.trim()) { status("Name is required.", true); return; }
    const abilities = {}; ABILITIES.forEach((a) => abilities[a.toLowerCase()] = parseInt(abInputs[a].value || "10", 10));
    const payload = {
      name: name.value.trim(), race: race.value, class: cls.value,
      level: parseInt(level.value || "1", 10), abilities,
    };
    try {
      const c = await api("POST", "/chargen", payload);
      c.background = bg.value.trim(); c.alignment = align.value.trim();
      if (target === "roster") {
        await api("POST", "/roster", c);
        status("Character saved to roster.");
      } else {
        // Append to the session party.
        const party = (await api("GET", "/sessions/" + encodeURIComponent(current) + "/party")) || [];
        party.push(c);
        await api("PUT", "/sessions/" + encodeURIComponent(current) + "/party", party);
        await refreshState();
        // If also-save-to-roster is requested, surface any failure rather than
        // reporting unqualified success.
        if (toRoster.checked) {
          try { await api("POST", "/roster", c); status("Character added and saved to roster."); }
          catch (re) { status("Character added to the party, but saving to roster failed: " + re.message, true); }
        } else {
          status("Character added.");
        }
      }
      if (onDone) onDone();
      create.closest(".modal").remove();
    } catch (e) { status(e.message, true); }
  };
  body.append(create);
  openModal("New character", body, { wide: false });
}

function roll4d6() {
  const r = () => 1 + Math.floor(Math.random() * 6);
  const d = [r(), r(), r(), r()].sort((a, b) => a - b);
  return d[1] + d[2] + d[3];
}

// --- Sheet editor (full 5e sheet, optimistic concurrency) ----------------

function openSheetEditor(character) {
  if (!hasLegacyDND5E()) { status("Character-sheet editing requires the exact built-in D&D 5e rules package.", true); return; }
  const baseChar = character;                       // the loaded version (baseline)
  const c = JSON.parse(JSON.stringify(character));  // working copy
  const body = el("div", "form");

  // Identity.
  const name = input(c.name), race = input(c.race), cls = input(c.class);
  const level = numInput(c.level), bg = input(c.background || ""), align = input(c.alignment || "");
  const idg = el("div", "fgrid");
  idg.append(field("Name", name), field("Race", race), field("Class", cls),
    field("Level", level), field("Background", bg), field("Alignment", align));
  body.append(idg);

  // Abilities.
  const secA = el("div", "sheet-sec"); secA.append(el("div", "label", "Abilities"));
  const abilRow = el("div", "abilrow"); const ab = c.abilities || {};
  const abInputs = {};
  ABILITIES.forEach((a) => { const inp = numInput(ab[a.toLowerCase()] ?? 10); abInputs[a] = inp; abilRow.append(field(a, inp)); });
  secA.append(abilRow); body.append(secA);

  // Combat / resources.
  const secC = el("div", "sheet-sec"); secC.append(el("div", "label", "Combat & resources"));
  const maxhp = numInput(c.max_hp), curhp = numInput(c.current_hp), temphp = numInput(c.temp_hp || 0);
  const ac = numInput(c.ac), init = numInput(c.initiative), speed = numInput(c.speed);
  const prof = numInput(c.proficiency_bonus), hdu = numInput(c.hit_dice_used || 0);
  const gold = numInput(c.gold || 0), xp = numInput(c.xp || 0);
  const insp = el("input"); insp.type = "checkbox"; insp.checked = !!c.inspiration;
  const inspLbl = el("label", "field"); inspLbl.append(insp, document.createTextNode(" Inspiration"));
  const cg = el("div", "fgrid");
  cg.append(field("Max HP", maxhp), field("Current HP", curhp), field("Temp HP", temphp),
    field("AC", ac), field("Initiative", init), field("Speed", speed),
    field("Prof. bonus", prof), field("Hit dice used", hdu), field("Gold", gold), field("XP", xp));
  secC.append(cg, inspLbl); body.append(secC);

  // Saving throws (proficient abilities).
  const secS = el("div", "sheet-sec"); secS.append(el("div", "label", "Saving throw proficiencies"));
  const saveChecks = el("div", "checks"); const saves = new Set((c.saving_throws || []));
  const saveInputs = {};
  ABILITIES.forEach((a, i) => {
    const cb = el("input"); cb.type = "checkbox"; cb.checked = saves.has(i); saveInputs[i] = cb;
    const l = el("label"); l.append(cb, document.createTextNode(" " + a)); saveChecks.append(l);
  });
  secS.append(saveChecks); body.append(secS);

  // Skills (proficient / expert).
  const secSk = el("div", "sheet-sec"); secSk.append(el("div", "label", "Skills (P = proficient, E = expert)"));
  const skillRows = el("div", "checks");
  const skillState = (c.skills || []).map((sk) => {
    const p = el("input"); p.type = "checkbox"; p.checked = !!sk.proficient;
    const e = el("input"); e.type = "checkbox"; e.checked = !!sk.expert;
    const l = el("label"); l.append(p, document.createTextNode(" P "), e, document.createTextNode(" E — " + sk.name));
    skillRows.append(l);
    return { sk, p, e };
  });
  secSk.append(skillRows); body.append(secSk);

  // Conditions.
  const secCo = el("div", "sheet-sec"); secCo.append(el("div", "label", "Conditions"));
  const condChecks = el("div", "checks"); const conds = new Set(c.conditions || []); const condInputs = {};
  CONDITIONS.forEach((cn) => {
    const cb = el("input"); cb.type = "checkbox"; cb.checked = conds.has(cn); condInputs[cn] = cb;
    const l = el("label"); l.append(cb, document.createTextNode(" " + cn)); condChecks.append(l);
  });
  secCo.append(condChecks); body.append(secCo);

  // Languages / proficiencies (CSV) + inventory / features / spells (text).
  const secL = el("div", "sheet-sec");
  const langs = input((c.languages || []).join(", "));
  const profs = input((c.proficiencies || []).join(", "));
  secL.append(field("Languages (comma-separated)", langs), field("Other proficiencies (comma-separated)", profs));
  const inv = textarea((c.inventory || []).map((it) => `${it.name} x${it.quantity || 1}${it.equipped ? " [E]" : ""}`).join("\n"), 4);
  secL.append(field("Inventory — one per line: Name xN [E]", inv));
  const feats = textarea((c.features || []).map((f) => `${f.name} | ${f.source || ""} | ${f.description || ""}`).join("\n"), 3);
  secL.append(field("Features — Name | Source | Description", feats));
  const notes = textarea(c.notes || "", 3);
  secL.append(field("Notes", notes));
  body.append(secL);

  // Spellcasting (optional).
  const secSp = el("div", "sheet-sec"); secSp.append(el("div", "label", "Spellcasting"));
  const hasSp = el("input"); hasSp.type = "checkbox"; hasSp.checked = !!c.spellcasting;
  const hasSpLbl = el("label", "field"); hasSpLbl.append(hasSp, document.createTextNode(" Has spellcasting"));
  const sp = c.spellcasting || { ability: 3, save_dc: 0, attack_bonus: 0, slots: { max: [0,0,0,0,0,0,0,0,0], used: [0,0,0,0,0,0,0,0,0] }, spells: [] };
  const spAbility = selectFrom(ABILITIES, ABILITIES[sp.ability || 0]);
  const spDC = numInput(sp.save_dc || 0), spAtk = numInput(sp.attack_bonus || 0);
  const spTop = el("div", "fgrid"); spTop.append(field("Ability", spAbility), field("Save DC", spDC), field("Attack bonus", spAtk));
  const slotGrid = el("div", "slotgrid"); const slotMax = [], slotUsed = [];
  for (let i = 0; i < 9; i++) {
    const m = numInput((sp.slots && sp.slots.max && sp.slots.max[i]) || 0);
    const u = numInput((sp.slots && sp.slots.used && sp.slots.used[i]) || 0);
    slotMax.push(m); slotUsed.push(u);
    const cell = el("div", "field", `L${i + 1} (max/used)`);
    const pair = el("div", "row"); pair.style.margin = "0"; pair.append(m, u); cell.append(pair); slotGrid.append(cell);
  }
  const spells = textarea((sp.spells || []).map((s) => `${s.name} | ${s.level || 0}${s.prepared ? " | P" : ""}${s.school ? " | " + s.school : ""}`).join("\n"), 3);
  const spBody = el("div"); spBody.append(spTop, el("div", "label", "Spell slots"), slotGrid, field("Spells — Name | Level | P | School", spells));
  spBody.classList.toggle("hidden", !hasSp.checked);
  hasSp.onchange = () => spBody.classList.toggle("hidden", !hasSp.checked);
  secSp.append(hasSpLbl, spBody); body.append(secSp);

  // Save.
  const save = el("button", null, "Save sheet");
  save.onclick = async () => {
    const edited = JSON.parse(JSON.stringify(c));
    edited.name = name.value.trim(); edited.race = race.value.trim(); edited.class = cls.value.trim();
    edited.level = parseInt(level.value || "1", 10); edited.background = bg.value.trim(); edited.alignment = align.value.trim();
    edited.abilities = {}; ABILITIES.forEach((a) => edited.abilities[a.toLowerCase()] = parseInt(abInputs[a].value || "10", 10));
    edited.max_hp = int(maxhp); edited.current_hp = int(curhp); edited.temp_hp = int(temphp);
    edited.ac = int(ac); edited.initiative = int(init); edited.speed = int(speed);
    edited.proficiency_bonus = int(prof); edited.hit_dice_used = int(hdu); edited.gold = int(gold); edited.xp = int(xp);
    edited.inspiration = insp.checked;
    edited.saving_throws = ABILITIES.map((a, i) => i).filter((i) => saveInputs[i].checked);
    edited.skills = skillState.map(({ sk, p, e }) => Object.assign({}, sk, { proficient: p.checked, expert: e.checked }));
    edited.conditions = CONDITIONS.filter((cn) => condInputs[cn].checked);
    edited.languages = csv(langs.value);
    edited.proficiencies = csv(profs.value);
    edited.inventory = parseInventory(inv.value);
    edited.features = parseFeatures(feats.value);
    edited.notes = notes.value;
    if (hasSp.checked) {
      edited.spellcasting = {
        ability: ABILITIES.indexOf(spAbility.value), save_dc: int(spDC), attack_bonus: int(spAtk),
        slots: { max: slotMax.map(int), used: slotUsed.map(int) }, spells: parseSpells(spells.value),
      };
    } else { edited.spellcasting = null; }

    try {
      await api("PUT", "/sessions/" + encodeURIComponent(current) + "/characters/" + encodeURIComponent(baseChar.name),
        { base: baseChar, edited });
      await refreshState();
      status("Sheet saved.");
      save.closest(".modal").remove();
    } catch (e) {
      status(e.message, true); // 409 conflict message comes from the server
    }
  };
  body.append(save);
  openModal("Edit sheet — " + (c.name || ""), body, { wide: true });
}

function int(inp) { return parseInt(inp.value || "0", 10) || 0; }
function csv(s) { return (s || "").split(",").map((x) => x.trim()).filter(Boolean); }
function parseInventory(text) {
  return (text || "").split("\n").map((l) => l.trim()).filter(Boolean).map((l) => {
    const eq = /\[e\]/i.test(l); l = l.replace(/\[e\]/ig, "").trim();
    const m = l.match(/^(.*?)\s*x\s*(\d+)$/i);
    return m ? { name: m[1].trim(), quantity: parseInt(m[2], 10), equipped: eq } : { name: l, quantity: 1, equipped: eq };
  });
}
function parseFeatures(text) {
  return (text || "").split("\n").map((l) => l.trim()).filter(Boolean).map((l) => {
    const [name, source, ...rest] = l.split("|").map((x) => x.trim());
    return { name: name || "", source: source || "", description: rest.join(" | ") };
  });
}
function parseSpells(text) {
  return (text || "").split("\n").map((l) => l.trim()).filter(Boolean).map((l) => {
    const parts = l.split("|").map((x) => x.trim());
    return { name: parts[0] || "", level: parseInt(parts[1] || "0", 10) || 0, prepared: /^p$/i.test(parts[2] || ""), school: parts[3] || "" };
  });
}

// --- Module editor (phase 5, issue #70) ----------------------------------

let editAdv = null;   // the adventure being edited (full object)
let editId = null;    // its module id
let edSel = null;     // current selection descriptor

// Friendly fields per node type; everything else on the node is edited via the
// "Advanced (JSON)" box, so nothing is uneditable. kinds: text | area | csv | lines.
const NODE_FIELDS = {
  meta: [["title", "text"], ["author", "text"], ["system", "text"], ["language", "text"],
    ["ruleset.id", "text"], ["ruleset.version", "text"],
    ["start_room", "text"], ["summary", "area"], ["background", "area"], ["introduction", "area"],
    ["conclusion", "area"], ["hooks", "lines"]],
  zone: [["id", "text"], ["name", "text"], ["overview", "area"], ["description", "area"], ["map_image", "text"]],
  room: [["id", "text"], ["name", "text"], ["read_aloud", "area"], ["dm_notes", "area"], ["image", "text"],
    ["npc_ids", "csv"], ["event_ids", "csv"], ["treasure", "lines"]],
  npc: [["id", "text"], ["name", "text"], ["role", "text"], ["appearance", "area"], ["personality", "area"],
    ["motivations", "area"], ["secrets", "area"], ["voice", "text"], ["disposition", "text"], ["image", "text"],
    ["default_location", "text"], ["knowledge", "lines"], ["sample_dialogue", "lines"]],
  event: [["id", "text"], ["name", "text"], ["trigger", "text"], ["description", "area"], ["read_aloud", "area"],
    ["dm_notes", "area"], ["consequences", "area"]],
  item: [["id", "text"], ["name", "text"], ["rarity", "text"], ["description", "area"], ["mechanics", "area"], ["image", "text"]],
  table: [["id", "text"], ["name", "text"], ["description", "area"], ["dice", "text"]],
};
// Keys handled elsewhere, so the Advanced (JSON) box never exposes them: rooms
// live as their own tree nodes under a zone, and the top-level collections are
// edited via the tree — keeping them out of the metadata JSON box means a stray
// edit there can't erase zones/NPCs/etc.
const NODE_EXTRA_SKIP = {
  zone: ["rooms"],
  meta: ["zones", "npcs", "events", "items", "tables"],
};

async function openEditor(id) {
  try {
    editAdv = await api("GET", "/adventures/" + encodeURIComponent(id));
  } catch (e) { status(e.message, true); return; }
  editId = id;
  editAdv.zones = editAdv.zones || [];
  edSel = { type: "meta" };
  document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
  $("#view-editor").classList.remove("hidden");
  $("#ed-title").textContent = editAdv.title || id;
  renderEdTree();
  renderEdForm();
}

function renderEdTree() {
  const box = $("#ed-tree"); box.innerHTML = "";
  const add = (key, label, sel) => {
    const n = el("div", "node", label);
    if (sameSel(sel, edSel)) n.classList.add("active");
    n.onclick = () => { edSel = sel; renderEdTree(); renderEdForm(); };
    return n;
  };
  box.append(add("meta", "▸ " + (editAdv.title || "Adventure"), { type: "meta" }));
  const group = (label, nodes) => {
    if (!nodes.length) { return; }
    const g = el("div", "group");
    g.append(el("div", "row-label", label));
    const kids = el("div", "children");
    nodes.forEach((n) => kids.append(n));
    g.append(kids); box.append(g);
  };
  // Zones with their rooms nested.
  const zoneNodes = [];
  (editAdv.zones || []).forEach((z, zi) => {
    zoneNodes.push(add("", z.name || z.id || ("zone " + zi), { type: "zone", zi }));
    const kids = el("div", "children");
    (z.rooms || []).forEach((r, ri) => kids.append(add("", r.name || r.id || ("room " + ri), { type: "room", zi, ri })));
    if (kids.children.length) zoneNodes.push(kids);
  });
  group("Zones", zoneNodes);
  group("NPCs", (editAdv.npcs || []).map((n, i) => add("", n.name || n.id, { type: "npc", i })));
  group("Events", (editAdv.events || []).map((e, i) => add("", e.name || e.id, { type: "event", i })));
  group("Items", (editAdv.items || []).map((it, i) => add("", it.name || it.id, { type: "item", i })));
  group("Tables", (editAdv.tables || []).map((t, i) => add("", t.name || t.id, { type: "table", i })));
}

function sameSel(a, b) { return a && b && a.type === b.type && a.zi === b.zi && a.ri === b.ri && a.i === b.i; }

function edNode() {
  const s = edSel; if (!s) return null;
  if (s.type === "meta") return editAdv;
  if (s.type === "zone") return editAdv.zones[s.zi];
  if (s.type === "room") return editAdv.zones[s.zi].rooms[s.ri];
  if (s.type === "npc") return editAdv.npcs[s.i];
  if (s.type === "event") return editAdv.events[s.i];
  if (s.type === "item") return editAdv.items[s.i];
  if (s.type === "table") return editAdv.tables[s.i];
  return null;
}

function edFieldValue(node, key) {
  return key.split(".").reduce((value, segment) => value && value[segment], node);
}

function setEdFieldValue(node, key, value) {
  const path = key.split(".");
  let target = node;
  for (const segment of path.slice(0, -1)) {
    if (!target[segment] || typeof target[segment] !== "object" || Array.isArray(target[segment])) target[segment] = {};
    target = target[segment];
  }
  target[path[path.length - 1]] = value;
}

function renderEdForm() {
  const node = edNode();
  const box = $("#ed-form"); box.innerHTML = "";
  if (!node) { box.append(el("div", "muted", "Nothing selected.")); return; }
  const type = edSel.type;
  $("#ed-nodetitle").textContent = type === "meta" ? "Adventure metadata" : (type[0].toUpperCase() + type.slice(1));

  const fields = NODE_FIELDS[type] || [];
  const friendlyKeys = new Set(fields.map((f) => f[0].split(".")[0]));
  for (const [key, kind] of fields) {
    const value = edFieldValue(node, key);
    let inp;
    if (kind === "area") { inp = textarea(value || "", 3); }
    else if (kind === "lines") { inp = textarea((value || []).join("\n"), 3); }
    else if (kind === "csv") { inp = input((value || []).join(", ")); }
    else { inp = input(value != null ? value : ""); }
    // Update the model live on each keystroke, but DON'T re-render the form (that
    // would drop focus mid-typing). Refresh the tree/header only on blur (change).
    inp.addEventListener("input", () => {
      if (kind === "lines") setEdFieldValue(node, key, inp.value.split("\n").map((s) => s.trim()).filter(Boolean));
      else if (kind === "csv") setEdFieldValue(node, key, inp.value.split(",").map((s) => s.trim()).filter(Boolean));
      else setEdFieldValue(node, key, inp.value);
    });
    if (key === "name" || key === "id" || key === "title") {
      inp.addEventListener("change", () => { $("#ed-title").textContent = editAdv.title || editId; renderEdTree(); });
    }
    box.append(field(key.replace(/[_.]/g, " "), inp));
  }

  // Advanced JSON for the remaining fields.
  const skip = new Set([...friendlyKeys, ...(NODE_EXTRA_SKIP[type] || [])]);
  const advObj = {};
  for (const k of Object.keys(node)) if (!skip.has(k)) advObj[k] = node[k];
  const adv = textarea(JSON.stringify(advObj, null, 2), 8);
  adv.addEventListener("change", () => {
    let parsed;
    try { parsed = JSON.parse(adv.value); } catch (e) { status("Advanced JSON is invalid: " + e.message, true); return; }
    // Must be a plain object; a valid non-object (e.g. [] or null) would otherwise
    // wipe the node's non-friendly fields after the delete below.
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
      status("Advanced fields must be a JSON object.", true);
      return;
    }
    // Only the non-reserved keys may be edited here; reserved/structural keys
    // (e.g. zones/rooms/npcs) are managed by the tree, so ignore them in the
    // parsed JSON rather than let them overwrite tree-managed collections.
    const ignored = Object.keys(parsed).filter((k) => skip.has(k));
    for (const k of Object.keys(node)) if (!skip.has(k)) delete node[k];
    for (const k of Object.keys(parsed)) if (!skip.has(k)) node[k] = parsed[k];
    status(ignored.length
      ? "Advanced fields applied (ignored reserved keys: " + ignored.join(", ") + ")."
      : "Advanced fields applied.");
  });
  box.append(el("div", "label", "Advanced (JSON) — nested fields like exits, stat_block, rows"));
  box.append(adv);

  // Per-node delete (not for metadata).
  if (type !== "meta") {
    const del = el("button", "ghost small", "Delete this " + type);
    del.onclick = () => edDeleteNode();
    const bar = el("div", "actions"); bar.append(del); box.append(bar);
  }
}

function edGenId(prefix) { return prefix + "-" + Math.random().toString(36).slice(2, 8); }

$("#ed-add").onclick = () => {
  const type = $("#ed-addtype").value;
  if (type === "zone") { editAdv.zones.push({ id: edGenId("zone"), name: "New zone", rooms: [] }); edSel = { type: "zone", zi: editAdv.zones.length - 1 }; }
  else if (type === "room") {
    if (!editAdv.zones.length) editAdv.zones.push({ id: edGenId("zone"), name: "New zone", rooms: [] });
    const zi = (edSel && edSel.type === "zone") ? edSel.zi : (edSel && edSel.type === "room" ? edSel.zi : 0);
    editAdv.zones[zi].rooms = editAdv.zones[zi].rooms || [];
    editAdv.zones[zi].rooms.push({ id: edGenId("room"), name: "New room" });
    edSel = { type: "room", zi, ri: editAdv.zones[zi].rooms.length - 1 };
  } else {
    const coll = type + "s";
    editAdv[coll] = editAdv[coll] || [];
    editAdv[coll].push({ id: edGenId(type), name: "New " + type });
    edSel = { type, i: editAdv[coll].length - 1 };
  }
  renderEdTree(); renderEdForm();
};

function edDeleteNode() {
  const s = edSel; if (!s || s.type === "meta") return;
  if (!confirm("Delete this " + s.type + "?")) return;
  if (s.type === "zone") editAdv.zones.splice(s.zi, 1);
  else if (s.type === "room") editAdv.zones[s.zi].rooms.splice(s.ri, 1);
  else editAdv[s.type + "s"].splice(s.i, 1);
  edSel = { type: "meta" };
  renderEdTree(); renderEdForm();
}

$("#ed-back").onclick = () => { editAdv = null; editId = null; edSel = null; show("library"); };
$("#ed-save").onclick = async () => {
  try { await api("PUT", "/adventures/" + encodeURIComponent(editId), editAdv); status("Adventure saved."); }
  catch (e) { status(e.message, true); }
};
$("#ed-validate").onclick = async () => {
  try {
    const r = await api("POST", "/adventures/" + encodeURIComponent(editId) + "/validate", editAdv);
    const errs = r.errors || [];
    if (!errs.length) { status("Valid — no problems found."); }
    else { alert("Validation problems (" + errs.length + "):\n\n" + errs.join("\n")); }
  } catch (e) { status(e.message, true); }
};
$("#ed-export").onclick = () => downloadAuthed("/adventures/" + encodeURIComponent(editId) + "/export", editId + ".tar.gz");
$("#ed-dmbook").onclick = () => downloadAuthed("/adventures/" + encodeURIComponent(editId) + "/dmbook", editId + "-dmbook.md");

// downloadAuthed fetches a file with the bearer header (so token-protected
// downloads work — an anchor can't set Authorization) and saves it locally.
async function downloadAuthed(path, filename) {
  const headers = {};
  if (token()) headers["Authorization"] = "Bearer " + token();
  try {
    const resp = await fetch("/api" + path, { headers });
    if (!resp.ok) { const d = await resp.json().catch(() => null); throw new Error((d && d.error) || ("HTTP " + resp.status)); }
    const url = URL.createObjectURL(await resp.blob());
    const a = el("a"); a.href = url; a.download = filename; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 10000);
  } catch (e) { status(e.message, true); }
}

// --- boot ----------------------------------------------------------------
show("library");
