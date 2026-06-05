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

Lint and type-check:

```bash
npm run lint
npx tsc --noEmit
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
| `/` | SSR | Landing: product hero with the pipeline typed out as setup steps, a "see it in action" terminal beside a duotone still, a feature grid, and live catalog rows |
| `/browse` | SSR + client | Genre chips, sort dropdown (popular/top-rated/newest/A-Z), 30-per-page pagination |
| `/browse?q=term` | SSR + client | Search results from the Go backend, same grid |
| `/movie/[id]` | SSR | Detail: TMDB backdrop, poster, ratings, genres, overview, interaction buttons, similar titles |
| `/for-you` | client | Personalized recommendations (auth required): "Top Picks", "Because you liked X", and popular rows. Signed-out visitors get demo taste profiles |
| `/how-it-works` | SSR + client | Technical deep-dive: pipeline diagram, live pgvector similarity demo, eval table, tech stack |
| `/login` | client | Supabase magic-link sign-in, 60-second resend cooldown |
| `/auth/callback` | SSR | Exchanges the magic-link code for a session |
| `/api/similar` | API route | Internal: pgvector neighbors for the how-it-works demo |

Movie cards and the detail page show a Film/TV badge from each title's
`media_type`.

## Design system — Atlas

Editorial, light, and futuristic, after the Hermes Agent site: a white canvas
with pale-blue washed sections, hairline-grid framing, and serif display and
body with monospace for the terminal. Tokens are defined CSS-first in
`src/app/globals.css` (Tailwind v4 `@theme`); the bundled `cinematch-design`
skill is the full reference.

**Color (light):**
- Background `#ffffff`, section washes `#e9f0ff`
- Ink text `#1b2440`, muted `#5b6a8f`
- Primary (cornflower) `#2f54ff`; amber `#f5a623` as a sparing spark
- Gold `#c8860b` reserved for star ratings only
- Hairline borders `#d2ddf2`, radius `0.25rem`

**Type (`next/font/google`):**
- Display and headings: Fraunces (serif), frequently uppercase
- Body: Newsreader (serif)
- Terminal, labels, numbers: JetBrains Mono
- App default sans: Inter

**Signature pieces:**
- `useTypewriter` + `TypingText` + `CodeTyper` drive the code/terminal typing
  motif. Both are reduced-motion aware and mirror full text to an `sr-only`
  node, so the animation never costs accessibility or SSR content.
- `.duotone` blue-tinted film stills, `.halftone` print grain, `.eyebrow`
  letter-spaced labels.
- `ScrollRow`, `MovieCard`, `SearchBar` (live TMDB-thumbnail dropdown),
  `InteractionButtons`, `Toast`, plus shadcn/ui (Base UI) primitives.

**Conventions:**
- Near-sharp corners, flat hairline borders, no glow.
- Lucide icons only, no emoji; no em dashes in copy.
- 200ms ease transitions; skeleton-shimmer loading states.
- WCAG AA contrast; `prefers-reduced-motion` honored.

## Client-side protections

- Search is debounced to 500ms.
- Interaction buttons disable briefly after each click.
- A toast surfaces on API rate limit (429).
- The magic-link button has a 60-second cooldown.
