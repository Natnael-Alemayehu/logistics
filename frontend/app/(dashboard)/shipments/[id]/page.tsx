'use client'

import { use, useState } from 'react'
import Link from 'next/link'
import Image from 'next/image'
import { format } from 'date-fns'
import {
  useShipment,
  useTrackingEvents,
  useProofOfDelivery,
  useUpdateShipmentStatus,
  useAssignDriver,
  useDrivers,
} from '@/hooks'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/shipments/status-badge'
import { TrackingTimeline } from '@/components/shipments/tracking-timeline'
import { ErrorDisplay } from '@/components/shared'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { SHIPMENT_STATUSES } from '@/lib/constants'
import {
  ArrowLeft,
  MapPin,
  Phone,
  User,
  Package,
  Calendar,
  Truck,
  FileText,
  Clock,
  Signature,
  Image as ImageIcon,
} from 'lucide-react'
import type { ShipmentStatus } from '@/types'

export default function ShipmentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = use(params)
  const [statusDialogOpen, setStatusDialogOpen] = useState(false)
  const [driverDialogOpen, setDriverDialogOpen] = useState(false)
  const [selectedStatus, setSelectedStatus] = useState<ShipmentStatus | ''>('')
  const [statusNote, setStatusNote] = useState('')
  const [selectedDriverId, setSelectedDriverId] = useState('')

  const { data: shipment, isLoading, error, refetch } = useShipment(id)
  const { data: trackingData } = useTrackingEvents(id)
  const { data: pod } = useProofOfDelivery(id)
  const { data: driversData } = useDrivers()

  const updateStatus = useUpdateShipmentStatus()
  const assignDriver = useAssignDriver()

  const drivers = driversData?.users || []

  const handleUpdateStatus = async () => {
    if (!selectedStatus) return
    await updateStatus.mutateAsync({
      shipmentId: id,
      data: {
        status: selectedStatus,
        status_note: statusNote || undefined,
      },
    })
    setStatusDialogOpen(false)
    setSelectedStatus('')
    setStatusNote('')
  }

  const handleAssignDriver = async () => {
    if (!selectedDriverId) return
    await assignDriver.mutateAsync({
      shipmentId: id,
      driverId: selectedDriverId,
    })
    setDriverDialogOpen(false)
    setSelectedDriverId('')
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Skeleton className="h-9 w-24" />
          <Skeleton className="h-8 w-48" />
        </div>
        <div className="grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <Skeleton className="h-6 w-32" />
            </CardHeader>
            <CardContent className="space-y-4">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <Skeleton className="h-6 w-32" />
            </CardHeader>
            <CardContent className="space-y-4">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
            </CardContent>
          </Card>
        </div>
      </div>
    )
  }

  if (error || !shipment) {
    return (
      <div className="space-y-6">
        <Button variant="ghost" size="sm" asChild>
          <Link href="/shipments">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Shipments
          </Link>
        </Button>
        <ErrorDisplay
          title="Failed to load shipment"
          message={error?.message || 'Shipment not found'}
          onRetry={() => refetch()}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="sm" asChild>
            <Link href="/shipments">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-bold">{shipment.tracking_number}</h1>
            <StatusBadge status={shipment.status} className="mt-1" />
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href={`/shipments/${id}/edit`}>Edit</Link>
          </Button>
          <Button variant="outline" onClick={() => setStatusDialogOpen(true)}>
            Update Status
          </Button>
          <Button variant="outline" onClick={() => setDriverDialogOpen(true)}>
            Assign Driver
          </Button>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Shipment Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-start gap-3">
              <MapPin className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Origin</p>
                <p className="text-sm text-muted-foreground">
                  {shipment.origin_address}
                </p>
                {shipment.origin_lat && shipment.origin_lng && (
                  <p className="text-xs text-muted-foreground">
                    {shipment.origin_lat.toFixed(4)}, {shipment.origin_lng.toFixed(4)}
                  </p>
                )}
              </div>
            </div>
            <div className="flex items-start gap-3">
              <MapPin className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Destination</p>
                <p className="text-sm text-muted-foreground">
                  {shipment.destination_address}
                </p>
                {shipment.destination_lat && shipment.destination_lng && (
                  <p className="text-xs text-muted-foreground">
                    {shipment.destination_lat.toFixed(4)}, {shipment.destination_lng.toFixed(4)}
                  </p>
                )}
              </div>
            </div>
            {shipment.driver_id && (
              <div className="flex items-start gap-3">
                <Truck className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Assigned Driver</p>
                  <p className="text-sm text-muted-foreground">{shipment.driver_id}</p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Customer Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-start gap-3">
              <User className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Customer Name</p>
                <p className="text-sm text-muted-foreground">
                  {shipment.customer_name}
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <Phone className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Customer Phone</p>
                <p className="text-sm text-muted-foreground">
                  {shipment.customer_phone}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Cargo Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {shipment.cargo_description && (
              <div className="flex items-start gap-3">
                <Package className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Description</p>
                  <p className="text-sm text-muted-foreground">
                    {shipment.cargo_description}
                  </p>
                </div>
              </div>
            )}
            {shipment.cargo_weight && (
              <div className="flex items-start gap-3">
                <Package className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Weight</p>
                  <p className="text-sm text-muted-foreground">
                    {shipment.cargo_weight} kg
                  </p>
                </div>
              </div>
            )}
            {shipment.cargo_value && (
              <div>
                <p className="text-sm font-medium">Value</p>
                <p className="text-sm text-muted-foreground">
                  {shipment.cargo_value.toLocaleString()} ETB
                </p>
              </div>
            )}
            {shipment.special_instructions && (
              <div className="flex items-start gap-3">
                <FileText className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Special Instructions</p>
                  <p className="text-sm text-muted-foreground">
                    {shipment.special_instructions}
                  </p>
                </div>
              </div>
            )}
            {!shipment.cargo_description &&
              !shipment.cargo_weight &&
              !shipment.cargo_value &&
              !shipment.special_instructions && (
                <p className="text-sm text-muted-foreground">
                  No cargo details provided.
                </p>
              )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Timeline</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-start gap-3">
              <Calendar className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Created</p>
                <p className="text-sm text-muted-foreground">
                  {format(new Date(shipment.created_at), 'MMM d, yyyy h:mm a')}
                </p>
              </div>
            </div>
            {shipment.estimated_delivery && (
              <div className="flex items-start gap-3">
                <Clock className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Estimated Delivery</p>
                  <p className="text-sm text-muted-foreground">
                    {format(new Date(shipment.estimated_delivery), 'MMM d, yyyy')}
                  </p>
                </div>
              </div>
            )}
            {shipment.actual_delivery && (
              <div className="flex items-start gap-3">
                <Clock className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Actual Delivery</p>
                  <p className="text-sm text-muted-foreground">
                    {format(new Date(shipment.actual_delivery), 'MMM d, yyyy h:mm a')}
                  </p>
                </div>
              </div>
            )}
            {shipment.status_note && (
              <div className="flex items-start gap-3">
                <FileText className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Status Note</p>
                  <p className="text-sm text-muted-foreground">
                    {shipment.status_note}
                  </p>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {trackingData?.events && trackingData.events.length > 0 && (
        <TrackingTimeline events={trackingData.events} />
      )}

      {pod && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Proof of Delivery</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex items-start gap-3">
                <User className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Recipient Name</p>
                  <p className="text-sm text-muted-foreground">
                    {pod.recipient_name}
                  </p>
                </div>
              </div>
              {pod.recipient_phone && (
                <div className="flex items-start gap-3">
                  <Phone className="mt-0.5 h-4 w-4 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">Recipient Phone</p>
                    <p className="text-sm text-muted-foreground">
                      {pod.recipient_phone}
                    </p>
                  </div>
                </div>
              )}
            </div>
            {pod.delivery_address && (
              <div className="flex items-start gap-3">
                <MapPin className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Delivery Address</p>
                  <p className="text-sm text-muted-foreground">
                    {pod.delivery_address}
                  </p>
                </div>
              </div>
            )}
            {pod.delivery_notes && (
              <div className="flex items-start gap-3">
                <FileText className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Delivery Notes</p>
                  <p className="text-sm text-muted-foreground">
                    {pod.delivery_notes}
                  </p>
                </div>
              </div>
            )}
            <div className="flex items-start gap-3">
              <Clock className="mt-0.5 h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">Delivered At</p>
                <p className="text-sm text-muted-foreground">
                  {format(new Date(pod.recorded_at), 'MMM d, yyyy h:mm a')}
                </p>
              </div>
            </div>
            {pod.signature_url && (
              <div className="flex items-start gap-3">
                <Signature className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Signature</p>
                  <Image
                    src={pod.signature_url}
                    alt="Delivery signature"
                    className="mt-2 h-24 rounded border"
                    width={200}
                    height={96}
                    unoptimized
                  />
                </div>
              </div>
            )}
            {pod.photo_urls && pod.photo_urls.length > 0 && (
              <div className="flex items-start gap-3">
                <ImageIcon className="mt-0.5 h-4 w-4 text-muted-foreground" />
                <div>
                  <p className="text-sm font-medium">Photos</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {pod.photo_urls.map((url, index) => (
                      <Image
                        key={index}
                        src={url}
                        alt={`Delivery photo ${index + 1}`}
                        className="h-24 w-24 rounded border object-cover"
                        width={96}
                        height={96}
                        unoptimized
                      />
                    ))}
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Dialog open={statusDialogOpen} onOpenChange={setStatusDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Update Status</DialogTitle>
            <DialogDescription>
              Update the status for shipment {shipment.tracking_number}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Status</label>
              <Select
                value={selectedStatus}
                onValueChange={(v) => setSelectedStatus(v as ShipmentStatus)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select status" />
                </SelectTrigger>
                <SelectContent>
                  {SHIPMENT_STATUSES.map((status) => (
                    <SelectItem key={status.value} value={status.value}>
                      {status.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Note (Optional)</label>
              <Input
                value={statusNote}
                onChange={(e) => setStatusNote(e.target.value)}
                placeholder="Add a note about the status change"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setStatusDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleUpdateStatus}
              disabled={!selectedStatus || updateStatus.isPending}
            >
              {updateStatus.isPending ? 'Updating...' : 'Update Status'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={driverDialogOpen} onOpenChange={setDriverDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Assign Driver</DialogTitle>
            <DialogDescription>
              Assign a driver to shipment {shipment.tracking_number}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Driver</label>
              <Select
                value={selectedDriverId}
                onValueChange={setSelectedDriverId}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a driver" />
                </SelectTrigger>
                <SelectContent>
                  {drivers.map((driver) => (
                    <SelectItem key={driver.id} value={driver.id}>
                      {driver.full_name} - {driver.phone}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDriverDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleAssignDriver}
              disabled={!selectedDriverId || assignDriver.isPending}
            >
              {assignDriver.isPending ? 'Assigning...' : 'Assign Driver'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
