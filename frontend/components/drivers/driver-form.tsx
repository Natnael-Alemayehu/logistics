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
import type { CreateDriverInput } from '@/types'

const ethiopianPhoneRegex = /^(\+251|0)[1-9]\d{8}$/

const driverFormSchema = z.object({
  full_name: z.string().min(1, 'Full name is required'),
  phone: z
    .string()
    .min(1, 'Phone number is required')
    .regex(ethiopianPhoneRegex, 'Please enter a valid Ethiopian phone number (e.g., +251912345678 or 0912345678)'),
  email: z.string().email('Please enter a valid email address').optional().or(z.literal('')),
  pin: z.string().length(4, 'PIN must be exactly 4 digits').optional().or(z.literal('')),
})

export type DriverFormData = z.infer<typeof driverFormSchema>

interface DriverFormProps {
  onSubmit: (data: CreateDriverInput) => Promise<void>
  isLoading?: boolean
  defaultValues?: Partial<DriverFormData>
  submitLabel?: string
}

export function DriverForm({
  onSubmit,
  isLoading,
  defaultValues,
  submitLabel = 'Add Driver',
}: DriverFormProps) {
  const form = useForm<DriverFormData>({
    resolver: zodResolver(driverFormSchema),
    defaultValues: {
      full_name: '',
      phone: '',
      email: '',
      pin: '',
      ...defaultValues,
    },
  })

  const handleSubmit = async (data: DriverFormData) => {
    const submitData: CreateDriverInput = {
      full_name: data.full_name,
      phone: data.phone,
      email: data.email || undefined,
      pin: data.pin || undefined,
    }
    await onSubmit(submitData)
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
        <FormField
          control={form.control}
          name="full_name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Full Name *</FormLabel>
              <FormControl>
                <Input placeholder="Enter driver's full name" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="phone"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Phone Number *</FormLabel>
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

        <FormField
          control={form.control}
          name="email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email (Optional)</FormLabel>
              <FormControl>
                <Input
                  type="email"
                  placeholder="driver@example.com"
                  {...field}
                  value={field.value ?? ''}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="pin"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Initial PIN (Optional)</FormLabel>
              <FormControl>
                <Input
                  type="password"
                  placeholder="4-digit PIN"
                  maxLength={4}
                  {...field}
                  value={field.value ?? ''}
                />
              </FormControl>
              <FormDescription>
                4-digit PIN for driver login. If not set, driver will need to set their own.
              </FormDescription>
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
