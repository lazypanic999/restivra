---
version: alpha
name: Restivra verification ledger
description: A working record of PostgreSQL restore tests for operators who need to inspect failures and their evidence.
colors:
  ground: "#ffffff"
  surface: "#f2f4f6"
  ink: "#202832"
  muted: "#586472"
  signal: "#234a68"
  rule: "#cbd2d9"
typography:
  display:
    fontFamily: Public Sans
    fontSize: 28px
    fontWeight: 600
    letterSpacing: -0.6px
  body:
    fontFamily: Public Sans
    fontSize: 14px
    lineHeight: 1.5
rounded:
  sm: 3px
  md: 6px
spacing:
  sm: 8px
  md: 16px
  lg: 32px
components:
  button-primary:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ground}"
    rounded: "{rounded.sm}"
    padding: 8px 14px
---

## Overview

Restivra is a PostgreSQL restore verification tool used by developers and small agencies. Its main artifact is a record of a real restore: which archive, which checks, which outcome, and whether temporary resources were removed. The interface follows that record, not a marketing dashboard. Profile: dashboard; full app rewrite while retaining API behavior.

Design review uses [avoid-ai-design](https://github.com/funboy322/avoid-ai-design), version 0.4.0, commit `2b7c71836b06cdf6b3d73265b56490c1b75ad5aa`. The skill is downloaded into the chat's work folder; no third-party skill code is bundled in this product.

## Colors

White is the reading surface. Cool gray separates navigation and column headers. Ink is used for all substantive information; muted text still meets AA on white. The deep blue action color comes from familiar database administration controls and is used for actionable controls only. Green/red/amber express verification state, always with a text label. They are not brand accents.

## Typography

Public Sans 400/600 is shipped locally through @fontsource, including Turkish glyphs. It was chosen for its legibility in forms and dense administrative records. Monospace is reserved for SQL, run identifiers and file fingerprints, not section labels. Numeric columns use tabular figures and right alignment. Labels and headings use sentence case.

## Layout

One compact horizontal navigation, a system-health line and a full-width verification ledger. Overview starts with projects, not a hero or four stat cards. Counts are a short text summary of the actual returned history window. Recent runs sit beneath project rows. Projects, history, setup, forms and reports reuse the same tokens and navigation. Small screens retain horizontal table scrolling inside the ledger.

## Elevation & Depth

Flat data surfaces. A shadow is reserved for modal elevation. No decorative shadows, gradients, orbits, blobs or oversized security illustrations.

## Shapes

Three-pixel corners for controls, six for dialogs. Statuses are text with a small geometric state marker; no chip soup or icon-in-a-rounded-square system.

## Components

Shared Button, Status, ProjectTable, JobTable, Modal and Field primitives. Language control always remains in the main navigation. English is the default regardless of OS/browser locale. A deliberate choice of Turkish persists locally. Dates and interface messages follow the choice; user-entered names and SQL remain unchanged.

## Do's and Don'ts

- Keep the whole app on this contract; no per-page font/palette invention.
- Make failed checks, source archive and cleanup readable before ancillary details.
- Preserve manual runs, edits, scheduling, cancellation, archive confirmation and JSON downloads.
- Do not substitute another template: no cream/terracotta, neon terminal theme, decorative mono labels, broadsheet masthead, slogan hero, equal feature-card grid or decorative numbering.
- Use a symbol only when it communicates an operation or state; controls primarily use explicit text.
- Re-scan source and inspect rendered English/Turkish pages, forms and reports before delivery.
