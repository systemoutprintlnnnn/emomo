# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emomo is an AI-powered meme search engine frontend built with React 19, TypeScript, and Vite. It features semantic search, a responsive meme grid, and modal detail views with copy/download actions.

## Commands

- `npm run dev` - Start Vite dev server with HMR
- `npm run build` - TypeScript build + Vite production build to `dist/`
- `npm run lint` - Run ESLint
- `npm run test` - Run Playwright e2e tests headless
- `npm run test:ui` - Run Playwright with UI runner
- `npm run test:headed` - Run Playwright in headed browser mode

## Architecture

### Entry Flow
`index.html` -> `src/main.tsx` -> `src/App.tsx` (state hub) -> child components

### Key Directories
- `src/components/` - React components with co-located CSS modules (`Component.tsx` + `Component.module.css`)
- `src/api/index.ts` - API client functions (searchMemes, getMemes, getMeme, getCategories)
- `src/types/index.ts` - TypeScript interfaces (Meme, SearchResponse, etc.)
- `e2e/` - Playwright test specs (`*.spec.ts`)

### State Management
Uses React hooks only (useState, useEffect, useCallback, useMemo, useRef). App.tsx is the state hub managing memes, search state, loading states, and modal state. No Redux or external state library.

### Styling
- CSS Modules for component-scoped styles (import as `styles` object)
- Global CSS variables defined in `src/index.css` for theming
- Framer Motion for animations

### API Configuration
Environment variables (copy `.env.example` to `.env`):
- `VITE_API_BASE` - Backend API URL (default: `http://localhost:8080/api/v1`)
- `VITE_API_TOKEN` - Optional Bearer token for auth

Demo data fallback exists in App.tsx for offline development.

## Code Conventions

- 2 spaces indentation, semicolons, single quotes
- Components: PascalCase (`MemeCard.tsx` with `MemeCard.module.css`)
- Hooks: `useSomething` pattern in `src/hooks/` when added
- Commits: Conventional Commits format (`feat:`, `fix:`, `refactor:`)
