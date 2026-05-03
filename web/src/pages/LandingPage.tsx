import { useEffect, useState } from "react";
import {
  LandingNav,
  HeroSection,
  ProblemSolution,
  FeaturesSection,
  SmartScoreSection,
  HowItWorks,
  StatsSection,
  FinalCTA,
  LandingFooter,
} from "@/components/landing";

function useAppVersion() {
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    fetch("/healthz")
      .then((r) => r.json())
      .then((d) => {
        if (d?.version) setVersion(d.version);
      })
      .catch(() => {});
  }, []);
  return version;
}

export function LandingPage() {
  const version = useAppVersion();

  return (
    <div
      dir="rtl"
      className="min-h-screen overflow-x-hidden bg-background text-foreground"
    >
      <LandingNav />
      <HeroSection />
      <ProblemSolution />
      <FeaturesSection />
      <SmartScoreSection />
      <HowItWorks />
      <StatsSection />
      <FinalCTA />
      <LandingFooter version={version} />
    </div>
  );
}
