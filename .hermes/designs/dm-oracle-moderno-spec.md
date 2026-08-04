# DM Oracle Modern Design Spec

Source reference: `.hermes/designs/dm-oracle-moderno.html`

Tony approved this as the target direction for the thAImaturgy web app.

## Product surface

Primary surface: **Command / Inspect**.

The design is a live Dungeon Master cockpit, not a landing page and not a generic dashboard. It prioritizes session control, fast inspection, readable prose and oracle interaction.

## Layout contract

Desktop:

```css
body { display: flex; flex-direction: column; overflow: hidden; }
header { height: 56px; }
main { display: grid; grid-template-columns: 264px 1fr 340px; }
```

Responsive:

- `<1100px`: hide/collapse the right detail panel initially.
- `<760px`: hide/collapse the sidebar initially.
- Production version should replace hidden panels with mobile tabs/drawers so Adventure, Oracle and Detail remain reachable.

## Visual tokens

```css
:root{
  --bg:#0d1017;
  --panel:#151923;
  --panel-2:#1a1f2c;
  --line:rgba(255,255,255,.07);
  --line-strong:rgba(255,255,255,.13);
  --text:#e9e9f0;
  --muted:#8b92a6;
  --faint:#5d6478;
  --gold:#e2b25c;
  --gold-soft:rgba(226,178,92,.14);
  --arcane:#7ee0d2;
  --arcane-soft:rgba(126,224,210,.10);
  --danger:#e0705c;
  --radius:12px;
  --radius-sm:8px;
}
```

## Typography

- `Sora`: interface text, controls, headings.
- `Spectral`: read-aloud and quoted adventure prose.
- `JetBrains Mono`: timestamps, IDs, keyboard hints, dice rolls.

If offline/local packaging is required, self-host the font files or use equivalent fallbacks while keeping the same three-role typography system.

## Component contract

### Header

- Brand block: eyebrow `DM Oracle`, adventure title.
- Breadcrumb: current zone/room with danger/pin dot.
- Actions: Library, Save, Export novel, Dice.
- Mode pill: live orb + `Mode: Oracle` / `Mode: Virtual DM`.

### Left sidebar

- Panel title `Adventure` + subtitle `Module browser`.
- Tree nodes with depth classes and selected state.
- Active location has gold soft background and optional `Party` pill.
- Bottom session log timeline with timestamps and status dots.

### Center oracle

- Panel title `Oracle` + subtitle.
- Chat transcript with distinct user/oracle bubbles.
- Read-aloud block using Spectral and gold accent.
- Composer anchored to bottom with textarea, send button and keyboard hints.

### Detail inspector

- `Move party here` primary contextual action.
- Scene title + slug.
- Read-aloud card.
- DM notes card.
- Exits/related links list.
- Later: image/map card and action chips for NPC/event/item/table entities.

## Interaction rules

- Hover states should be subtle, using `--panel-2` and text color elevation.
- Focus state uses `--arcane` outline.
- Activity/live state uses small pulsing `--arcane` orb, disabled with `prefers-reduced-motion`.
- Gold is reserved for selected party/current location, primary send/action, and read-aloud emphasis.
- Arcane/cyan is reserved for oracle, focus and live/typing states.

## Slop audit

Score: **1/10**.

The design avoids the major AI slop tells:

- no centered hero on the session surface;
- no generic feature grid;
- no fake metrics;
- no glassmorphism;
- no rainbow/tech gradient;
- no decorative icon cards.

Only issue to address before production: mobile needs accessible tabs/drawers rather than simply hiding panels.
