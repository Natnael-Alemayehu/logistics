'use client'

import { useState, useEffect, createContext, useContext, ReactNode } from 'react'
import { IntlProvider } from 'next-intl'
import { locales, Locale, defaultLocale } from './config'

type Messages = typeof import('./en/messages.json')

interface I18nContextType {
  locale: Locale
  setLocale: (locale: Locale) => void
}

const I18nContext = createContext<I18nContextType | null>(null)

async function getMessages(locale: Locale): Promise<Messages> {
  const messages = await import(`./${locale}/messages.json`)
  return messages.default
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale)
  const [messages, setMessages] = useState<Messages | null>(null)

  useEffect(() => {
    const stored = localStorage.getItem('locale') as Locale | null
    if (stored && locales.includes(stored)) {
      setLocaleState(stored)
    }
  }, [])

  useEffect(() => {
    getMessages(locale).then(setMessages)
  }, [locale])

  const setLocale = (newLocale: Locale) => {
    setLocaleState(newLocale)
    localStorage.setItem('locale', newLocale)
  }

  if (!messages) {
    return null
  }

  return (
    <I18nContext.Provider value={{ locale, setLocale }}>
      <IntlProvider locale={locale} messages={messages}>
        {children}
      </IntlProvider>
    </I18nContext.Provider>
  )
}

export function useLocale() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useLocale must be used within an I18nProvider')
  }
  return context
}
