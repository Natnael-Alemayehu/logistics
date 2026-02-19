'use client'

import { useState } from 'react'
import { useTracking } from '@/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/shipments/status-badge'
import { EmptyState } from '@/components/shared'
import { Skeleton } from '@/components/ui/skeleton'
import { Package, Search, Loader2, MapPin, Phone, Calendar, User } from 'lucide-react'

function TrackingSkeleton() {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-6 w-20" />
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="flex items-start gap-3">
              <Skeleton className="h-5 w-5 rounded-full" />
              <div className="space-y-2 flex-1">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-5 w-32" />
              </div>
            </div>
          ))}
        </div>
        <div className="border-t pt-4 space-y-2">
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-5 w-48" />
        </div>
      </CardContent>
    </Card>
  )
}

export default function TrackPage() {
  const [trackingNumber, setTrackingNumber] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  const { data: shipment, isFetching, error, refetch } = useTracking(searchQuery)

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    if (trackingNumber.trim()) {
      setSearchQuery(trackingNumber.trim().toUpperCase())
    }
  }

  return (
    <div className="min-h-screen bg-muted/30">
      <div className="container mx-auto px-4 py-12">
        <div className="mx-auto max-w-2xl space-y-8">
          <div className="text-center">
            <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary">
              <Package className="h-8 w-8 text-primary-foreground" />
            </div>
            <h1 className="text-3xl font-bold">Track Your Shipment</h1>
            <p className="text-muted-foreground mt-2">
              Enter your tracking number to see delivery status
            </p>
          </div>

          <form onSubmit={handleSearch} className="flex gap-2">
            <Input
              type="text"
              placeholder="Enter tracking number (e.g., ET-20260219-1234)"
              value={trackingNumber}
              onChange={(e) => setTrackingNumber(e.target.value)}
              className="text-lg h-12"
            />
            <Button type="submit" size="lg" disabled={isFetching}>
              {isFetching ? (
                <Loader2 className="h-5 w-5 animate-spin" />
              ) : (
                <>
                  <Search className="h-5 w-5 mr-2" />
                  Track
                </>
              )}
            </Button>
          </form>

          {isFetching && <TrackingSkeleton />}

          {error && (
            <Card className="border-destructive">
              <CardContent className="pt-6 text-center">
                <p className="text-destructive">
                  Shipment not found. Please check your tracking number and try again.
                </p>
              </CardContent>
            </Card>
          )}

          {shipment && (
            <div className="space-y-6">
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-lg">
                      Shipment {shipment.tracking_number}
                    </CardTitle>
                    <StatusBadge status={shipment.status} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <div className="flex items-start gap-3">
                      <MapPin className="h-5 w-5 text-muted-foreground mt-0.5" />
                      <div>
                        <p className="text-sm text-muted-foreground">From</p>
                        <p className="font-medium">
                          {shipment.origin_address}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-start gap-3">
                      <MapPin className="h-5 w-5 text-primary mt-0.5" />
                      <div>
                        <p className="text-sm text-muted-foreground">To</p>
                        <p className="font-medium">
                          {shipment.destination_address}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-start gap-3">
                      <User className="h-5 w-5 text-muted-foreground mt-0.5" />
                      <div>
                        <p className="text-sm text-muted-foreground">Customer</p>
                        <p className="font-medium">
                          {shipment.customer_name}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-start gap-3">
                      <Phone className="h-5 w-5 text-muted-foreground mt-0.5" />
                      <div>
                        <p className="text-sm text-muted-foreground">Contact</p>
                        <p className="font-medium">
                          {shipment.customer_phone}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-start gap-3 sm:col-span-2">
                      <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
                      <div>
                        <p className="text-sm text-muted-foreground">
                          Estimated Delivery
                        </p>
                        <p className="font-medium">
                          {shipment.estimated_delivery
                            ? new Date(
                                shipment.estimated_delivery
                              ).toLocaleDateString('en-US', {
                                weekday: 'long',
                                year: 'numeric',
                                month: 'long',
                                day: 'numeric',
                              })
                            : 'Pending'}
                        </p>
                      </div>
                    </div>
                  </div>

                  {shipment.cargo_description && (
                    <div className="border-t pt-4">
                      <p className="text-sm text-muted-foreground">Cargo</p>
                      <p className="font-medium">
                        {shipment.cargo_description}
                      </p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          )}

          {!searchQuery && !shipment && (
            <EmptyState
              title="Enter a tracking number"
              description="Track your shipment by entering the tracking number provided to you"
              icon={Package}
            />
          )}
        </div>
      </div>
    </div>
  )
}
