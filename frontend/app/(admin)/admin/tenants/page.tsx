'use client'

import { useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useTenants, useUpdateTenant } from '@/hooks'
import { TenantForm } from '@/components/admin'
import { Building2, Plus, Pencil, Power } from 'lucide-react'
import type { Tenant } from '@/types'

export default function TenantsPage() {
  const { data: tenantsData, isLoading } = useTenants()
  const updateTenant = useUpdateTenant()
  const [editingTenant, setEditingTenant] = useState<Tenant | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)

  const handleToggleActive = (tenant: Tenant) => {
    updateTenant.mutate({
      id: tenant.id,
      data: { is_active: !tenant.is_active },
    })
  }

  const handleEdit = (tenant: Tenant) => {
    setEditingTenant(tenant)
    setIsDialogOpen(true)
  }

  const handleDialogClose = () => {
    setIsDialogOpen(false)
    setEditingTenant(null)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Tenants</h1>
          <p className="text-muted-foreground">
            Manage platform tenants
          </p>
        </div>
        <Button onClick={() => setIsDialogOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Add Tenant
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="h-5 w-5" />
            All Tenants
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="h-12 animate-pulse rounded bg-muted"></div>
              ))}
            </div>
          ) : tenantsData?.tenants?.length === 0 ? (
            <p className="text-center py-8 text-muted-foreground">No tenants found</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Plan</TableHead>
                  <TableHead>Max Drivers</TableHead>
                  <TableHead>Max Vehicles</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tenantsData?.tenants?.map((tenant) => (
                  <TableRow key={tenant.id}>
                    <TableCell className="font-medium">{tenant.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="capitalize">
                        {tenant.plan}
                      </Badge>
                    </TableCell>
                    <TableCell>{tenant.max_drivers}</TableCell>
                    <TableCell>{tenant.max_vehicles ?? '-'}</TableCell>
                    <TableCell>
                      <Badge
                        variant={tenant.is_active ? 'default' : 'secondary'}
                        className={
                          tenant.is_active
                            ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200'
                            : 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200'
                        }
                      >
                        {tenant.is_active ? 'Active' : 'Inactive'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {new Date(tenant.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleEdit(tenant)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleToggleActive(tenant)}
                        >
                          <Power
                            className={`h-4 w-4 ${
                              tenant.is_active ? 'text-red-500' : 'text-green-500'
                            }`}
                          />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogClose}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingTenant ? 'Edit Tenant' : 'Create Tenant'}
            </DialogTitle>
          </DialogHeader>
          <TenantForm
            tenant={editingTenant}
            onSuccess={handleDialogClose}
          />
        </DialogContent>
      </Dialog>
    </div>
  )
}