export default function HowItWorksLoading() {
  return (
    <div className="min-h-screen pt-32 pb-20 px-4">
      <div className="max-w-3xl mx-auto">
        {/* Hero skeleton */}
        <div className="text-center space-y-6 mb-20">
          <div className="h-3 w-32 bg-surface animate-pulse mx-auto" />
          <div className="h-12 w-3/4 bg-surface animate-pulse mx-auto" />
          <div className="h-5 w-2/3 bg-surface animate-pulse mx-auto" />
        </div>

        {/* Section skeletons */}
        {[1, 2, 3].map((i) => (
          <div key={i} className="mb-20 space-y-4">
            <div className="h-3 w-8 bg-surface animate-pulse" />
            <div className="h-8 w-64 bg-surface animate-pulse" />
            <div className="h-3 w-32 bg-surface animate-pulse" />
            <div className="mt-8 h-32 w-full bg-surface animate-pulse" />
          </div>
        ))}
      </div>
    </div>
  );
}
