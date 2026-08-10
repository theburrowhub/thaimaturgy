"use strict";
// Minimal, dependency-free SPA for the thAImaturgy server (issue #36, Phase C).
// Talks to the REST API under /api and tails the session timeline over SSE.

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

// --- View switching ------------------------------------------------------

function show(view) {
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
      c.append(el("span", "muted", s.adventure_title || s.adventure_id || ""));
      c.append(el("span", "spacer"));
      const open = el("button", null, "Open");
      open.onclick = () => openSession(s.name);
      const del = el("button", "ghost", "Delete");
      del.onclick = async () => {
        if (!confirm("Delete session " + s.name + "?")) return;
        try { await api("DELETE", "/sessions/" + encodeURIComponent(s.name)); loadLibrary(); }
        catch (e) { status(e.message, true); }
      };
      c.append(open, del);
      sess.append(c);
    }
  } catch (e) { status(e.message, true); }
}

// --- Session -------------------------------------------------------------

let current = null;   // session name
let evtSource = null; // EventSource
let openGen = 0;      // bumped on each open/leave; stale async work checks it

function appendLine(cls, text) {
  const t = $("#transcript");
  t.append(el("div", cls, text));
  t.scrollTop = t.scrollHeight;
}

// leaveSession invalidates any in-flight open and tears down the live stream.
function leaveSession() {
  openGen++;
  current = null;
  if (evtSource) { evtSource.close(); evtSource = null; }
}

async function openSession(name) {
  const gen = ++openGen; // this open supersedes any earlier one
  if (evtSource) { evtSource.close(); evtSource = null; }
  try {
    const st = await api("GET", "/sessions/" + encodeURIComponent(name));
    if (gen !== openGen) return; // a newer open (or leave) happened while loading
    current = name;
    $("#session-name").textContent = name;
    $("#session-loc").textContent = st.current_room ? ("in " + (st.current_room)) : "";
    renderParty(st);
    $("#transcript").innerHTML = "";
    // Replay the persisted oracle conversation, if any.
    const conv = (st.conversation && st.conversation.messages) || [];
    for (const m of conv) appendLine(m.role === "assistant" ? "a" : "u", (m.role === "assistant" ? "" : "» ") + m.content);
    show2("session");
    subscribeEvents(name, gen);
  } catch (e) { if (gen === openGen) status(e.message, true); }
}

function renderParty(st) {
  const p = $("#party"); p.innerHTML = "";
  const party = st.characters || [];
  if (!party.length) { p.append(el("div", "muted", "No party.")); return; }
  for (const c of party) {
    const line = `${c.name} — L${c.level || 1} ${c.race || ""} ${c.class || ""}  (HP ${c.current_hp}/${c.max_hp}, AC ${c.ac})`;
    p.append(el("div", "card small", line));
  }
}

function show2(view) { // show session (not in the nav)
  document.querySelectorAll(".view").forEach((v) => v.classList.add("hidden"));
  $("#view-session").classList.remove("hidden");
}

async function subscribeEvents(name, gen) {
  if (evtSource) { evtSource.close(); evtSource = null; }
  let url = "/api/sessions/" + encodeURIComponent(name) + "/events";
  if (token()) {
    try {
      const t = await api("POST", "/sse-ticket");
      if (gen !== openGen) return; // superseded while fetching the ticket
      url += "?ticket=" + encodeURIComponent(t.ticket);
    } catch (e) { if (gen === openGen) status("live updates unavailable: " + e.message, true); return; }
  }
  if (gen !== openGen) return;
  evtSource = new EventSource(url);
  evtSource.addEventListener("log", (ev) => {
    try {
      const entry = JSON.parse(ev.data);
      appendLine("log", `[${entry.type}] ${entry.message}`);
    } catch { /* ignore */ }
  });
  evtSource.onerror = () => { /* EventSource auto-reconnects within the ticket window */ };
}

$("#back").onclick = () => { leaveSession(); show("library"); };
$("#save").onclick = async () => {
  try { await api("POST", "/sessions/" + encodeURIComponent(current) + "/save"); status("Saved."); }
  catch (e) { status(e.message, true); }
};
$("#close").onclick = async () => {
  const name = current;
  try {
    await api("POST", "/sessions/" + encodeURIComponent(name) + "/close");
    leaveSession();
    status("Closed."); show("library");
  } catch (e) { status(e.message, true); }
};

$("#ask").addEventListener("submit", async (e) => {
  e.preventDefault();
  const input = $("#ask-input");
  const text = input.value.trim();
  if (!text) return;
  input.value = "";
  if (text.startsWith("/")) {
    appendLine("u", "» " + text);
    try {
      const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/command", { input: text });
      if (r.response) appendLine("a", r.response);
      else if (r.message) appendLine("log", r.message);
    } catch (err) { appendLine("err", "⚠ " + err.message); }
  } else {
    appendLine("u", "» " + text);
    appendLine("log", "…thinking…");
    try {
      const r = await api("POST", "/sessions/" + encodeURIComponent(current) + "/oracle", { input: text });
      if (r.error) appendLine("err", "⚠ " + r.error);
      else appendLine("a", r.answer || "(no answer)");
    } catch (err) { appendLine("err", "⚠ " + err.message); }
  }
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
