import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { localeMeta, locales, messages, type Locale } from "@/i18n/messages"

const STORAGE_KEY = "code-insight-locale"

type Vars = Record<string, string | number>

type I18nContextValue = {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: string, vars?: Vars) => string
  dir: "ltr" | "rtl"
}

const I18nContext = createContext<I18nContextValue | null>(null)

function normalizeLocale(value: string): Locale | null {
  const lower = value.toLowerCase()
  if (lower.startsWith("zh")) return "zh"
  if (lower.startsWith("es")) return "es"
  if (lower.startsWith("hi")) return "hi"
  if (lower.startsWith("ar")) return "ar"
  if (lower.startsWith("pt")) return "pt"
  if (lower.startsWith("ru")) return "ru"
  if (lower.startsWith("ja")) return "ja"
  if (lower.startsWith("fr")) return "fr"
  if (lower.startsWith("de")) return "de"
  if (lower.startsWith("en")) return "en"
  return null
}

function detectLocale(): Locale {
  if (typeof window === "undefined") return "en"
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored && (locales as readonly string[]).includes(stored)) return stored as Locale
  for (const candidate of navigator.languages ?? [navigator.language]) {
    const locale = normalizeLocale(candidate)
    if (locale) return locale
  }
  return "en"
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(detectLocale)
  const dir = localeMeta[locale].dir

  useEffect(() => {
    const root = document.documentElement
    root.lang = locale
    root.dir = dir
    window.localStorage.setItem(STORAGE_KEY, locale)
  }, [locale, dir])

  const t = useCallback(
    (key: string, vars?: Vars) => {
      const template = messages[locale][key] ?? messages.en[key] ?? key
      if (!vars) return template
      return template.replace(/\{(\w+)\}/g, (match, name: string) => {
        const value = vars[name]
        return value === undefined ? match : String(value)
      })
    },
    [locale],
  )

  const value = useMemo(() => ({ locale, setLocale, t, dir }), [locale, t, dir])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext)
  if (!value) throw new Error("useI18n must be used inside I18nProvider")
  return value
}
