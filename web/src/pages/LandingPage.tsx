import { useAppVersion } from "@/hooks/useAppVersion";
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
