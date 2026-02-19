'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormControl,
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
import { useCreateTenant, useUpdateTenant } from '@/hooks'
import type { Tenant } from '@/types'

const PLANS = ['free', 'basic', 'pro', 'enterprise'] as const

const tenantFormSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required').regex(/^[a-z0-9-]+$/, 'Slug must be lowercase alphanumeric with hyphens'),
  plan: z.enum(PLANS),
  max_drivers: z.number().min(1, 'Must be at least 1'),
  max_vehicles: z.number().min(0, 'Must be 0 or greater'),
})

export type TenantFormData = z.infer<typeof tenantFormSchema>

interface TenantFormProps {
  tenant?: Tenant | null
  onSuccess: () => void
}

export function TenantForm({ tenant, onSuccess }: TenantFormProps) {
  const createTenant = useCreateTenant()
  const updateTenant = useUpdateTenant()

  const form = useForm<TenantFormData>({
    resolver: zodResolver(tenantFormSchema),
    defaultValues: {
      name: tenant?.name ?? '',
      slug: tenant?.slug ?? '',
      plan: (tenant?.plan as typeof PLANS[number]) ?? 'basic',
      max_drivers: tenant?.max_drivers ?? 10,
      max_vehicles: tenant?.max_vehicles ?? 10,
    },
  })

  const isEditing = !!tenant
  const isLoading = createTenant.isPending || updateTenant.isPending

  const onSubmit = async (data: TenantFormData) => {
    if (isEditing) {
      updateTenant.mutate(
        {
          id: tenant.id,
          data: {
            name: data.name,
            plan: data.plan,
            max_drivers: data.max_drivers,
            max_vehicles: data.max_vehicles,
          },
        },
        {
          onSuccess,
        }
      )
    } else {
      createTenant.mutate(data, { onSuccess })
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Name</FormLabel>
              <FormControl>
                <Input placeholder="Company name" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="slug"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Slug</FormLabel>
              <FormControl>
                <Input
                  placeholder="company-slug"
                  {...field}
                  disabled={isEditing}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="plan"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Plan</FormLabel>
              <Select onValueChange={field.onChange} defaultValue={field.value}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a plan" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {PLANS.map((plan) => (
                    <SelectItem key={plan} value={plan}>
                      {plan.charAt(0).toUpperCase() + plan.slice(1)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="max_drivers"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Max Drivers</FormLabel>
                <FormControl>
                  <Input
                    type="number"
                    min={1}
                    {...field}
                    onChange={(e) => field.onChange(parseInt(e.target.value, 10) || 0)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="max_vehicles"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Max Vehicles</FormLabel>
                <FormControl>
                  <Input
                    type="number"
                    min={0}
                    {...field}
                    onChange={(e) => field.onChange(parseInt(e.target.value, 10) || 0)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <div className="flex justify-end gap-2 pt-4">
          <Button type="button" variant="outline" onClick={onSuccess}>
            Cancel
          </Button>
          <Button type="submit" disabled={isLoading}>
            {isLoading ? 'Saving...' : isEditing ? 'Update Tenant' : 'Create Tenant'}
          </Button>
        </div>
      </form>
    </Form>
  )
}