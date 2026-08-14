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
  vehicle_type: z.string().optional(),
  is_active: z.boolean(),
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
      vehicle_type: '',
      is_active: true,
      ...defaultValues,
    },
  })

  const handleSubmit = async (data: VehicleFormData) => {
    const submitData: CreateVehicleInput = {
      plate_number: data.plate_number.toUpperCase().replace(/\s+/g, ' ').trim(),
      vehicle_type: data.vehicle_type || undefined,
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

        <FormField
          control={form.control}
          name="vehicle_type"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Vehicle Type</FormLabel>
              <FormControl>
                <Input placeholder="e.g., Isuzu NPR" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="is_active"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Status</FormLabel>
              <Select onValueChange={(v) => field.onChange(v === 'true')} defaultValue={field.value ? 'true' : 'false'}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select status" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value="true">Active</SelectItem>
                  <SelectItem value="false">Inactive</SelectItem>
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
