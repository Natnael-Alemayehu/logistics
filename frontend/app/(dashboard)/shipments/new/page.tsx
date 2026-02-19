'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useCreateShipment, useDrivers } from '@/hooks'
import { ShipmentForm } from '@/components/shipments/shipment-form'
import { Button } from '@/components/ui/button'
import { ArrowLeft } from 'lucide-react'
import type { CreateShipmentInput } from '@/types'

export default function NewShipmentPage() {
  const router = useRouter()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const createShipment = useCreateShipment()
  const { data: driversData } = useDrivers()

  const drivers = driversData?.users || []

  const handleSubmit = async (data: CreateShipmentInput) => {
    setIsSubmitting(true)
    try {
      await createShipment.mutateAsync(data)
      router.push('/shipments')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/shipments">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Shipments
          </Link>
        </Button>
      </div>

      <div>
        <h1 className="text-2xl font-bold">New Shipment</h1>
        <p className="text-muted-foreground">
          Create a new shipment with customer and cargo details
        </p>
      </div>

      <ShipmentForm
        onSubmit={handleSubmit}
        isLoading={isSubmitting}
        drivers={drivers}
        submitLabel="Create Shipment"
      />
    </div>
  )
}
