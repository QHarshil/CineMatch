"use client";

import { Button } from "@/components/ui/button";

export default function BrowseError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-24 gap-4 px-4">
      <h2 className="text-xl font-bold">Failed to load movies</h2>
      <p className="text-muted-foreground text-center max-w-md">
        The movie catalog could not be loaded. The backend may be unavailable.
      </p>
      <Button onClick={reset}>Try again</Button>
    </div>
  );
}
