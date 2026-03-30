export default function MovieDetailLoading() {
  return (
    <div className="-mt-14">
      <div className="w-full h-[50vh] min-h-[360px] bg-surface animate-pulse" />
      <div className="mx-auto max-w-4xl px-4 lg:px-8 -mt-32 relative z-10 pb-16">
        <div className="flex flex-col sm:flex-row gap-8">
          <div className="w-48 sm:w-56 shrink-0 aspect-[2/3] bg-surface animate-pulse" />
          <div className="flex flex-col gap-4 flex-1 pt-2">
            <div className="h-10 w-3/4 bg-surface animate-pulse" />
            <div className="h-4 w-1/3 bg-surface animate-pulse" />
            <div className="flex gap-2">
              <div className="h-7 w-20 bg-surface animate-pulse" />
              <div className="h-7 w-20 bg-surface animate-pulse" />
              <div className="h-7 w-20 bg-surface animate-pulse" />
            </div>
            <div className="h-px w-full bg-border" />
            <div className="h-4 w-full bg-surface animate-pulse" />
            <div className="h-4 w-full bg-surface animate-pulse" />
            <div className="h-4 w-2/3 bg-surface animate-pulse" />
          </div>
        </div>
      </div>
    </div>
  );
}
