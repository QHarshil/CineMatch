export default function BrowseLoading() {
  return (
    <div className="mx-auto max-w-7xl px-4 pb-12 pt-24 lg:px-8">
      <div className="mb-6 h-7 w-28 animate-pulse bg-muted" />

      {/* Filter bar skeleton */}
      <div className="mb-8 flex gap-2">
        {[56, 72, 64, 80, 60, 76, 68, 72].map((w, i) => (
          <div
            key={i}
            className="h-8 shrink-0 animate-pulse bg-muted"
            style={{ width: `${w}px` }}
          />
        ))}
      </div>

      {/* Grid skeleton */}
      <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="flex flex-col gap-2">
            <div className="aspect-[2/3] animate-pulse bg-muted" />
            <div className="h-4 w-3/4 animate-pulse bg-muted" />
            <div className="h-3 w-1/3 animate-pulse bg-muted" />
          </div>
        ))}
      </div>
    </div>
  );
}
