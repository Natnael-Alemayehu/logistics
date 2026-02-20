'use client'

import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useLogin, useDriverLogin } from '@/hooks'
import { useAuthStore } from '@/stores'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Package, Loader2 } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'

const dispatcherLoginSchema = z.object({
  email: z.string().email('Please enter a valid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
})

const driverLoginSchema = z.object({
  phone: z
    .string()
    .min(10, 'Please enter a valid phone number')
    .regex(/^(\+251|0)[97]\d{8}$/, 'Please enter a valid Ethiopian phone number'),
  pin: z
    .string()
    .length(4, 'PIN must be 4 digits')
    .regex(/^\d+$/, 'PIN must contain only digits'),
})

type DispatcherLoginFormData = z.infer<typeof dispatcherLoginSchema>
type DriverLoginFormData = z.infer<typeof driverLoginSchema>

export default function LoginPage() {
  const router = useRouter()
  const [activeTab, setActiveTab] = useState<'dispatcher' | 'driver'>('dispatcher')
  
  const dispatcherLogin = useLogin()
  const driverLoginMutation = useDriverLogin()
  const { isAuthenticated } = useAuthStore()

  useEffect(() => {
    if (isAuthenticated) {
      router.push('/dashboard')
    }
  }, [isAuthenticated, router])

  useEffect(() => {
    if (dispatcherLogin.isError) {
      toast.error(dispatcherLogin.error?.message || 'Invalid email or password')
    }
  }, [dispatcherLogin.isError, dispatcherLogin.error])

  useEffect(() => {
    if (driverLoginMutation.isError) {
      toast.error(driverLoginMutation.error?.message || 'Invalid phone or PIN')
    }
  }, [driverLoginMutation.isError, driverLoginMutation.error])

  const dispatcherForm = useForm<DispatcherLoginFormData>({
    resolver: zodResolver(dispatcherLoginSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  const driverForm = useForm<DriverLoginFormData>({
    resolver: zodResolver(driverLoginSchema),
    defaultValues: {
      phone: '',
      pin: '',
    },
  })

  const onDispatcherSubmit = (data: DispatcherLoginFormData) => {
    dispatcherLogin.mutate(data)
  }

  const onDriverSubmit = (data: DriverLoginFormData) => {
    driverLoginMutation.mutate(data)
  }

  const isLoading = dispatcherLogin.isPending || driverLoginMutation.isPending

  return (
    <div className="w-full max-w-md p-4">
      <div className="mb-8 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-primary">
          <Package className="h-6 w-6 text-primary-foreground" />
        </div>
        <h1 className="text-2xl font-bold">Ethiopian Logistics</h1>
        <p className="text-muted-foreground">
          Shipment tracking and delivery management
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Welcome back</CardTitle>
          <CardDescription>
            Sign in to your account to continue
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'dispatcher' | 'driver')}>
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="dispatcher">Dispatcher</TabsTrigger>
              <TabsTrigger value="driver">Driver</TabsTrigger>
            </TabsList>

            <TabsContent value="dispatcher" className="mt-4">
              <form onSubmit={dispatcherForm.handleSubmit(onDispatcherSubmit)} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    placeholder="dispatcher@company.com"
                    {...dispatcherForm.register('email')}
                    disabled={isLoading}
                  />
                  {dispatcherForm.formState.errors.email && (
                    <p className="text-sm text-destructive">
                      {dispatcherForm.formState.errors.email.message}
                    </p>
                  )}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    {...dispatcherForm.register('password')}
                    disabled={isLoading}
                  />
                  {dispatcherForm.formState.errors.password && (
                    <p className="text-sm text-destructive">
                      {dispatcherForm.formState.errors.password.message}
                    </p>
                  )}
                </div>

                <Button type="submit" className="w-full" disabled={isLoading}>
                  {dispatcherLogin.isPending ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Signing in...
                    </>
                  ) : (
                    'Sign In'
                  )}
                </Button>
              </form>
            </TabsContent>

            <TabsContent value="driver" className="mt-4">
              <form onSubmit={driverForm.handleSubmit(onDriverSubmit)} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="phone">Phone Number</Label>
                  <Input
                    id="phone"
                    type="tel"
                    placeholder="0912345678 or +251912345678"
                    {...driverForm.register('phone')}
                    disabled={isLoading}
                  />
                  {driverForm.formState.errors.phone && (
                    <p className="text-sm text-destructive">
                      {driverForm.formState.errors.phone.message}
                    </p>
                  )}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="pin">PIN</Label>
                  <Input
                    id="pin"
                    type="password"
                    placeholder="Enter your 4-digit PIN"
                    maxLength={4}
                    {...driverForm.register('pin')}
                    disabled={isLoading}
                  />
                  {driverForm.formState.errors.pin && (
                    <p className="text-sm text-destructive">
                      {driverForm.formState.errors.pin.message}
                    </p>
                  )}
                </div>

                <Button type="submit" className="w-full" disabled={isLoading}>
                  {driverLoginMutation.isPending ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Signing in...
                    </>
                  ) : (
                    'Sign In'
                  )}
                </Button>
              </form>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      <p className="mt-4 text-center text-sm text-muted-foreground">
        <button
          type="button"
          onClick={() => router.push('/track')}
          className="text-primary hover:underline"
        >
          Track a shipment
        </button>
        {' '}without signing in
      </p>
    </div>
  )
}
