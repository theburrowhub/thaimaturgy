const state = {
  library: { adventures: [], sessions: [] },
  current: null,
  selectedAdventure: null,
};

const $ = (id) => document.getElementById(id);

function api() {
  const app = window.go?.wailsapp?.App || window.go?.main?.App;
  if (app) return app;
  return demoBackend;
}

const demoAdventure = {
  id: 'the-sunken-crypt', title: 'The Sunken Crypt', system: 'D&D 5e', summary: 'A sunken shrine full of drowned vows.',
  zones: [{ id: 'crypt', name: 'Sunken Crypt', overview: 'Wet stone, candle smoke and old bells.', rooms: [
    { id: 'stair', name: 'Flooded Stair', read_aloud: 'Cold water laps against worn steps. A bell rings once somewhere below.', dm_notes: 'A pressure plate is hidden under the third submerged step.', exits: [{ to: 'altar', direction: 'east' }] },
    { id: 'altar', name: 'Drowned Altar', read_aloud: 'Black water mirrors a cracked altar crowned by a tarnished sun-disc.', dm_notes: 'The reliquary opens only when the old hymn is spoken.', exits: [{ to: 'stair', direction: 'west' }] }
  ]}]
};

const demoBackend = {
  async GetLibrary() { return { adventures: [{ id: demoAdventure.id, title: demoAdventure.title, system: demoAdventure.system }], sessions: [] }; },
  async StartSession() { return demoPayload('stair'); },
  async LoadSession() { return demoPayload('stair'); },
  async MoveParty(roomID) { return demoPayload(roomID); },
  async Submit(input) { return { success: true, message: `Demo command accepted: ${input}`, session: demoPayload('stair') }; }
};
function demoPayload(roomID) {
  const room = demoAdventure.zones[0].rooms.find((r) => r.id === roomID) || demoAdventure.zones[0].rooms[0];
  return { adventure: demoAdventure, current_room: room, current_zone: demoAdventure.zones[0], state: { name: 'demo-wails-session', current_room: room.id, visited_rooms: { [room.id]: true }, log: { entries: [{ type: 'location', message: `Entered ${room.name}` }] } } };
}

async function loadLibrary() {
  try {
    state.library = await api().GetLibrary();
    if (!state.selectedAdventure && state.library.adventures?.length) {
      state.selectedAdventure = state.library.adventures[0].id;
    }
    renderLibrary();
    if (!window.go && new URLSearchParams(window.location.search).get('demo') === 'session' && !state.current) {
      await startSelectedAdventure();
    }
  } catch (err) {
    renderMessage('system', `Could not load library: ${err.message || err}`);
  }
}

function renderLibrary() {
  const adventures = state.library.adventures || [];
  const sessions = state.library.sessions || [];
  $('adventure-count').textContent = `${adventures.length} modules`;
  $('session-count').textContent = `${sessions.length} saves`;
  $('library-list').innerHTML = adventures.map((adv) => `
    <button class="node ${adv.id === state.selectedAdventure ? 'active' : ''}" data-adventure="${escapeAttr(adv.id)}">
      <span class="node-dot"></span>
      <span><strong>${escapeHtml(adv.title || adv.id)}</strong><small>${escapeHtml(adv.system || 'Adventure module')}</small></span>
    </button>`).join('') || '<p class="empty">No imported adventures yet.</p>';
  $('session-list').innerHTML = sessions.map((s) => `
    <button class="save" data-session="${escapeAttr(s.name)}">
      <strong>${escapeHtml(s.name)}</strong><small>${escapeHtml(s.adventure_title || s.adventure_id || '')}</small>
    </button>`).join('') || '<p class="empty">No saved sessions.</p>';
}

function renderSession(payload) {
  state.current = payload;
  const adv = payload.adventure;
  const room = payload.current_room;
  $('crumb-adventure').textContent = adv?.title || 'Adventure';
  $('crumb-room').textContent = room?.name || 'No location';
  $('session-title').textContent = adv?.title || 'DM Oracle';
  $('session-subtitle').textContent = payload.state?.name ? `Session ${payload.state.name}` : 'Running locally in Wails';
  $('room-id').textContent = room?.id || '—';
  renderTree(adv, payload.state);
  renderDetail(payload);
  renderTranscript(payload);
}

function renderTree(adv, session) {
  if (!adv) return renderLibrary();
  $('library-list').innerHTML = (adv.zones || []).map((zone) => `
    <div class="zone">
      <div class="zone-title">${escapeHtml(zone.name || zone.id)}</div>
      ${(zone.rooms || []).map((room) => `
        <button class="node room ${room.id === session.current_room ? 'active' : ''}" data-room="${escapeAttr(room.id)}">
          <span class="node-dot"></span>
          <span><strong>${escapeHtml(room.name || room.id)}</strong><small>${escapeHtml(room.id)}</small></span>
        </button>`).join('')}
    </div>`).join('');
}

function renderDetail(payload) {
  const room = payload.current_room;
  if (!room) {
    $('detail-content').innerHTML = '<p class="empty">No room selected.</p>';
    return;
  }
  $('detail-content').innerHTML = `
    <article class="detail-card readaloud">
      <p class="kicker">Read aloud</p>
      <p>${escapeHtml(room.read_aloud || 'No boxed text for this room.')}</p>
    </article>
    <article class="detail-card">
      <p class="kicker">DM notes</p>
      <p>${escapeHtml(room.dm_notes || 'No DM notes recorded.')}</p>
    </article>
    <article class="detail-card">
      <p class="kicker">Exits</p>
      ${(room.exits || []).length ? room.exits.map((exit) => `<button class="exit" data-room="${escapeAttr(exit.to)}">${escapeHtml(exit.direction || 'Exit')} → ${escapeHtml(exit.to)}</button>`).join('') : '<p class="muted">No exits listed.</p>'}
    </article>`;
}

function renderTranscript(payload) {
  const entries = payload.state?.log?.entries || [];
  $('transcript').innerHTML = entries.map((entry) => `
    <div class="bubble ${entry.type || 'system'}"><span>${escapeHtml(entry.type || 'system')}</span><p>${escapeHtml(entry.message || '')}</p></div>`).join('') || '<div class="bubble system"><span>system</span><p>Session ready. Use the adventure tree or type a slash command.</p></div>';
  $('transcript').scrollTop = $('transcript').scrollHeight;
}

function renderMessage(type, text) {
  $('transcript').insertAdjacentHTML('beforeend', `<div class="bubble ${type}"><span>${escapeHtml(type)}</span><p>${escapeHtml(text)}</p></div>`);
}

async function startSelectedAdventure() {
  const id = state.selectedAdventure || state.library.adventures?.[0]?.id;
  if (!id) return renderMessage('system', 'Import an adventure module first.');
  renderSession(await api().StartSession(id));
  await loadLibrary();
}

async function submitCommand(event) {
  event.preventDefault();
  const input = $('command').value.trim();
  if (!input) return;
  $('command').value = '';
  renderMessage('dm', input);
  try {
    const result = await api().Submit(input);
    if (result?.message) renderMessage(result.success ? 'oracle' : 'system', result.message);
    if (result?.session) renderSession(result.session);
  } catch (err) {
    renderMessage('system', err.message || String(err));
  }
}

document.addEventListener('click', async (event) => {
  const adv = event.target.closest('[data-adventure]');
  const room = event.target.closest('[data-room]');
  const save = event.target.closest('[data-session]');
  if (adv) { state.selectedAdventure = adv.dataset.adventure; renderLibrary(); }
  if (room) { renderSession(await api().MoveParty(room.dataset.room)); }
  if (save) { renderSession(await api().LoadSession(save.dataset.session)); }
});

$('refresh-btn').addEventListener('click', loadLibrary);
$('new-session-btn').addEventListener('click', startSelectedAdventure);
$('composer').addEventListener('submit', submitCommand);
$('command').addEventListener('input', (event) => {
  event.target.style.height = 'auto';
  event.target.style.height = `${event.target.scrollHeight}px`;
});

function escapeHtml(value) { return String(value ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }
function escapeAttr(value) { return escapeHtml(value).replace(/'/g, '&#39;'); }

if (document.readyState === 'loading') {
  window.addEventListener('DOMContentLoaded', loadLibrary);
} else {
  loadLibrary();
}
