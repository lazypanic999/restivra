# Restivra interface review

Used [avoid-ai-design](https://github.com/funboy322/avoid-ai-design/blob/main/SKILL.md), v0.4.0 at commit `2b7c71836b06cdf6b3d73265b56490c1b75ad5aa`. Profile: dashboard. Actual React/CSS files and the rendered app were reviewed.

## Before

Scanner: **1 P0, 5 P1, 1 P2** across the original source files.

| ID | Finding / location | Assessment |
| --- | --- | --- |
| T1 / P0 | Inter stack in styles.css | Code-certain; no type choice particular to verification records. |
| C2 / P1 | Violet primary buttons | Code-certain and rendered; combined with pastel cards/illustration. |
| C9 / P1 | Very light secondary text | Code-certain and rendered; small labels hard to read. |
| I3 / P1 | Large generic Lucide import | Code-certain; icons often repeated their labels. |
| K3 / P1 | Modal backdrop blur | Contextual, not inherently defective; unnecessary here. |
| SD5 / P1 | Count spans in headings | Scanner false positive about accented words; counts were meaningful. |
| SD4d / P2 | Decorative middle-dot metadata | Code-certain; reduced in interface chrome. |

Rendered judgment: slogan/illustration, equal summary cards and permanent dark sidebar put decoration before backup records. The silhouette could front unrelated SaaS tools. [Before image](screenshots/dashboard-before.jpg).

## Plan and rewrite

The primary artifact is a verification record. Adopted direction: compact administration ledger, project/source rows first, verification runs next, report stages with explicit results. Alternatives considered: a dark terminal and another card dashboard; both add a visual costume to the same task.

Shared palette, Public Sans, spacing, control states and dialogs are in [DESIGN.md](../DESIGN.md). The font and license are bundled locally. Lucide was removed. Green/red/amber express real states. No slogan hero, statistic-card grid, illustration, decorative numbering or gradient remains.

English defaults regardless of browser locale. An explicit Turkish choice persists locally. Navigation, forms, status, errors, warnings, setup, accessible labels, dates and durations follow that choice. User names, SQL and machine JSON reports remain unchanged.

## After

Re-scan: **0 P0, 0 P1, 0 P2**, without ignore comments. A clean scan is not a design guarantee; rendered desktop/mobile views and both languages were checked too.

- Justified: restore records occupy the first screen instead of promotional content.
- Coherent: controls, tables, fonts and tokens are shared across every app view.
- No replacement template: no cream/terracotta, neon terminal or decorative mono labels.
- Consistent: language changes content, not the component/layout system.

Browser checks include initial English, Turkish persistence, localized forms/query errors, actual PostgreSQL reports, JSON download and a narrow viewport. Screenshots accompany this review.

## Dependency note

`npm audit --omit=dev` reports zero runtime dependency vulnerabilities. The Vite development chain reports four high findings rooted in [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf); the advisory lists no patched release. No forced downgrade or audit suppression was applied. Production runs Go and static assets, not Vite; application backup patterns are handled in Go. Build trusted source and keep dev servers on loopback pending an upstream fix.
