import { useContext } from "react";
import { ThemeContext } from "@/lib/theme-context";

export function useTheme() {
  return useContext(ThemeContext);
}
