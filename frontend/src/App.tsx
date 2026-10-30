import { lazy, Suspense } from "react"
import { BrowserRouter, Route, Routes } from "react-router-dom"
import { Toaster } from "sonner"
import { ThemeProvider, useAppTheme } from "@/hooks/ThemeProvider"
import { I18nProvider } from "@/i18n"
import { TooltipProvider } from "@/components/ui/tooltip"
import { FeedbackFab } from "@/components/layout/FeedbackFab"

// Pages are split by route so the landing page never downloads the code
// viewer (highlight.js grammars), markdown/katex or diagram code.
const HomePage = lazy(() => import("@/pages/HomePage").then((m) => ({ default: m.HomePage })))
const RepoPage = lazy(() => import("@/pages/RepoPage").then((m) => ({ default: m.RepoPage })))
const NotFoundPage = lazy(() => import("@/pages/NotFoundPage").then((m) => ({ default: m.NotFoundPage })))

function RouteFallback() {
  return (
    <div className="grid min-h-dvh place-items-center">
      <span className="size-7 animate-spin rounded-full border-2 border-primary/25 border-t-primary" />
    </div>
  )
}

function AppFrame() {
  const { theme } = useAppTheme()
  return (
    <TooltipProvider>
      <BrowserRouter>
        <Suspense fallback={<RouteFallback />}>
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="/repo/:id" element={<RepoPage />} />
            <Route path="*" element={<NotFoundPage />} />
          </Routes>
        </Suspense>
      </BrowserRouter>
      <Toaster
        theme={theme}
        position="bottom-right"
        richColors
        closeButton
        toastOptions={{ className: "font-sans" }}
      />
      <FeedbackFab />
    </TooltipProvider>
  )
}

export default function App() {
  return (
    <I18nProvider>
      <ThemeProvider>
        <AppFrame />
      </ThemeProvider>
    </I18nProvider>
  )
}
