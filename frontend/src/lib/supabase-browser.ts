import { createBrowserClient } from "@supabase/ssr";

/**
 * Creates a Supabase client for use in Client Components.
 * Uses the publishable anon key only -- RLS restricts access.
 *
 * Falls back to placeholder values during build-time prerendering
 * (when env vars are absent). The client will fail on actual network
 * calls, but the component tree can still render statically.
 */
export function createSupabaseBrowserClient() {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL ?? "https://placeholder.supabase.co";
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "placeholder";
  return createBrowserClient(url, key);
}
