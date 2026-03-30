export default function BrowseLoading() {
  return (
    <div className="mx-auto max-w-7xl px-4 lg:px-8 pt-20 pb-12">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-10">
        <div className="h-9 w-32 bg-surface animate-pulse" />
        <div className="h-10 w-full max-w-sm bg-surface animate-pulse" />
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-x-4 gap-y-8">
        {Array.from({ length: 20 }).map((_, i) => (
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
