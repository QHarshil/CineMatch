# CineMatch Frontend

Next.js app for movie and TV browsing, search, and recommendations. Server
components render content pages (SSR for SEO and first paint); client components
handle auth, interactions, search, and the recommendation feed.

## Running locally

```bash
cd frontend
npm install
npm run dev
# http://localhost:3000
```

Build and run production:

```bash
npm run build
npm start
```

Lint, type-check, and test:

```bash
npm run lint
npm run format:check
npx tsc --noEmit
npm test          # Vitest + Testing Library render tests
```

## Environment variables

Create `frontend/.env.local` from the example:

```bash
cp .env.local.example .env.local
```

| Variable | Description |
|----------|-------------|
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase project URL (e.g. `https://xyz.supabase.co`) |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Supabase publishable anon key (RLS restricts what it can access) |
| `NEXT_PUBLIC_API_URL` | Go backend URL (default `http://localhost:8080`) |

Only `NEXT_PUBLIC_*` vars reach the browser. The Supabase service key, TMDB
token, OpenAI key, and OMDb key all stay in the Go backend.

## Pages

| Route | Rendering | Description |
|-------|-----------|-------------|
| `/` | SSR + client | Landing: Vanta hero with a prompt that hands off to the assistant, a live `/discover` terminal, a scroll story of one agent run, measured eval results, and catalog rows |
| `/assistant` | client | The grounded assistant: streamed tool steps, pick cards with a confidence meter and like/dislike, and a sticky agent trace with the audit run ID. Guest sign-in for visitors |
| `/browse` | SSR + client | Genre chips, sort dropdown (popular/top-rated/newest/A-Z), 30-per-page pagination |
| `/browse?q=term` | SSR + client | Natural-language results from `/discover` (hybrid retrieval), labeled with how they matched |
| `/movie/[id]` | SSR | Detail: TMDB backdrop, poster, ratings, genres, overview, interaction buttons, similar titles |
| `/for-you` | client | Personalized recommendations (auth required): "Top Picks" with the nearest liked title and ranker factors under each, "Because you liked X", and popular rows. Signed-out visitors get demo taste profiles |
| `/how-it-works` | SSR + client | Technical deep-dive: pipeline diagram, live pgvector similarity demo, ranker eval, the AI layer with retrieval and agent eval tables, tech stack |
| `/search?q=term` | redirect | Redirects to `/browse?q=term` |
| `/login` | client | Supabase magic-link sign-in, 60-second resend cooldown |
| `/auth/callback` | SSR | Exchanges the magic-link code for a session |
| `/api/similar` | API route | Internal: pgvector neighbors for the how-it-works demo |

Movie cards and the detail page show a Film/TV badge from each title's
`media_type`.

## Design

A light editorial look: white canvas, pale-blue washed sections, hairline-grid
framing, Fraunces and Newsreader serifs, and JetBrains Mono for data and
terminals. Tokens live in `src/app/globals.css` (Tailwind v4 `@theme`), on top
of shadcn/ui (Base UI) primitives and Lucide icons.

## Motion

All motion lives in `src/components/motion/` and runs only under
`(prefers-reduced-motion: no-preference)`. Markup renders complete first, so
content reads the same with JavaScript off.

- **Lenis** (`smooth-scroll.tsx`) smooths page scroll on GSAP's ticker, so
  ScrollTrigger reads the position Lenis renders. Nested scroll areas opt out
  with `data-lenis-prevent`.
- **GSAP**: `Reveal`, `SplitHeading` (SplitText line reveal), `CountUp`,
  `ScrambleCycle` (ScrambleTextPlugin), and the pinned assistant story on the
  landing page.
- **React Bits**: `SplitHeading` and `SpotlightCard` are adapted from React
  Bits. The spotlight writes pointer position to CSS variables, so the mouse
  never triggers a re-render.
- **Vanta** (`vanta-net.tsx`): the hero's NET effect. Vanta and three.js r134
  load on demand, render only while visible, and stay off below 768px.

## Assistant client

`src/lib/assistant-stream.ts` is a dependency-free, typed parser for the
`/assistant` event stream. It validates each event's shape and ignores unknown
ones. The eval runner in `eval/ai` imports the same file.
`src/lib/assistant-session.ts` folds events into UI state as a pure reducer,
and `src/hooks/use-assistant.ts` owns the stream, cancellation, and quota.

## Client-side protections

- Search is debounced to 500ms.
- Interaction buttons disable briefly after each click.
- A toast surfaces on API rate limit (429).
- The magic-link button has a 60-second cooldown.
