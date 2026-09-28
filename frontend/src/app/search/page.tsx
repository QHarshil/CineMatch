import { redirect } from "next/navigation";

export default async function SearchPage({ searchParams }: { searchParams: Promise<{ q?: string }> }) {
  const { q } = await searchParams;
  const query = q?.trim();
  redirect(query ? `/browse?q=${encodeURIComponent(query)}` : "/browse");
}
