import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function MovieNotFound() {
  return (
    <div className="flex flex-col items-center justify-center py-24 gap-4 px-4">
      <h2 className="text-xl font-bold">Movie not found</h2>
      <p className="text-muted-foreground">
        The movie you are looking for does not exist or has been removed.
      </p>
      <Link href="/browse">
        <Button>Browse movies</Button>
      </Link>
    </div>
  );
}
