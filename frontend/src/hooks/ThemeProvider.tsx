import { createContext, useContext, type ReactNode } from "react"
import { useTheme, type Theme } from "@/hooks/useTheme"

type ThemeContextValue = {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggle: () => void
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export function ThemeProvider({ children }: { children: ReactNode }) {
  const value = useTheme()
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useAppTheme(): ThemeContextValue {
  const context = useContext(ThemeContext)
  if (!context) throw new Error("useAppTheme must be used inside ThemeProvider")
  return context
}
