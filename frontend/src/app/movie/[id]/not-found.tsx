import Link from "next/link";

export default function MovieNotFound() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 px-4 pb-16 pt-32 text-center">
      <p className="eyebrow text-primary">404</p>
      <h2 className="font-heading text-2xl font-semibold uppercase tracking-tight">
        Title not found
      </h2>
      <p className="max-w-md font-serif text-muted-foreground">
        The title you are looking for does not exist or has been removed.
      </p>
      <Link
        href="/browse"
        className="eyebrow mt-2 border border-primary px-6 py-2.5 text-primary transition-colors duration-200 hover:bg-primary hover:text-primary-foreground"
      >
        Browse titles
      </Link>
    </div>
  );
}
