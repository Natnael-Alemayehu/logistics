'use client'

import { useState } from 'react'
import { useDrivers, useCreateDriver, useUpdateDriver, useResetDriverPassword } from '@/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { TableSkeleton, EmptyState } from '@/components/shared'
import { DriverForm } from '@/components/drivers/driver-form'
import { Plus, Search, Eye, Edit, KeyRound, MoreHorizontal } from 'lucide-react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { MapPin, Truck } from 'lucide-react'
import type { Driver, CreateDriverInput } from '@/types'

export default function DriversPage() {
  const [searchQuery, setSearchQuery] = useState('')
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [editingDriver, setEditingDriver] = useState<Driver | null>(null)
  const [viewingDriver, setViewingDriver] = useState<Driver | null>(null)

  const { data, isLoading, refetch } = useDrivers()
  const createDriver = useCreateDriver()
  const updateDriver = useUpdateDriver()
  const resetPassword = useResetDriverPassword()

  const filteredDrivers = data?.users?.filter((driver: Driver) =>
    driver.full_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    driver.phone.includes(searchQuery) ||
    driver.email?.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const handleCreateDriver = async (input: CreateDriverInput) => {
    await createDriver.mutateAsync(input)
    setIsAddDialogOpen(false)
  }

  const handleUpdateDriver = async (input: CreateDriverInput) => {
    if (editingDriver) {
      await updateDriver.mutateAsync({ id: editingDriver.id, data: input })
      setEditingDriver(null)
    }
  }

  const handleResetPassword = (driver: Driver) => {
    if (confirm(`Reset password for ${driver.full_name}?`)) {
      resetPassword.mutate(driver.id)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Drivers</h1>
          <p className="text-muted-foreground">
            Manage your fleet drivers
          </p>
        </div>
        <Button onClick={() => setIsAddDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Add Driver
        </Button>
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle>All Drivers</CardTitle>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search drivers..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-9 w-full sm:w-[250px]"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <TableSkeleton rows={5} cols={5} />
          ) : filteredDrivers?.length === 0 ? (
            <EmptyState
              title="No drivers found"
              description={searchQuery ? "Try adjusting your search" : "Get started by adding your first driver"}
              icon={Plus}
              action={
                !searchQuery && (
                  <Button onClick={() => setIsAddDialogOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Add Driver
                  </Button>
                )
              }
            />
          ) : (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Phone</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="hidden md:table-cell">Assigned Vehicle</TableHead>
                    <TableHead className="hidden lg:table-cell">Current Location</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredDrivers?.map((driver: Driver) => (
                    <TableRow key={driver.id}>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary font-semibold text-sm">
                            {driver.full_name.charAt(0).toUpperCase()}
                          </div>
                          <div>
                            <p className="font-medium">{driver.full_name}</p>
                            {driver.email && (
                              <p className="text-xs text-muted-foreground">{driver.email}</p>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{driver.phone}</TableCell>
                      <TableCell>
                        <Badge variant={driver.is_active ? 'default' : 'secondary'}>
                          {driver.is_active ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        {driver.assigned_vehicle_plate ? (
                          <div className="flex items-center gap-1">
                            <Truck className="h-4 w-4 text-muted-foreground" />
                            <span>{driver.assigned_vehicle_plate}</span>
                          </div>
                        ) : (
                          <span className="text-muted-foreground">Unassigned</span>
                        )}
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        {driver.current_lat && driver.current_lng ? (
                          <div className="flex items-center gap-1">
                            <MapPin className="h-4 w-4 text-green-500" />
                            <span className="text-sm">
                              {driver.current_lat.toFixed(4)}, {driver.current_lng.toFixed(4)}
                            </span>
                          </div>
                        ) : (
                          <span className="text-muted-foreground">Unknown</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon">
                              <MoreHorizontal className="h-4 w-4" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => setViewingDriver(driver)}>
                              <Eye className="mr-2 h-4 w-4" />
                              View Details
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => setEditingDriver(driver)}>
                              <Edit className="mr-2 h-4 w-4" />
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => handleResetPassword(driver)}>
                              <KeyRound className="mr-2 h-4 w-4" />
                              Reset Password
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add New Driver</DialogTitle>
            <DialogDescription>
              Enter the driver's information to create a new driver account.
            </DialogDescription>
          </DialogHeader>
          <DriverForm
            onSubmit={handleCreateDriver}
            isLoading={createDriver.isPending}
          />
        </DialogContent>
      </Dialog>

      <Sheet open={!!editingDriver} onOpenChange={() => setEditingDriver(null)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Edit Driver</SheetTitle>
            <SheetDescription>
              Update driver information.
            </SheetDescription>
          </SheetHeader>
          {editingDriver && (
            <DriverForm
              onSubmit={handleUpdateDriver}
              isLoading={updateDriver.isPending}
              defaultValues={{
                full_name: editingDriver.full_name,
                phone: editingDriver.phone,
                email: editingDriver.email || '',
              }}
              submitLabel="Save Changes"
            />
          )}
        </SheetContent>
      </Sheet>

      <Dialog open={!!viewingDriver} onOpenChange={() => setViewingDriver(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Driver Details</DialogTitle>
          </DialogHeader>
          {viewingDriver && (
            <div className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary font-semibold text-lg">
                  {viewingDriver.full_name.charAt(0).toUpperCase()}
                </div>
                <div>
                  <h3 className="font-semibold text-lg">{viewingDriver.full_name}</h3>
                  <Badge variant={viewingDriver.is_active ? 'default' : 'secondary'}>
                    {viewingDriver.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                </div>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Phone:</span>
                  <span>{viewingDriver.phone}</span>
                </div>
                {viewingDriver.email && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Email:</span>
                    <span>{viewingDriver.email}</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Role:</span>
                  <span className="capitalize">{viewingDriver.role}</span>
                </div>
                {viewingDriver.assigned_vehicle_plate && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Vehicle:</span>
                    <span>{viewingDriver.assigned_vehicle_plate}</span>
                  </div>
                )}
                {viewingDriver.current_lat && viewingDriver.current_lng && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Location:</span>
                    <span>{viewingDriver.current_lat.toFixed(4)}, {viewingDriver.current_lng.toFixed(4)}</span>
                  </div>
                )}
              </div>
              <div className="flex gap-2 pt-4">
                <Button
                  variant="outline"
                  className="flex-1"
                  onClick={() => {
                    setViewingDriver(null)
                    setEditingDriver(viewingDriver)
                  }}
                >
                  <Edit className="mr-2 h-4 w-4" />
                  Edit
                </Button>
                <Button
                  variant="outline"
                  className="flex-1"
                  onClick={() => {
                    handleResetPassword(viewingDriver)
                    setViewingDriver(null)
                  }}
                >
                  <KeyRound className="mr-2 h-4 w-4" />
                  Reset Password
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
