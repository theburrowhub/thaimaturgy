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
      c.append(play);
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
    applyModeUI();
    detailPlaceholder();
    subscribeEvents(name, gen);
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
  const party = (sess && sess.characters) || [];
  if (!party.length) { p.append(el("div", "muted", "No party.")); return; }
  for (const c of party) {
    const line = `${c.name} — L${c.level || 1} ${c.race || ""} ${c.class || ""}  (HP ${c.current_hp ?? "?"}/${c.max_hp ?? "?"}, AC ${c.ac ?? "?"})`;
    p.append(el("div", "card small", line));
  }
}

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

function applyModeUI() {
  const dm = effectiveMode() === "dm";
  $("#browser-wrap").classList.toggle("hidden", dm);
  $("#party-wrap").classList.toggle("hidden", !dm);
  $("#detail-wrap").classList.toggle("hidden", dm);
  $("#mode-toggle").textContent = dm ? "Mode: Virtual DM" : "Mode: Oracle";
  $("#transcript-title").textContent = dm ? "Virtual DM" : "Oracle";
  const started = !!(sess && sess.started);
  $("#begin").classList.toggle("hidden", !(dm && !started));
  $("#rest").classList.toggle("hidden", !(dm && started));
  $("#ask-input").placeholder = dm
    ? "Describe what your character does…  (Enter sends · Shift/Ctrl+Enter = newline)"
    : "Ask the oracle, or type a /command.  (Enter sends · Shift/Ctrl+Enter = newline)";
}

$("#mode-toggle").onclick = () => runCommand("/mode");
$("#begin").onclick = () => runCommand("/begin");
$("#rest").onclick = () => {
  const kind = prompt("Rest type: short or long?", "short");
  if (!kind) return;
  const k = kind.trim().toLowerCase();
  if (k !== "short" && k !== "long") { status("Type 'short' or 'long'.", true); return; }
  runCommand("/rest " + k);
};

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
  try {
    const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/command", { input: "/roll " + notation });
    const msg = r.message || r.response || "(no result)";
    $("#dice-result").textContent = msg;
    appendLine("log", msg);
    renderLog();
  } catch (e) { $("#dice-result").textContent = e.message; }
}
$("#dice").onclick = () => { buildDice(); $("#dice-result").textContent = ""; $("#dice-modal").classList.remove("hidden"); };
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
async function loadSettings() {
  try {
    cfgCache = await api("GET", "/config");
    $("#cfg-provider").value = cfgCache.provider || "";
    $("#cfg-model").value = cfgCache.model || "";
    $("#cfg-language").value = cfgCache.language || "";
  } catch (e) { status(e.message, true); }
}
$("#settings-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const cfg = Object.assign({}, cfgCache || {}, {
    provider: $("#cfg-provider").value.trim(),
    model: $("#cfg-model").value.trim(),
    language: $("#cfg-language").value.trim(),
  });
  try { await api("PUT", "/config", cfg); status("Settings saved."); }
  catch (err) { status(err.message, true); }
});

// --- boot ----------------------------------------------------------------
show("library");
