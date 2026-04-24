---
name: build-page-mock
description: "Use when creating or revising a page mock, browser-runnable prototype, or interactive demo for this repository's web, setup, onboarding, status, or admin pages, and also when implementing a real product page from an approved mock. Enforces a visible-content contract, final-user-facing content, post-action feedback states, and mock-to-product parity except for real business data and runtime feedback instances inside approved feedback slots."
---

# build-page-mock

Use this skill when the task is to create, revise, review, or implement from a page mock / prototype / interactive demo that users should experience in a browser.

Typical triggers:

- user asks for `页面 Mock` / `页面原型` / `高保真原型` / `可交互 demo`
- user asks for a browser-runnable preview page before the real backend is ready
- user asks to `按 mock 落产品` / `从 mock 生成页面` / `按原型实现最终页面`
- touched files include `docs/draft/*mock*.html`, `web/src/**` preview routes, or `internal/app/daemon/adminui/**` mock pages

## Read first

Read these docs before editing:

- `docs/general/page-mock-guidelines.md`
- `docs/general/web-design-guidelines.md`

## Required execution order

Before editing the page, explicitly lock these five items for yourself:

1. Who is the final user?
2. What single task is this page serving right now?
3. Which information types are allowed to appear on the page?
4. Which information types are not allowed to appear on the page?
5. Which feedback slots are allowed to show success / validation / error / fallback?

Treat this as the page's visible-content contract.

Do not start writing visible page content until this contract is clear.

## Rules

### 1. Treat the mock as a final-user artifact

- The rendered page must show only final-user-facing content.
- Do not render `mock`, `demo`, `prototype`, `wireframe`, `设计说明`, `TODO`, or other internal wording in the page.
- This includes browser title, page header, helper text, placeholders, empty states, notices, and debug panels.
- This rule applies to fake data and demo data too, not only to fixed copy.

### 1.1 Default to fail-closed for visible content

- Only content that clearly belongs to the visible-content contract may appear on screen.
- If you cannot justify why a visible element helps the final user complete the page's current task, remove it.
- Do not keep questionable content just because it is sample data, illustrative data, or easy to reuse.
- When the page is not explicitly for engineering, debugging, or operator diagnosis, treat code, repo paths, internal object names, protocol fields, and implementation notes as disallowed by default.

### 2. Make it runnable, not illustrative

- The mock must run in a browser.
- Static screenshots or dead HTML do not count.
- Standalone HTML is acceptable only if it is genuinely interactive and browser-runnable.
- If the repository already has an app shell or route model that fits the task, prefer integrating there instead of building a disconnected static artifact.

### 3. Fake data is allowed only as real interaction coverage

- Use fake data freely when the backend is absent.
- Cover every user-editable, user-selectable, filterable, searchable, or navigable data surface that the page exposes.
- If the user can reach empty / populated / validation / success / failure states, make those states reachable in the mock.
- Fake data must still obey the visible-content contract.
- Do not use source code, repo paths, internal state names, debug output, or design notes as visible sample content unless the page contract explicitly allows technical content.

### 4. Cover the feedback contract, not just the happy path

- For each key user action, make the relevant feedback states reachable in the mock.
- At minimum, consider the needed subset of:
  - loading
  - success
  - validation error
  - recoverable business error
  - transient system error
  - empty / no permission / expired when applicable
- The mock does not need every exact backend error string.
- The mock must define where feedback appears and what the user can do next.

### 5. Every visible interaction must be real

- Buttons, tabs, dialogs, forms, expanders, navigation, search, filters, sort, pagination, and multi-step flows must all change real page state.
- Do not leave dead controls.
- If a backend action does not exist yet, simulate it with local state, fake services, or deterministic in-browser behavior instead of a no-op.

### 6. Responsive behavior is part of the deliverable

- Verify desktop and mobile.
- Verify width changes and portrait/landscape rotation.
- If layout or navigation should differ by breakpoint or orientation, implement those behaviors in the mock.
- Do not rely on a single static viewport.

### 7. The approved mock is the user-visible contract for the real product

- When implementing the real product from an approved mock, keep user-visible structure, copy, states, interaction paths, feedback slots, and responsive behavior aligned with that mock.
- The main allowed differences are replacing fake business data and replacing mock feedback examples with real runtime feedback instances inside the approved feedback slots.
- Do not add extra user-visible copy during implementation just because the backend, validation, or edge cases are more complicated than the mock.
- Do not invent new user-visible error regions or fallback blocks during implementation.
- If the backend returns an error that the mock did not enumerate exactly, route it through the page's approved generic fallback slot instead of adding a new presentation surface.
- If the user-visible contract must change, update the mock or the canonical guideline first, then update the product.

## Delivery checklist

Before finishing, verify:

1. No user-visible design-intent wording remains.
2. The artifact runs in a browser.
3. All visible controls and flows work.
4. Fake data covers the page's full interactive surface.
5. Key user actions include reachable feedback states, not only the happy path.
6. Generic fallback behavior is defined inside approved feedback slots.
7. Desktop, mobile, and orientation changes remain usable.
8. If implementing from an approved mock, the real product still matches that mock in all user-visible aspects except business data and runtime feedback instances inside approved slots.
9. A visible-content contract exists: final user, current task, allowed information types, disallowed information types, and allowed feedback slots.
10. Every visible string, label, placeholder, sample value, list item, and notice can be justified against that contract.
11. No visible sample data accidentally exposes code, repo structure, internal names, protocol fields, or other engineering-facing content unless that is explicitly the page's user-facing purpose.
12. If any visible content felt questionable, it was removed unless the contract clearly allowed it.

If one of these fails, the mock is not done.
