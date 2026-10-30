import { BrowserRouter, Route, Routes } from "react-router-dom"
import { Toaster } from "sonner"
import { ThemeProvider, useAppTheme } from "@/hooks/ThemeProvider"
import { I18nProvider } from "@/i18n"
import { TooltipProvider } from "@/components/ui/tooltip"
import { HomePage } from "@/pages/HomePage"
import { RepoPage } from "@/pages/RepoPage"
import { NotFoundPage } from "@/pages/NotFoundPage"
import { FeedbackFab } from "@/components/layout/FeedbackFab"

function AppFrame() {
  const { theme } = useAppTheme()
  return (
    <TooltipProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/repo/:id" element={<RepoPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
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
