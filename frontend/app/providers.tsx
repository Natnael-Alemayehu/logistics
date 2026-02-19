'use client'

import { QueryProvider } from '@/lib/query-provider'
import { Toaster } from '@/components/ui/sonner'
import { ErrorBoundary } from '@/components/shared/error-boundary'
import { I18nProvider } from '@/i18n'
import { useOffline } from '@/hooks/use-offline'
import { useOfflineSync } from '@/hooks/use-offline-sync'
import { OfflineIndicator } from '@/components/shared/offline-indicator'

function OfflineProvider({ children }: { children: React.ReactNode }) {
  const { isOffline, pendingOperationsCount } = useOffline()
  const { isSyncing, sync } = useOfflineSync()

  return (
    <>
      <OfflineIndicator
        isOffline={isOffline}
        pendingCount={pendingOperationsCount}
        isSyncing={isSyncing}
        onSync={sync}
      />
      <div className={isOffline ? 'pt-9' : ''}>{children}</div>
    </>
  )
}

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <ErrorBoundary>
      <I18nProvider>
        <QueryProvider>
          <OfflineProvider>
            {children}
          </OfflineProvider>
          <Toaster position="top-right" />
        </QueryProvider>
      </I18nProvider>
    </ErrorBoundary>
  )
}
