import { BrowserRouter, Route, Routes } from "react-router-dom"
import { Toaster } from "sonner"
import { ThemeProvider, useAppTheme } from "@/hooks/ThemeProvider"
import { TooltipProvider } from "@/components/ui/tooltip"
import { HomePage } from "@/pages/HomePage"
import { RepoPage } from "@/pages/RepoPage"
import { NotFoundPage } from "@/pages/NotFoundPage"

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
    </TooltipProvider>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <AppFrame />
    </ThemeProvider>
  )
}
