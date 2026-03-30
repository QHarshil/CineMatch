# CineMatch Frontend

Next.js app that renders the movie browsing, search, and recommendation UI. Server components handle movie pages (SSR for SEO and initial load speed), client components handle interactive bits like auth, interactions, and the recommendation feed.

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

Lint:

```bash
npm run lint
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

Only `NEXT_PUBLIC_*` vars are exposed to the browser. The Supabase service key and all other secrets stay in the Go backend.

## Pages

| Route | Rendering | Description |
|-------|-----------|-------------|
| `/` | SSR | Landing page with featured movie backdrop, trending/top-rated scroll rows, search bar |
| `/browse` | SSR + client | Genre filter chips, sort dropdown (popular/top-rated/newest/A-Z), 30-per-page pagination |
| `/browse?q=term` | SSR + client | Search results from the Go backend, same grid layout as browse |
| `/movie/[id]` | SSR | Movie detail: full-width TMDB backdrop, poster, rating, genres, overview, interaction buttons, similar movies row |
| `/for-you` | client | Personalized recommendations (auth required). Shows "Because you liked X" rows, top picks, and popular movies. Unauthenticated users see demo taste profiles. |
| `/how-it-works` | SSR + client | Technical deep-dive: pipeline diagram, interactive vector search demo, eval results, tech stack |
| `/login` | client | Magic link sign-in via Supabase. 60-second cooldown between sends. |
| `/auth/callback` | SSR | Handles the magic link redirect, exchanges the code for a session |
| `/api/similar` | API route | Internal: fetches similar movies via pgvector for the how-it-works demo |

## Design system

The visual language is inspired by [Mubi](https://mubi.com) -- editorial, restrained, dark-first.

**Colors:**
- Background: `#101012`
- Surface (cards, inputs): `#18181B`
- Gold accent: `#D4A843` (CTAs, ratings, active states)
- Text: `#FAFAFA`
- Muted text: `#A1A1AA`

**Typography:**
- Headings: Cormorant (serif), loaded via `next/font/google`
- Body: Inter (sans-serif)

**Components:**
- Base UI from shadcn/ui (button, input, skeleton, dropdown-menu, etc.)
- Custom components: `ScrollRow` (CSS scroll-snap with chevron navigation), `MovieCard`, `SearchBar` (live dropdown with TMDB thumbnails), `InteractionButtons` (Lucide icons), `Toast`
- No third-party carousel or slider libraries

**Interaction patterns:**
- Square corners (0rem radius), no box shadows
- 200ms ease-out transitions on hover/focus
- Movie cards scale to 1.03 on hover
- Scroll rows have edge-fade gradients and arrow buttons on hover
- Skeleton loading states on all pages (surface-colored pulse)

**Client-side protections:**
- Search debounced to 500ms
- Interaction buttons disabled for 1 second after click
- Toast notification on API rate limit (429)
- Magic link button has a 60-second cooldown
