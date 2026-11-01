# WalacTV visual design

## Direction

Product interface for choosing something to watch at home. A dim living room, a film ready to start, and enough ambient light to read comfortably. The experience should feel quiet and cinematic while keeping search, filters, navigation and playback controls obvious.

Color strategy: restrained. Warm ink surfaces carry the page; a muted brass accent marks primary actions, selection and focus. Coral is reserved for live/on-air and warning states. Artwork supplies the strongest visual color. No brand imitation, gradient text, decorative glass, or glow around ordinary controls.

## Brand distinction

WalacTV keeps its own brand, navigation and catalog; do not borrow Nuvio or Stremio assets, motifs, naming or branded UI. Use the existing WalacTV app mark (`apps/web/public/logo.png`) and app icon (`apps/web/public/favicon.png`) as the canonical logo assets. The VOD player may use Nuvio's general affordances as inspiration: let the video dominate, keep playback controls discreet and keep progress understandable. Live playback uses a separate TV/IPTV composition with the channel title, an explicit on-air badge and one clear return action. Keep WalacTV's warm brass palette and restrained editorial tone. The catalog uses a compact typographic heading and direct search/filter controls. The poster grid remains a practical way to browse the user's IPTV catalog.

## Color roles

| Token | Value | Use |
|---|---|---|
| `--color-canvas` | `oklch(14% 0.014 68)` | Page background |
| `--color-rail` | `oklch(17% 0.016 68)` | Navigation rail and utility surfaces |
| `--color-surface` | `oklch(20% 0.018 68)` | Forms, controls, dialogs |
| `--color-raised` | `oklch(24% 0.019 68)` | Hover and selected surfaces |
| `--color-line` | `oklch(34% 0.016 68)` | Dividers and control borders |
| `--color-text` | `oklch(92% 0.012 82)` | Main text |
| `--color-text-soft` | `oklch(78% 0.014 78)` | Supporting text |
| `--color-text-muted` | `oklch(68% 0.014 74)` | Secondary labels, still readable |
| `--color-accent` | `oklch(78% 0.12 76)` | Primary action, selected item, focus |
| `--color-accent-ink` | `oklch(20% 0.024 66)` | Text on accent controls |
| `--color-live` | `oklch(67% 0.15 32)` | Live indicator |
| `--color-success` | `oklch(70% 0.10 145)` | Successful connection/sync |
| `--color-danger` | `oklch(67% 0.14 28)` | Destructive action and errors |

Verify actual rendered contrast for all text and interactive states. Do not lower text contrast to make the page feel cinematic.

## Typography

Use one system sans stack: `Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`. Do not download fonts. Use fixed rem steps: body `0.9375rem`, supporting copy `0.875rem`, controls and labels at least `0.8125rem`, compact legal/attribution text no smaller than `0.75rem`. Page titles are `2rem` to `2.5rem`; section titles are `1.25rem` to `1.5rem`. Keep normal prose under 75ch. Use weight and scale for hierarchy, not tiny tracked uppercase labels.

## Surfaces and layout

- Desktop app shell: one stable navigation rail, one utility/header row and a flexible content area. Keep an even reading edge and let content use the available width.
- Catalog: recognizable poster art in a stable vertical ratio; direct channel marks use contained artwork and a distinct landscape or square treatment. Titles remain readable below artwork and are not repeated in a second identical card layer.
- Search and filters form a compact, clear toolbar. On smaller screens, controls wrap to rows and keep full tap targets.
- Login and source setup prioritize the form. Supporting copy remains brief and secondary.
- Player: use separate live and VOD layouts. Live emphasizes the channel name and on-air state; VOD makes the video dominant and keeps the existing progress/conversion controls discreet. Preserve one return action, loading/error states and accessible fullscreen behavior.
- Microcopy: use only text needed to identify, operate or recover from the current state. Keep labels, privacy, compatibility, attribution and useful errors; remove slogans, repeated explanations and decorative session/device statements.
- Dialogs use one surface and a clear close/action area. Do not nest cards or use modal navigation for ordinary browsing.

## Components and interaction

Buttons, inputs and selects share a minimum 44px touch height, clear disabled state, visible 2px focus indicator, and 150–220ms transitions for color/opacity/transform only. Focus color must contrast against adjacent surfaces. Errors pair a concise explanation with a useful action. Empty states explain the next step. Loading feedback reserves content space and avoids repeated announcements.

Honor `prefers-reduced-motion: reduce`: remove non-essential movement and shimmer. Artwork hover effects are subtle, do not shift layout, and are absent as a prerequisite on touch devices. Live state is identified by label and icon as well as color.

## Responsive behavior

Support 320 CSS px, 390 px mobile, 768 px tablet and 1440 px desktop, including 200% zoom. Navigation, search, filters, pagination, dialogs and player controls remain reachable without document-level horizontal scrolling. Respect device safe areas when controls approach screen edges. Do not hide actions behind hover or clip long titles without an accessible full name.
