'use client'

import * as React from 'react'
import { cn } from '@/lib/utils'
import { SHIPMENT_STATUSES } from '@/lib/constants'
import { Badge } from '@/components/ui/badge'
import type { ShipmentStatus } from '@/types'

interface StatusBadgeProps {
  status: ShipmentStatus
  className?: string
}

const statusColors: Record<ShipmentStatus, string> = {
  pending: 'bg-gray-100 text-gray-800 hover:bg-gray-100',
  assigned: 'bg-blue-100 text-blue-800 hover:bg-blue-100',
  in_transit: 'bg-yellow-100 text-yellow-800 hover:bg-yellow-100',
  delayed: 'bg-orange-100 text-orange-800 hover:bg-orange-100',
  arrived: 'bg-purple-100 text-purple-800 hover:bg-purple-100',
  delivered: 'bg-green-100 text-green-800 hover:bg-green-100',
  issue: 'bg-red-100 text-red-800 hover:bg-red-100',
  cancelled: 'bg-gray-100 text-gray-600 hover:bg-gray-100',
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const statusInfo = SHIPMENT_STATUSES.find((s) => s.value === status)
  const colorClass = statusColors[status]

  return (
    <Badge variant="secondary" className={cn(colorClass, className)}>
      {statusInfo?.label || status}
    </Badge>
  )
}
