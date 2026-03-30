import Link from "next/link";

export default function MovieNotFound() {
  return (
    <div className="flex flex-col items-center justify-center pt-32 pb-16 gap-4 px-4">
      <h2 className="font-heading text-2xl font-semibold">Movie not found</h2>
      <p className="text-muted-foreground text-sm">
        The movie you are looking for does not exist or has been removed.
      </p>
      <Link
        href="/browse"
        className="px-6 py-2.5 border border-gold text-gold text-sm hover:bg-gold hover:text-background transition-colors duration-200"
      >
        Browse movies
      </Link>
    </div>
  );
}
