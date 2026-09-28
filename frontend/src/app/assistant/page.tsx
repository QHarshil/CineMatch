import { Suspense } from "react";
import { AssistantView } from "./assistant-view";

export const metadata = {
  title: "Ask",
  description:
    "A grounded AI assistant that searches the catalog, reads your taste, and recommends only titles it retrieved, with every step shown.",
};

export default function AssistantPage() {
  return (
    <div className="pt-16">
      <Suspense fallback={null}>
        <AssistantView />
      </Suspense>
    </div>
  );
}
