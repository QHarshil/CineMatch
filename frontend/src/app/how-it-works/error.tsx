"use client";

export default function HowItWorksError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="text-center space-y-4">
        <h2 className="font-heading text-2xl font-bold">
          Something went wrong
        </h2>
        <p className="text-sm text-muted-foreground">
          Could not load the page. Please try again.
        </p>
        <button
          onClick={reset}
          className="border border-border px-5 py-2 text-sm text-foreground hover:border-gold hover:text-gold transition-colors duration-200"
        >
          Try again
        </button>
      </div>
    </div>
  );
}
