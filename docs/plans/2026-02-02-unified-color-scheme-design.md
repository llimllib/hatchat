# Unified Color Scheme and Design

**Date:** 2026-02-02  
**Status:** Ready for implementation

## Overview

Redesign Hatchat with an "adult, calm" aesthetic using earthy and neutral tones. Replace the current Slack-inspired purple/rounded design with a square, understated look.

## Design Principles

- **Light mode primary** with warm, calm tones
- **Tonal sidebar** for subtle definition without high contrast
- **Barely rounded corners** (`2px`) throughout—architectural but not harsh
- **Minimal shadows**—use subtle borders instead

## Color Palette

| Name | Hex | Role |
|------|-----|------|
| Coco's Black | `#1F1C1A` | Primary text, modal overlay |
| Graphite | `#3B3823` | (Reserved for future use) |
| Mushroom Forest | `#8D7C55` | Borders/dividers (at ~30% opacity) |
| Another One Bites the Dust | `#C6B7A0` | Page background, sidebar muted text |
| Shadow of the Colossus | `#A4A0A1` | Muted text (timestamps, hints) |
| Your Shadow | `#78798B` | Sidebar background |
| Stone Cold | `#5B5958` | Secondary text, input borders |
| Peat Brown | `#54311C` | Accent/active states, primary buttons |

**Additional colors:**

| Role | Value |
|------|-------|
| Off-white (content background) | `#FAF9F7` |
| Pure white (input backgrounds) | `#FFFFFF` |

## Color Mapping

| UI Element | Color |
|------------|-------|
| Page background | `#C6B7A0` |
| Content area background | `#FAF9F7` |
| Sidebar background | `#78798B` |
| Primary text | `#1F1C1A` |
| Secondary text | `#5B5958` |
| Muted text | `#A4A0A1` |
| Active/accent | `#54311C` |
| Sidebar text | `#FAF9F7` |
| Sidebar text (muted) | `#C6B7A0` |
| Borders/dividers | `#8D7C55` at 30% opacity |

## Layout Structure

### Main Chat Interface

```
┌─────────────────────────────────────────────────────────┐
│  Page background (#C6B7A0) - visible as subtle frame    │
│  ┌────────────┬────────────────────────────────────┐    │
│  │            │  Chat header (#FAF9F7)             │    │
│  │  Sidebar   │  ─────────────────────────────────│    │
│  │  (#78798B) │                                    │    │
│  │            │  Messages area (#FAF9F7)           │    │
│  │  Channels  │                                    │    │
│  │  Actions   │                                    │    │
│  │            │  ─────────────────────────────────│    │
│  │            │  Input bar                         │    │
│  └────────────┴────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### Login/Register Pages

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│           Page background (#C6B7A0)                     │
│                                                         │
│           ┌─────────────────────────┐                   │
│           │  Card (#FAF9F7)         │                   │
│           │                         │                   │
│           │  Hatchat 🪓             │                   │
│           │                         │                   │
│           │  Username               │                   │
│           │  [ input ]              │                   │
│           │                         │                   │
│           │  Password               │                   │
│           │  [ input ]              │                   │
│           │                         │                   │
│           │  [ Login ]              │                   │
│           │                         │                   │
│           │  Don't have an account? │                   │
│           │  Register link          │                   │
│           └─────────────────────────┘                   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Component Specifications

### Buttons

**Primary:**
- Background: `#54311C` (Peat Brown)
- Text: `#FAF9F7` (off-white)
- Border radius: `2px`
- Hover: 10% darker

**Secondary:**
- Background: transparent
- Border: 1px `#5B5958` (Stone Cold)
- Text: `#5B5958`
- Border radius: `2px`
- Hover: background `#5B5958` at 10% opacity

**Danger:**
- Keep existing red tones but apply `2px` border radius

### Inputs

- Background: `#FFFFFF`
- Border: 1px `#5B5958` at 50% opacity
- Border radius: `2px`
- Focus state: border becomes `#54311C` (Peat Brown)

### Sidebar Channels

**Active channel:**
- Background: `#54311C` (Peat Brown)
- Text: `#FAF9F7` (off-white)
- Border radius: `2px`

**Inactive channel:**
- Background: transparent
- Text: `#FAF9F7` at 80% opacity
- Hover: background at 10% white opacity

### Message Avatars

- Keep existing colored squares
- Border radius: `2px`

### Modals

- Background: `#FAF9F7` (off-white)
- Border: 1px `#8D7C55` at 30% opacity
- Border radius: `2px`
- Overlay: `#1F1C1A` (Coco's Black) at 50% opacity
- No box-shadow

### Login/Register Card

- Background: `#FAF9F7`
- Border: 1px `#8D7C55` at 30% opacity
- Border radius: `2px`
- No box-shadow
- Max-width: ~400px, centered

## Files to Modify

### `static/style.css`

- Set page background to `#C6B7A0`
- Restyle form as centered card
- Update header (remove salmon color or remove header entirely)
- Update inputs, buttons, links to new palette
- Replace all `border-radius` with `2px`

### `static/chat.css`

- Sidebar: `#3f0e40` → `#78798B`
- Chat header: `#350d36` → `#FAF9F7` with border separator
- Content backgrounds → `#FAF9F7`
- All `border-radius` values → `2px`
- Update button colors (`.btn-primary`, `.btn-secondary`)
- Update modal overlay and styling
- Update active channel styling
- Replace shadows with subtle borders

### `template/login.html`

- Add `<link>` to `style.css`
- Add proper structure: card wrapper, labels, title
- Add link to register page

### `template/register.html`

- Add `<link>` to `style.css`
- Add proper structure: card wrapper, labels, title
- Add link to login page

## Files Unchanged

- `template/chat.html` – Structure is fine, CSS changes only
- Backend code – No changes needed

## Future Considerations

- Typography refinement (separate task)
- Avatar colors could shift toward this palette
- Dark mode variant using the darker colors from the palette
