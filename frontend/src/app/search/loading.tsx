export default function SearchLoading() {
  return (
    <div className="mx-auto max-w-7xl px-4 pb-12 pt-24 lg:px-8">
      <div className="mb-10 h-9 w-64 animate-pulse bg-muted" />
      <div className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: 10 }).map((_, i) => (
          <div key={i} className="flex flex-col gap-2">
            <div className="aspect-[2/3] animate-pulse bg-muted" />
            <div className="h-4 w-3/4 animate-pulse bg-muted" />
            <div className="h-3 w-1/2 animate-pulse bg-muted" />
          </div>
        ))}
      </div>
    </div>
  );
}
