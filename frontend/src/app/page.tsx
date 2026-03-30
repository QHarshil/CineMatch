import Link from "next/link";
import { SearchBar } from "@/components/search-bar";
import { Button } from "@/components/ui/button";

export default function HomePage() {
  return (
    <div className="flex flex-col items-center justify-center gap-8 py-24 px-4 text-center">
      <div className="flex flex-col gap-3 max-w-2xl">
        <h1 className="text-4xl sm:text-5xl font-bold tracking-tight">
          Find your next favourite movie
        </h1>
        <p className="text-muted-foreground text-lg">
          CineMatch uses a two-stage ML pipeline to surface movies you will
          actually enjoy, not just what is popular.
        </p>
      </div>

      <SearchBar />

      <div className="flex gap-3">
        <Link href="/browse">
          <Button variant="outline" size="lg">
            Browse all
          </Button>
        </Link>
        <Link href="/for-you">
          <Button size="lg">Get recommendations</Button>
        </Link>
      </div>
    </div>
  );
}
