# CineMatch Frontend

Next.js 16 App Router frontend for CineMatch. Renders the movie browsing, search, and personalised recommendation UI.

## Pages

| Route | Type | Description |
|---|---|---|
| `/` | Static | Landing page with search bar and CTA |
| `/browse` | Dynamic (SSR) | Paginated movie grid fetched from Go API |
| `/search?q=` | Dynamic (SSR) | Title search results |
| `/movie/[id]` | Dynamic (SSR) | Movie detail with poster, metadata, interaction buttons |
| `/for-you` | Client | Personalised recommendations (auth required) |
| `/login` | Client | Magic link sign-in via Supabase Auth |

## Tech stack

- Next.js 16 (App Router, Server Components)
- TypeScript strict mode
- Tailwind CSS v4
- shadcn/ui components (button, input, card, badge, skeleton, separator, dropdown-menu, avatar)
- Supabase Auth (@supabase/ssr) for magic link sign-in
- Go backend API client (`lib/api.ts`)

## Running locally

```bash
cp .env.local.example .env.local   # fill in your Supabase and API values
npm install
npm run dev
```

App available at `http://localhost:3000`.

## Environment variables

| Variable | Description |
|---|---|
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase project URL |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Supabase publishable (anon) key |
| `NEXT_PUBLIC_API_URL` | Go backend URL (default: `http://localhost:8080`) |

## Building for production

```bash
npm run build
npm start
```

## Deploying to Vercel

Connect the `frontend/` directory as the root in Vercel project settings. Set all three env vars above. Vercel auto-detects Next.js and deploys on push.
