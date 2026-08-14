'use client'

import { useEffect } from 'react'
import { Sidebar, Header } from '@/components/layout'
import { useCurrentUser } from '@/hooks'
import { useAuthStore } from '@/stores'
import { useRouter } from 'next/navigation'
import type { UserRole } from '@/types'

const ADMIN_ROLES: UserRole[] = ['admin', 'platform_admin']

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const router = useRouter()
  const { user, isAuthenticated, isLoading } = useCurrentUser()
  const { setLoading } = useAuthStore()

  useEffect(() => {
    setLoading(false)
  }, [setLoading])

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login')
    }
  }, [isLoading, isAuthenticated, router])

  useEffect(() => {
    if (!isLoading && isAuthenticated && user && !ADMIN_ROLES.includes(user.role)) {
      router.push('/dashboard')
    }
  }, [isLoading, isAuthenticated, user, router])

  if (isLoading || !isAuthenticated || !user || !ADMIN_ROLES.includes(user.role)) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header user={user} />
        <main className="flex-1 overflow-y-auto bg-muted/30 p-4 md:p-6">
          {children}
        </main>
      </div>
    </div>
  )
}