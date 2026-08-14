'use client'

import { useState } from 'react'
import { useVehicles, useCreateVehicle, useUpdateVehicle, useDeleteVehicle } from '@/hooks'
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
import { VehicleForm } from '@/components/vehicles/vehicle-form'
import { Plus, Search, Eye, Edit, Trash2, MoreHorizontal, Truck, MapPin } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { VEHICLE_STATUSES } from '@/lib/constants'
import type { Vehicle, CreateVehicleInput } from '@/types'

export default function VehiclesPage() {
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [editingVehicle, setEditingVehicle] = useState<Vehicle | null>(null)
  const [viewingVehicle, setViewingVehicle] = useState<Vehicle | null>(null)

  const { data, isLoading } = useVehicles()
  const createVehicle = useCreateVehicle()
  const updateVehicle = useUpdateVehicle()
  const deleteVehicle = useDeleteVehicle()

  const filteredVehicles = data?.vehicles?.filter((vehicle: Vehicle) => {
    const matchesSearch = vehicle.plate_number.toLowerCase().includes(searchQuery.toLowerCase()) ||
      
      vehicle.vehicle_type?.toLowerCase().includes(searchQuery.toLowerCase())
    const matchesStatus = statusFilter === 'all' || (vehicle.is_active ? "active" : "inactive") === statusFilter
    return matchesSearch && matchesStatus
  })

  const handleCreateVehicle = async (input: CreateVehicleInput) => {
    await createVehicle.mutateAsync(input)
    setIsAddDialogOpen(false)
  }

  const handleUpdateVehicle = async (input: CreateVehicleInput) => {
    if (editingVehicle) {
      await updateVehicle.mutateAsync({ id: editingVehicle.id, data: input })
      setEditingVehicle(null)
    }
  }

  const handleDeleteVehicle = (vehicle: Vehicle) => {
    if (confirm(`Are you sure you want to delete vehicle ${vehicle.plate_number}?`)) {
      deleteVehicle.mutate(vehicle.id)
      if (viewingVehicle?.id === vehicle.id) {
        setViewingVehicle(null)
      }
    }
  }

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'active':
        return 'default'
      case 'maintenance':
        return 'secondary'
      case 'inactive':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const getStatusLabel = (status: string) => {
    return VEHICLE_STATUSES.find(s => s.value === status)?.label || status
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">Vehicles</h1>
          <p className="text-muted-foreground">
            Manage your fleet vehicles
          </p>
        </div>
        <Button onClick={() => setIsAddDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Add Vehicle
        </Button>
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle>All Vehicles</CardTitle>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="w-full sm:w-[150px]">
                  <SelectValue placeholder="Filter by status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Statuses</SelectItem>
                  {VEHICLE_STATUSES.map((status) => (
                    <SelectItem key={status.value} value={status.value}>
                      {status.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search vehicles..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-9 w-full sm:w-[250px]"
                />
              </div>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <TableSkeleton rows={5} cols={6} />
          ) : filteredVehicles?.length === 0 ? (
            <EmptyState
              title="No vehicles found"
              description={searchQuery || statusFilter !== 'all' ? "Try adjusting your filters" : "Get started by adding your first vehicle"}
              icon={Truck}
              action={
                !searchQuery && statusFilter === 'all' && (
                  <Button onClick={() => setIsAddDialogOpen(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Add Vehicle
                  </Button>
                )
              }
            />
          ) : (
            <div className="rounded-lg border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Plate Number</TableHead>
                    <TableHead>Make / Model</TableHead>
                    <TableHead className="hidden md:table-cell">Year</TableHead>
                    <TableHead className="hidden lg:table-cell">Capacity</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="hidden md:table-cell">Driver</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredVehicles?.map((vehicle: Vehicle) => (
                    <TableRow key={vehicle.id}>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-primary">
                            <Truck className="h-4 w-4" />
                          </div>
                          <span className="font-medium">{vehicle.plate_number}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        {vehicle.vehicle_type ? (
                          <span>{vehicle.vehicle_type}</span>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        {false || <span className="text-muted-foreground">-</span>}
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        {false ? (
                          <span>{false.toLocaleString()} kg</span>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={getStatusBadgeVariant(vehicle.is_active ? "active" : "inactive")}>
                          {getStatusLabel(vehicle.is_active ? "active" : "inactive")}
                        </Badge>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        {false ? (
                          <span>{false}</span>
                        ) : (
                          <span className="text-muted-foreground">Unassigned</span>
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
                            <DropdownMenuItem onClick={() => setViewingVehicle(vehicle)}>
                              <Eye className="mr-2 h-4 w-4" />
                              View Details
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => setEditingVehicle(vehicle)}>
                              <Edit className="mr-2 h-4 w-4" />
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem 
                              onClick={() => handleDeleteVehicle(vehicle)}
                              className="text-destructive"
                            >
                              <Trash2 className="mr-2 h-4 w-4" />
                              Delete
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
            <DialogTitle>Add New Vehicle</DialogTitle>
            <DialogDescription>
              Enter the vehicle information to add it to your fleet.
            </DialogDescription>
          </DialogHeader>
          <VehicleForm
            onSubmit={handleCreateVehicle}
            isLoading={createVehicle.isPending}
          />
        </DialogContent>
      </Dialog>

      <Sheet open={!!editingVehicle} onOpenChange={() => setEditingVehicle(null)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>Edit Vehicle</SheetTitle>
            <SheetDescription>
              Update vehicle information.
            </SheetDescription>
          </SheetHeader>
          {editingVehicle && (
            <VehicleForm
              onSubmit={handleUpdateVehicle}
              isLoading={updateVehicle.isPending}
              defaultValues={{
                plate_number: editingVehicle.plate_number,
                is_active: editingVehicle.is_active,
              }}
              submitLabel="Save Changes"
            />
          )}
        </SheetContent>
      </Sheet>

      <Dialog open={!!viewingVehicle} onOpenChange={() => setViewingVehicle(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Vehicle Details</DialogTitle>
          </DialogHeader>
          {viewingVehicle && (
            <div className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
                  <Truck className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="font-semibold text-lg">{viewingVehicle.plate_number}</h3>
                  <Badge variant={getStatusBadgeVariant(viewingVehicle.is_active ? "active" : "inactive")}>
                    {getStatusLabel(viewingVehicle.is_active ? "active" : "inactive")}
                  </Badge>
                </div>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Make:</span>
                  <span>{viewingVehicle.vehicle_type || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Model:</span>
                  <span>{viewingVehicle.vehicle_type || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Year:</span>
                  <span>{false || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Capacity:</span>
                  <span>{false ? `${false.toLocaleString()} kg` : '-'}</span>
                </div>
                {false && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Driver:</span>
                    <span>{false}</span>
                  </div>
                )}
                {false && false && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Location:</span>
                    <span className="flex items-center gap-1">
                      <MapPin className="h-3 w-3 text-green-500" />
                      {""}, {""}
                    </span>
                  </div>
                )}
              </div>
              <div className="flex gap-2 pt-4">
                <Button
                  variant="outline"
                  className="flex-1"
                  onClick={() => {
                    setViewingVehicle(null)
                    setEditingVehicle(viewingVehicle)
                  }}
                >
                  <Edit className="mr-2 h-4 w-4" />
                  Edit
                </Button>
                <Button
                  variant="outline"
                  className="flex-1 text-destructive hover:text-destructive"
                  onClick={() => {
                    handleDeleteVehicle(viewingVehicle)
                  }}
                >
                  <Trash2 className="mr-2 h-4 w-4" />
                  Delete
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
