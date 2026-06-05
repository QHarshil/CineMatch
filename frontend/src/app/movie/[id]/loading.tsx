export default function MovieDetailLoading() {
  return (
    <div className="-mt-16">
      <div className="h-[50vh] min-h-[360px] w-full animate-pulse bg-muted" />
      <div className="relative z-10 mx-auto -mt-32 max-w-4xl px-4 pb-16 lg:px-8">
        <div className="flex flex-col gap-8 sm:flex-row">
          <div className="aspect-[2/3] w-48 shrink-0 animate-pulse bg-muted sm:w-56" />
          <div className="flex flex-1 flex-col gap-4 pt-2">
            <div className="h-10 w-3/4 animate-pulse bg-muted" />
            <div className="h-4 w-1/3 animate-pulse bg-muted" />
            <div className="flex gap-2">
              <div className="h-7 w-20 animate-pulse bg-muted" />
              <div className="h-7 w-20 animate-pulse bg-muted" />
              <div className="h-7 w-20 animate-pulse bg-muted" />
            </div>
            <div className="h-px w-full bg-border" />
            <div className="h-4 w-full animate-pulse bg-muted" />
            <div className="h-4 w-full animate-pulse bg-muted" />
            <div className="h-4 w-2/3 animate-pulse bg-muted" />
          </div>
        </div>
      </div>
    </div>
  );
}
