/**
 * The publishable Supabase settings both client factories use. Builds without
 * these env vars (CI) still prerender; requests then fail and pages show their
 * empty states.
 */
export const SUPABASE_URL = process.env.NEXT_PUBLIC_SUPABASE_URL ?? "https://placeholder.supabase.co";
export const SUPABASE_ANON_KEY = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ?? "placeholder";
