"use client";

export default function BrowseError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 px-4 pb-16 pt-32 text-center">
      <p className="eyebrow text-primary">Error</p>
      <h2 className="font-heading text-2xl font-semibold uppercase tracking-tight">
        Failed to load titles
      </h2>
      <p className="max-w-md font-serif text-muted-foreground">
        The catalog could not be loaded. The backend may be unavailable.
      </p>
      <button
        onClick={reset}
        className="eyebrow border border-border px-6 py-2.5 text-muted-foreground transition-colors duration-200 hover:border-primary hover:text-primary"
      >
        Try again
      </button>
    </div>
  );
}
