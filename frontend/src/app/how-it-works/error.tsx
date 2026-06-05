"use client";

export default function HowItWorksError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="space-y-4 text-center">
        <p className="eyebrow text-primary">Error</p>
        <h2 className="font-heading text-2xl font-semibold uppercase tracking-tight">
          Something went wrong
        </h2>
        <p className="font-serif text-muted-foreground">
          Could not load the page. Please try again.
        </p>
        <button
          onClick={reset}
          className="eyebrow border border-border px-5 py-2.5 text-foreground transition-colors duration-200 hover:border-primary hover:text-primary"
        >
          Try again
        </button>
      </div>
    </div>
  );
}
