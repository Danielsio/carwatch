import { lazy, Suspense } from "react";
import { useAppVersion } from "@/hooks/useAppVersion";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { LandingNav, HeroSection } from "@/components/landing";

const ProblemSolution = lazy(() =>
  import("@/components/landing/ProblemSolution").then((m) => ({ default: m.ProblemSolution })),
);
const FeaturesSection = lazy(() =>
  import("@/components/landing/FeaturesSection").then((m) => ({ default: m.FeaturesSection })),
);
const SmartScoreSection = lazy(() =>
  import("@/components/landing/SmartScoreSection").then((m) => ({ default: m.SmartScoreSection })),
);
const HowItWorks = lazy(() =>
  import("@/components/landing/HowItWorks").then((m) => ({ default: m.HowItWorks })),
);
const StatsSection = lazy(() =>
  import("@/components/landing/StatsSection").then((m) => ({ default: m.StatsSection })),
);
const FinalCTA = lazy(() =>
  import("@/components/landing/FinalCTA").then((m) => ({ default: m.FinalCTA })),
);
const LandingFooter = lazy(() =>
  import("@/components/landing/LandingFooter").then((m) => ({ default: m.LandingFooter })),
);

export function LandingPage() {
  const version = useAppVersion();
  useDocumentTitle();

  return (
    <div
      dir="rtl"
      className="min-h-screen overflow-x-hidden bg-background text-foreground"
    >
      <LandingNav />
      <HeroSection />
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <ProblemSolution />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <FeaturesSection />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <SmartScoreSection />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <HowItWorks />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <StatsSection />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <FinalCTA />
      </Suspense>
      <Suspense fallback={<div className="min-h-[200px]" />}>
        <LandingFooter version={version} />
      </Suspense>
    </div>
  );
}
