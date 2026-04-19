# Repository Guidelines

## Project Structure & Module Organization
- `src/main.tsx` is the app entry; `src/App.tsx` is the root component.
- `src/components/` holds UI components; each uses `Component.tsx` with `Component.module.css`.
- `src/api/` contains API clients, `src/types/` shared types, `src/assets/` app assets, `src/index.css` global styles.
- `public/` stores static files served by Vite; `dist/` is the production build output.
- `e2e/` contains Playwright end-to-end tests (`*.spec.ts`).

## Build, Test, and Development Commands
- `npm install` installs dependencies.
- `npm run dev` starts the Vite dev server with HMR.
- `npm run build` runs TypeScript build (`tsc -b`) and outputs `dist/`.
- `npm run preview` serves the production build locally.
- `npm run lint` runs ESLint across the repo.
- `npm run test` runs Playwright tests headless.
- `npm run test:ui` runs Playwright with the UI runner.
- `npm run test:headed` runs Playwright in headed mode.

## Coding Style & Naming Conventions
- Use TypeScript + React function components with hooks.
- Indentation: 2 spaces; use semicolons and single quotes to match existing files.
- Components use `PascalCase.tsx` and matching `PascalCase.module.css`.
- Hooks should be named `useSomething` in `src/hooks/` when added.
- Types live in `src/types/`; prefer explicit types at module boundaries.

## Testing Guidelines
- Framework: Playwright (`playwright.config.ts`).
- Place tests in `e2e/` and name them `*.spec.ts` (example: `e2e/app.spec.ts`).
- Add or update tests for UI behavior changes; no coverage gate is enforced.

## Commit & Pull Request Guidelines
- Commit history commonly uses Conventional Commits (`feat:`, `fix:`, `refactor:`) with optional scopes; keep subjects short and imperative.
- Non-English commit messages appear, but keep the style consistent within a series.
- PRs should include a concise description, linked issues if applicable, and before/after screenshots for UI changes.
- Call out any API or env changes in the PR description.

## Configuration Tips
- Copy `.env.example` to `.env` and set `VITE_API_BASE` and optional `VITE_API_TOKEN`.
- Vite only exposes env vars prefixed with `VITE_`.
