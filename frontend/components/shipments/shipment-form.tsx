'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Driver, CreateShipmentInput } from '@/types'

const ethiopianPhoneRegex = /^(\+251|0)[1-9]\d{8}$/

const shipmentFormSchema = z.object({
  origin_address: z.string().min(1, 'Origin address is required'),
  origin_lat: z.number().optional(),
  origin_lng: z.number().optional(),
  destination_address: z.string().min(1, 'Destination address is required'),
  destination_lat: z.number().optional(),
  destination_lng: z.number().optional(),
  customer_name: z.string().min(1, 'Customer name is required'),
  customer_phone: z
    .string()
    .min(1, 'Customer phone is required')
    .regex(ethiopianPhoneRegex, 'Please enter a valid Ethiopian phone number (e.g., +251912345678 or 0912345678)'),
  cargo_description: z.string().optional(),
  cargo_weight: z.number().positive('Weight must be positive').optional().or(z.undefined()),
  cargo_value: z.number().positive('Value must be positive').optional().or(z.undefined()),
  special_instructions: z.string().optional(),
  driver_id: z.string().optional(),
})

export type ShipmentFormData = z.infer<typeof shipmentFormSchema>

interface ShipmentFormProps {
  onSubmit: (data: CreateShipmentInput) => Promise<void>
  isLoading?: boolean
  drivers?: Driver[]
  defaultValues?: Partial<ShipmentFormData>
  submitLabel?: string
}

export function ShipmentForm({
  onSubmit,
  isLoading,
  drivers,
  defaultValues,
  submitLabel = 'Create Shipment',
}: ShipmentFormProps) {
  const form = useForm<ShipmentFormData>({
    resolver: zodResolver(shipmentFormSchema),
    defaultValues: {
      origin_address: '',
      origin_lat: undefined,
      origin_lng: undefined,
      destination_address: '',
      destination_lat: undefined,
      destination_lng: undefined,
      customer_name: '',
      customer_phone: '',
      cargo_description: '',
      cargo_weight: undefined,
      cargo_value: undefined,
      special_instructions: '',
      driver_id: undefined,
      ...defaultValues,
    },
  })

  const handleSubmit = async (data: ShipmentFormData) => {
    const submitData: CreateShipmentInput = {
      origin_address: data.origin_address,
      origin_lat: data.origin_lat ? Number(data.origin_lat) : undefined,
      origin_lng: data.origin_lng ? Number(data.origin_lng) : undefined,
      destination_address: data.destination_address,
      destination_lat: data.destination_lat ? Number(data.destination_lat) : undefined,
      destination_lng: data.destination_lng ? Number(data.destination_lng) : undefined,
      customer_name: data.customer_name,
      customer_phone: data.customer_phone,
      cargo_description: data.cargo_description || undefined,
      cargo_weight: data.cargo_weight ? Number(data.cargo_weight) : undefined,
      cargo_value: data.cargo_value ? Number(data.cargo_value) : undefined,
      special_instructions: data.special_instructions || undefined,
      driver_id: data.driver_id || undefined,
    }
    await onSubmit(submitData)
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Origin Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              control={form.control}
              name="origin_address"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Origin Address *</FormLabel>
                  <FormControl>
                    <Input placeholder="Enter origin address" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="origin_lat"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Origin Latitude</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 9.0320"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="origin_lng"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Origin Longitude</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 38.7635"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Destination Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              control={form.control}
              name="destination_address"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Destination Address *</FormLabel>
                  <FormControl>
                    <Input placeholder="Enter destination address" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="destination_lat"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Destination Latitude</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 9.0320"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="destination_lng"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Destination Longitude</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 38.7635"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Customer Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="customer_name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Customer Name *</FormLabel>
                    <FormControl>
                      <Input placeholder="Enter customer name" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="customer_phone"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Customer Phone *</FormLabel>
                    <FormControl>
                      <Input placeholder="+251912345678" {...field} />
                    </FormControl>
                    <FormDescription>
                      Ethiopian phone format: +251XXXXXXXXX or 0XXXXXXXXX
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Cargo Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              control={form.control}
              name="cargo_description"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Cargo Description</FormLabel>
                  <FormControl>
                    <Input
                      placeholder="Describe the cargo"
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="cargo_weight"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Cargo Weight (kg)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 500"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="cargo_value"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Cargo Value (ETB)</FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        step="any"
                        placeholder="e.g., 10000"
                        {...field}
                        value={field.value ?? ''}
                        onChange={(e) => {
                          const val = e.target.value
                          field.onChange(val === '' ? undefined : parseFloat(val))
                        }}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Additional Information</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <FormField
              control={form.control}
              name="special_instructions"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Special Instructions</FormLabel>
                  <FormControl>
                    <textarea
                      className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-base shadow-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
                      placeholder="Any special handling instructions..."
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {drivers && drivers.length > 0 && (
              <FormField
                control={form.control}
                name="driver_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Assign Driver (Optional)</FormLabel>
                    <Select
                      onValueChange={field.onChange}
                      value={field.value}
                      defaultValue={field.value}
                    >
                      <FormControl>
                        <SelectTrigger className="w-full">
                          <SelectValue placeholder="Select a driver" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {drivers.map((driver) => (
                          <SelectItem key={driver.id} value={driver.id}>
                            {driver.full_name} - {driver.phone}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
          </CardContent>
        </Card>

        <div className="flex justify-end gap-4">
          <Button type="submit" disabled={isLoading}>
            {isLoading ? 'Saving...' : submitLabel}
          </Button>
        </div>
      </form>
    </Form>
  )
}