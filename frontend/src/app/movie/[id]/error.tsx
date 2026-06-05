"use client";

import Link from "next/link";

export default function MovieError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 px-4 pb-16 pt-32 text-center">
      <p className="eyebrow text-primary">Error</p>
      <h2 className="font-heading text-2xl font-semibold uppercase tracking-tight">
        Could not load title
      </h2>
      <p className="max-w-md font-serif text-muted-foreground">
        This title could not be loaded or does not exist.
      </p>
      <div className="mt-2 flex gap-3">
        <button
          onClick={reset}
          className="eyebrow border border-border px-6 py-2.5 text-muted-foreground transition-colors duration-200 hover:border-primary hover:text-primary"
        >
          Try again
        </button>
        <Link
          href="/browse"
          className="eyebrow border border-primary px-6 py-2.5 text-primary transition-colors duration-200 hover:bg-primary hover:text-primary-foreground"
        >
          Browse titles
        </Link>
      </div>
    </div>
  );
}
