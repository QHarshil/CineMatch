"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function MovieError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-24 gap-4 px-4">
      <h2 className="text-xl font-bold">Movie not found</h2>
      <p className="text-muted-foreground text-center max-w-md">
        This movie could not be loaded or does not exist.
      </p>
      <div className="flex gap-3">
        <Button variant="outline" onClick={reset}>
          Try again
        </Button>
        <Link href="/browse">
          <Button>Browse movies</Button>
        </Link>
      </div>
    </div>
  );
}
