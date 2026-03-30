"use client";

import Link from "next/link";

export default function MovieError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4 px-4">
      <h2 className="font-heading text-2xl font-semibold">
        Could not load movie
      </h2>
      <p className="text-muted-foreground text-sm text-center max-w-md">
        This movie could not be loaded or does not exist.
      </p>
      <div className="flex gap-3">
        <button
          onClick={reset}
          className="px-6 py-2.5 border border-border text-sm text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200"
        >
          Try again
        </button>
        <Link
          href="/browse"
          className="px-6 py-2.5 border border-gold text-gold text-sm hover:bg-gold hover:text-background transition-colors duration-200"
        >
          Browse movies
        </Link>
      </div>
    </div>
  );
}
