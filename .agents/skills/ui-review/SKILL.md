---
name: ui-review
description: >-
  Expert user interface, design, usability, responsiveness, and accessibility review.
  Use to audit changes affecting visual layout, user journeys, interaction states,
  contrast, or accessibility after verification confirms the tree builds.
---

# UI / UX Review Skill

This skill acts as an expert designer, usability auditor, and accessibility reviewer. It evaluates changes across different domain targets (Web, Desktop/Games, Terminal/CLI, or Headless) to guarantee high visual quality and seamless user experience.

Run after verification, as ordered by the managed workflow. If the change cannot
alter rendered output, return an explicit not-applicable verdict without launching
the application.

---

## 1. Domain Detection & Scope

Determine the project's UI domain from `AGENTS.md` (or inspect repository files):

```mermaid
flowchart TD
    A["Inspect Project Target"] --> B{"UI Domain"}
    B -->|Web App| C["Web Checklist: Viewports, States, WCAG AA, A11y"]
    B -->|Desktop / 2D Game| D["Game/App Checklist: Readability, Input Latency, HUD, Contrast"]
    B -->|Terminal / CLI| E["CLI Checklist: Layout, ANSI Colors, Error Readability"]
    B -->|Headless / API / Backend| F["Record Explicit N/A Justification"]
```

---

## 2. Review Checklists by Domain

### A. Web Applications (Desktop, Tablet, Mobile)
1. **Responsive Viewports**:
   - Mobile (~375px–420px), Tablet (~768px), Desktop (~1280px+).
   - No unexpected horizontal scrolling, text clipping, or overlapping controls.
2. **Interactive States Matrix**:
   - Default, Loading (skeletons / spinners), Empty (no items state), Error (clear message + recovery action), Hover, Active, Disabled, Focus.
3. **Accessibility (a11y) & Usability**:
   - WCAG 2.1 AA color contrast for all text and interactive icons.
   - Visible focus indicator for keyboard navigation (Tab / Shift-Tab).
   - Semantic HTML elements (`<button>`, `<main>`, `<nav>`, `aria-label` when icon-only).
   - Adequate touch/tap target size (minimum 44x44px or 48x48px on mobile).

### B. Desktop Applications & Fixed-Viewport Games (e.g. Go/Ebitengine)
1. **On-Screen Readability & Visual Hierarchy**:
   - High contrast between foreground sprites/vectors and dark or textured background.
   - Clear font rendering for scoreboards, HUD indicators, version strings, and menus.
2. **Input Responsiveness & Frame Smoothness**:
   - Keyboard, mouse, or controller latency and responsive handling.
   - Clean transitions between play, pause, game over, and restart states.
3. **Audio Sync & Polish**:
   - Sound effects trigger appropriately without clipping, popping, or lag.
4. **Viewport / Scale Stability**:
   - Fixed aspect ratios or full-screen resize handling without tearing.

### C. Terminal / CLI Tools
1. **Output Formatting & Alignment**:
   - Tables, indentation, progress bars, and structured terminal output.
2. **Terminal Theme Compatibility**:
   - ANSI color codes readable on both dark and light terminal backgrounds.
   - Clean stderr vs stdout separation.

### D. Headless / Pure Backend / Internal Refactor (No UI Impact)
- Explicitly record that UI review is **Not Applicable** and state the concrete reason (e.g. *"Changes are strictly internal data structures / CLI flags without visual presentation impact"*). Never skip UI review silently.

---

## 3. UI Review Report Output Template

When UI review is completed, generate a markdown report:

```markdown
### UI Review Report
- **UI Domain**: [Responsive Web | Desktop Game | CLI Tool | N/A]
- **Target Inspected**: `<component, route, or screen>`
- **Visual Evidence / Run Method**: `<how change was exercised / screenshots>`

#### Evaluation Matrix
| Criterion | Status | Observations |
|---|---|---|
| Visual Polish & Typography | [PASS / FAIL / NA] | Clear hierarchy, no text overlap |
| Responsive Layout / Scaling | [PASS / FAIL / NA] | Verified at desktop and mobile widths |
| Interactive States & Contrast | [PASS / FAIL / NA] | Loading, error, and hover states verified |
| Accessibility / Input Ergonomics | [PASS / FAIL / NA] | Keyboard navigability & high contrast |

- **Verdict**: [APPROVED | CHANGES_REQUESTED]
- **Actionable Findings**: (List blocking UI issues if any)
```
