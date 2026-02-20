'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useTranslations } from 'next-intl'
import { cn } from '@/lib/utils'
import { useUIStore } from '@/stores'
import { useAuthStore } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Separator } from '@/components/ui/separator'
import {
  LayoutDashboard,
  Package,
  Users,
  Truck,
  Settings,
  BarChart3,
  X,
} from 'lucide-react'
import type { UserRole } from '@/types'

interface NavItem {
  titleKey: string
  href: string
  icon: React.ComponentType<{ className?: string }>
  roles?: UserRole[]
}

const navItems: NavItem[] = [
  {
    titleKey: 'dashboard.title',
    href: '/dashboard',
    icon: LayoutDashboard,
  },
  {
    titleKey: 'shipments.title',
    href: '/shipments',
    icon: Package,
  },
  {
    titleKey: 'drivers.title',
    href: '/drivers',
    icon: Users,
    roles: ['admin', 'fleet_manager', 'dispatcher'],
  },
  {
    titleKey: 'vehicles.title',
    href: '/vehicles',
    icon: Truck,
    roles: ['admin', 'fleet_manager'],
  },
  {
    titleKey: 'reports.title',
    href: '/reports',
    icon: BarChart3,
    roles: ['admin', 'fleet_manager'],
  },
  {
    titleKey: 'settings.title',
    href: '/settings',
    icon: Settings,
    roles: ['admin'],
  },
]

const adminNavItems: NavItem[] = [
  {
    titleKey: 'admin.users',
    href: '/admin/users',
    icon: Users,
    roles: ['admin', 'platform_admin'],
  },
]

const ADMIN_ROLES: UserRole[] = ['admin', 'platform_admin']

function hasRequiredRole(userRole: UserRole, allowedRoles?: UserRole[]): boolean {
  if (!allowedRoles) return true
  return allowedRoles.includes(userRole)
}

export function Sidebar() {
  const t = useTranslations()
  const pathname = usePathname()
  const { sidebarOpen, setSidebarOpen, sidebarCollapsed } = useUIStore()
  const { user } = useAuthStore()

  const NavContent = () => (
    <div className="flex h-full flex-col">
      <div className="flex h-16 items-center justify-between border-b px-4">
        <Link href="/dashboard" className="flex items-center gap-2 font-bold">
          <Package className="h-6 w-6 text-primary" />
          {!sidebarCollapsed && <span>Logistics</span>}
        </Link>
        <Button
          variant="ghost"
          size="icon"
          className="lg:hidden"
          onClick={() => setSidebarOpen(false)}
        >
          <X className="h-5 w-5" />
        </Button>
      </div>
      <ScrollArea className="flex-1 px-3 py-4">
        <nav className="space-y-2">
          {navItems
            .filter((item) => user && hasRequiredRole(user.role, item.roles))
            .map((item) => (
              <Link
                key={item.href}
                href={item.href}
                onClick={() => setSidebarOpen(false)}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  pathname === item.href || pathname.startsWith(item.href + '/')
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )}
              >
                <item.icon className="h-5 w-5" />
                {!sidebarCollapsed && <span>{t(item.titleKey)}</span>}
              </Link>
            ))}
        </nav>
        
        {user && ADMIN_ROLES.includes(user.role) && (
          <>
            <Separator className="my-4" />
            <div className="mb-2 px-3">
              {!sidebarCollapsed && (
                <span className="text-xs font-semibold uppercase text-muted-foreground">
                  Admin
                </span>
              )}
            </div>
            <nav className="space-y-2">
              {adminNavItems
                .filter((item) => hasRequiredRole(user.role, item.roles))
                .map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setSidebarOpen(false)}
                    className={cn(
                      'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                      pathname === item.href || pathname.startsWith(item.href + '/')
                        ? 'bg-primary text-primary-foreground'
                        : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                    )}
                  >
                    <item.icon className="h-5 w-5" />
                    {!sidebarCollapsed && <span>{t(item.titleKey)}</span>}
                  </Link>
                ))}
            </nav>
          </>
        )}
      </ScrollArea>
      <div className="border-t p-4">
        <div
          className={cn(
            'text-xs text-muted-foreground',
            sidebarCollapsed && 'text-center'
          )}
        >
          {!sidebarCollapsed && <span>Ethiopian Logistics Platform v1.0</span>}
        </div>
      </div>
    </div>
  )

  return (
    <>
      <aside
        className={cn(
          'hidden border-r bg-background lg:block',
          sidebarCollapsed ? 'w-16' : 'w-64'
        )}
      >
        <NavContent />
      </aside>

      <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
        <SheetContent side="left" className="w-64 p-0">
          <NavContent />
        </SheetContent>
      </Sheet>
    </>
  )
}
