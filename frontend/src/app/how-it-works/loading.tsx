export default function HowItWorksLoading() {
  return (
    <div className="min-h-screen px-4 pb-20 pt-32">
      <div className="mx-auto max-w-3xl">
        {/* Hero skeleton */}
        <div className="mb-20 space-y-6 text-center">
          <div className="mx-auto h-3 w-32 animate-pulse bg-muted" />
          <div className="mx-auto h-12 w-3/4 animate-pulse bg-muted" />
          <div className="mx-auto h-5 w-2/3 animate-pulse bg-muted" />
        </div>

        {/* Section skeletons */}
        {[1, 2, 3].map((i) => (
          <div key={i} className="mb-20 space-y-4">
            <div className="h-3 w-8 animate-pulse bg-muted" />
            <div className="h-8 w-64 animate-pulse bg-muted" />
            <div className="h-3 w-32 animate-pulse bg-muted" />
            <div className="mt-8 h-32 w-full animate-pulse bg-muted" />
          </div>
        ))}
      </div>
    </div>
  );
}
