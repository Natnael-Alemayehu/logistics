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
import { VEHICLE_STATUSES } from '@/lib/constants'
import type { CreateVehicleInput } from '@/types'

const ethiopianPlateRegex = /^[A-Za-z]{2,3}\s*\d{1,4}$|^\d{2,3}-[A-Za-z]{2,3}-\d{1,4}$/i

const vehicleFormSchema = z.object({
  plate_number: z
    .string()
    .min(1, 'Plate number is required')
    .regex(ethiopianPlateRegex, 'Please enter a valid Ethiopian plate number (e.g., AA 1234 or 01-AA-1234)'),
  make: z.string().optional(),
  model: z.string().optional(),
  year: z
    .number()
    .int('Year must be a whole number')
    .min(1990, 'Year must be 1990 or later')
    .max(new Date().getFullYear() + 1, 'Year cannot be in the future')
    .optional()
    .or(z.undefined()),
  capacity_kg: z
    .number()
    .positive('Capacity must be positive')
    .optional()
    .or(z.undefined()),
  status: z.enum(['active', 'maintenance', 'inactive']).optional(),
})

export type VehicleFormData = z.infer<typeof vehicleFormSchema>

interface VehicleFormProps {
  onSubmit: (data: CreateVehicleInput) => Promise<void>
  isLoading?: boolean
  defaultValues?: Partial<VehicleFormData>
  submitLabel?: string
}

export function VehicleForm({
  onSubmit,
  isLoading,
  defaultValues,
  submitLabel = 'Add Vehicle',
}: VehicleFormProps) {
  const form = useForm<VehicleFormData>({
    resolver: zodResolver(vehicleFormSchema),
    defaultValues: {
      plate_number: '',
      make: '',
      model: '',
      year: undefined,
      capacity_kg: undefined,
      status: 'active',
      ...defaultValues,
    },
  })

  const handleSubmit = async (data: VehicleFormData) => {
    const submitData: CreateVehicleInput = {
      plate_number: data.plate_number.toUpperCase().replace(/\s+/g, ' ').trim(),
      make: data.make || undefined,
      model: data.model || undefined,
      year: data.year,
      capacity_kg: data.capacity_kg,
      status: data.status,
    }
    await onSubmit(submitData)
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
        <FormField
          control={form.control}
          name="plate_number"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Plate Number *</FormLabel>
              <FormControl>
                <Input placeholder="AA 1234" {...field} className="uppercase" />
              </FormControl>
              <FormDescription>
                Ethiopian plate format: AA 1234 or 01-AA-1234
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="make"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Make</FormLabel>
                <FormControl>
                  <Input placeholder="e.g., Isuzu" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="model"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Model</FormLabel>
                <FormControl>
                  <Input placeholder="e.g., NRR" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="year"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Year</FormLabel>
                <FormControl>
                  <Input
                    type="number"
                    placeholder="e.g., 2020"
                    {...field}
                    value={field.value ?? ''}
                    onChange={(e) => {
                      const value = e.target.value ? parseInt(e.target.value) : undefined
                      field.onChange(value)
                    }}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="capacity_kg"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Capacity (kg)</FormLabel>
                <FormControl>
                  <Input
                    type="number"
                    placeholder="e.g., 5000"
                    {...field}
                    value={field.value ?? ''}
                    onChange={(e) => {
                      const value = e.target.value ? parseFloat(e.target.value) : undefined
                      field.onChange(value)
                    }}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name="status"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Status</FormLabel>
              <Select onValueChange={field.onChange} defaultValue={field.value}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select status" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {VEHICLE_STATUSES.map((status) => (
                    <SelectItem key={status.value} value={status.value}>
                      {status.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="flex justify-end gap-2 pt-4">
          <Button type="submit" disabled={isLoading}>
            {isLoading ? 'Saving...' : submitLabel}
          </Button>
        </div>
      </form>
    </Form>
  )
}
