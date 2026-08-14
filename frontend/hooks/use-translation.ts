'use client'

import { useTranslations } from 'next-intl'
import { useLocale } from '@/i18n'

export function useTranslation() {
  const t = useTranslations()
  const { locale, setLocale } = useLocale()

  return {
    t,
    locale,
    setLocale,
  }
}
