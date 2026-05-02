import { useEffect, useState } from "react";

export function useAppVersion(): string | null {
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
