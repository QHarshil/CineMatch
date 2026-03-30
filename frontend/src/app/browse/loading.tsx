export default function BrowseLoading() {
  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <div className="h-7 w-28 bg-surface animate-pulse mb-6" />

      {/* Filter bar skeleton */}
      <div className="flex gap-2 mb-8">
        {[56, 72, 64, 80, 60, 76, 68, 72].map((w, i) => (
          <div
            key={i}
            className="h-8 bg-surface animate-pulse shrink-0"
            style={{ width: `${w}px` }}
          />
        ))}
      </div>

      {/* Grid skeleton */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-5">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="flex flex-col gap-2">
            <div className="aspect-[2/3] bg-surface animate-pulse" />
            <div className="h-4 w-3/4 bg-surface animate-pulse" />
            <div className="h-3 w-1/3 bg-surface animate-pulse" />
          </div>
        ))}
      </div>
    </div>
  );
}
