'use client'

import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { MapPin, Phone, Mail, Truck, MoreHorizontal, Eye, Edit, KeyRound } from 'lucide-react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { Driver } from '@/types'

interface DriverCardProps {
  driver: Driver
  onViewDetails?: (driver: Driver) => void
  onEdit?: (driver: Driver) => void
  onResetPassword?: (driver: Driver) => void
}

export function DriverCard({ driver, onViewDetails, onEdit, onResetPassword }: DriverCardProps) {
  const statusColor = driver.is_active ? 'bg-green-500' : 'bg-gray-400'

  return (
    <Card className="relative">
      <CardContent className="p-4">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-3">
            <div className="relative">
              <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-primary font-semibold">
                {driver.full_name.charAt(0).toUpperCase()}
              </div>
              <div
                className={`absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-white ${statusColor}`}
                title={driver.is_active ? 'Active' : 'Inactive'}
              />
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold truncate">{driver.full_name}</h3>
              <div className="flex items-center gap-1 text-sm text-muted-foreground mt-1">
                <Phone className="h-3 w-3" />
                <span className="truncate">{driver.phone}</span>
              </div>
              {driver.email && (
                <div className="flex items-center gap-1 text-sm text-muted-foreground mt-0.5">
                  <Mail className="h-3 w-3" />
                  <span className="truncate">{driver.email}</span>
                </div>
              )}
            </div>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onViewDetails?.(driver)}>
                <Eye className="mr-2 h-4 w-4" />
                View Details
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onEdit?.(driver)}>
                <Edit className="mr-2 h-4 w-4" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onResetPassword?.(driver)}>
                <KeyRound className="mr-2 h-4 w-4" />
                Reset Password
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          <Badge variant={driver.is_active ? 'default' : 'secondary'}>
            {driver.is_active ? 'Active' : 'Inactive'}
          </Badge>
          {false && (
            <Badge variant="outline" className="flex items-center gap-1">
              <Truck className="h-3 w-3" />
              {false}
            </Badge>
          )}
        </div>

        {false && false && (
          <div className="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
            <MapPin className="h-3 w-3" />
            <span>Location available</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
