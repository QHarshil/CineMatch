"use client";

export default function BrowseError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4 px-4">
      <h2 className="font-heading text-2xl font-semibold">
        Failed to load movies
      </h2>
      <p className="text-muted-foreground text-sm text-center max-w-md">
        The movie catalog could not be loaded. The backend may be unavailable.
      </p>
      <button
        onClick={reset}
        className="px-6 py-2.5 border border-border text-sm text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors duration-200"
      >
        Try again
      </button>
    </div>
  );
}
