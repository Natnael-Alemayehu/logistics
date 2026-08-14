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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useUsers, useUpdateUser, useTenants } from '@/hooks'
import { Users, Pencil, Power, Shield } from 'lucide-react'
import type { User, UserRole } from '@/types'

const USER_ROLES: UserRole[] = ['driver', 'dispatcher', 'fleet_manager', 'admin', 'platform_admin']

export default function AdminUsersPage() {
  const [tenantFilter, setTenantFilter] = useState<string>('all')
  const [roleFilter, setRoleFilter] = useState<string>('all')
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [isDialogOpen, setIsDialogOpen] = useState(false)

  const { data: usersData, isLoading } = useUsers(
    tenantFilter !== 'all' || roleFilter !== 'all'
      ? {
          tenant_id: tenantFilter !== 'all' ? tenantFilter : undefined,
          role: roleFilter !== 'all' ? (roleFilter as UserRole) : undefined,
        }
      : undefined
  )
  const { data: tenantsData } = useTenants()
  const updateUser = useUpdateUser()

  const handleToggleActive = (user: User) => {
    updateUser.mutate({
      id: user.id,
      data: { is_active: !user.is_active },
    })
  }

  const handleEdit = (user: User) => {
    setEditingUser(user)
    setIsDialogOpen(true)
  }

  const handleRoleChange = (user: User, role: UserRole) => {
    updateUser.mutate({
      id: user.id,
      data: { role },
    })
  }

  const handleDialogClose = () => {
    setIsDialogOpen(false)
    setEditingUser(null)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Users</h1>
          <p className="text-muted-foreground">
            Manage users across all tenants
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="flex items-center gap-2">
              <Users className="h-5 w-5" />
              All Users
            </CardTitle>
            <div className="flex gap-2">
              <Select value={tenantFilter} onValueChange={setTenantFilter}>
                <SelectTrigger className="w-[180px]">
                  <SelectValue placeholder="Filter by tenant" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Tenants</SelectItem>
                  {tenantsData?.tenants?.map((tenant) => (
                    <SelectItem key={tenant.id} value={tenant.id}>
                      {tenant.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select value={roleFilter} onValueChange={setRoleFilter}>
                <SelectTrigger className="w-[150px]">
                  <SelectValue placeholder="Filter by role" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Roles</SelectItem>
                  {USER_ROLES.map((role) => (
                    <SelectItem key={role} value={role}>
                      {role.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase())}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="h-12 animate-pulse rounded bg-muted"></div>
              ))}
            </div>
          ) : usersData?.users?.length === 0 ? (
            <p className="text-center py-8 text-muted-foreground">No users found</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Tenant</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersData?.users?.map((user) => {
                  const tenant = tenantsData?.tenants?.find((t) => t.id === user.tenant_id)
                  return (
                    <TableRow key={user.id}>
                      <TableCell className="font-medium">{user.full_name}</TableCell>
                      <TableCell>{user.email || '-'}</TableCell>
                      <TableCell>
                        <Select
                          value={user.role}
                          onValueChange={(role) => handleRoleChange(user, role as UserRole)}
                        >
                          <SelectTrigger className="w-[130px] h-8">
                            <Badge variant="outline" className="capitalize">
                              <Shield className="mr-1 h-3 w-3" />
                              {user.role.replace('_', ' ')}
                            </Badge>
                          </SelectTrigger>
                          <SelectContent>
                            {USER_ROLES.map((role) => (
                              <SelectItem key={role} value={role}>
                                {role.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase())}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell>{tenant?.name || user.tenant_id}</TableCell>
                      <TableCell>
                        <Badge
                          variant={user.is_active ? 'default' : 'secondary'}
                          className={
                            user.is_active
                              ? 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200'
                              : 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200'
                          }
                        >
                          {user.is_active ? 'Active' : 'Inactive'}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {new Date(user.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => handleEdit(user)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => handleToggleActive(user)}
                          >
                            <Power
                              className={`h-4 w-4 ${
                                user.is_active ? 'text-red-500' : 'text-green-500'
                              }`}
                            />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={isDialogOpen} onOpenChange={handleDialogClose}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit User</DialogTitle>
          </DialogHeader>
          {editingUser && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <p className="text-sm text-muted-foreground">{editingUser.full_name}</p>
                </div>
                <div>
                  <label className="text-sm font-medium">Email</label>
                  <p className="text-sm text-muted-foreground">{editingUser.email || '-'}</p>
                </div>
                <div>
                  <label className="text-sm font-medium">Phone</label>
                  <p className="text-sm text-muted-foreground">{editingUser.phone || '-'}</p>
                </div>
                <div>
                  <label className="text-sm font-medium">Role</label>
                  <p className="text-sm text-muted-foreground capitalize">
                    {editingUser.role.replace('_', ' ')}
                  </p>
                </div>
              </div>
              <div className="flex justify-end">
                <Button variant="outline" onClick={handleDialogClose}>
                  Close
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}