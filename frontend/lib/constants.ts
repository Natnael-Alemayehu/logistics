export const API_ENDPOINTS = {
  auth: {
    login: '/api/v1/auth/login/dispatcher',
    driverLogin: '/api/v1/auth/login/driver',
    logout: '/api/v1/auth/logout',
    refresh: '/api/v1/auth/refresh',
    sessions: '/api/v1/sessions',
    revokeSession: (id: string) => `/api/v1/sessions/${id}`,
    revokeOtherSessions: '/api/v1/sessions/others',
  },
  shipments: {
    list: '/api/v1/shipments',
    myShipments: '/api/v1/my-shipments',
    get: (id: string) => `/api/v1/shipments/${id}`,
    create: '/api/v1/shipments',
    update: (id: string) => `/api/v1/shipments/${id}`,
    cancel: (id: string) => `/api/v1/shipments/${id}/cancel`,
    assignDriver: (id: string) => `/api/v1/shipments/${id}/assign`,
    updateStatus: (id: string) => `/api/v1/shipments/${id}/status`,
    trackingEvents: (id: string) => `/api/v1/shipments/${id}/tracking`,
    pod: (id: string) => `/api/v1/shipments/${id}/pod`,
  },
  drivers: {
    list: '/api/v1/drivers',
    get: (id: string) => `/api/v1/drivers/${id}`,
    create: '/api/v1/drivers',
    update: (id: string) => `/api/v1/drivers/${id}`,
    delete: (id: string) => `/api/v1/drivers/${id}`,
    locations: '/api/v1/drivers/locations',
    location: (id: string) => `/api/v1/drivers/${id}/location`,
    resetPassword: (id: string) => `/api/v1/drivers/${id}/reset-password`,
  },
  vehicles: {
    list: '/api/v1/vehicles',
    active: '/api/v1/vehicles/active',
    get: (id: string) => `/api/v1/vehicles/${id}`,
    create: '/api/v1/vehicles',
    update: (id: string) => `/api/v1/vehicles/${id}`,
    delete: (id: string) => `/api/v1/vehicles/${id}`,
  },
  tracking: {
    track: (trackingNumber: string) => `/api/v1/track/${trackingNumber}`,
  },
  dashboard: {
    stats: '/api/v1/dashboard/stats',
    alerts: '/api/v1/dashboard/alerts',
    activity: '/api/v1/dashboard/activity',
  },
  users: {
    list: '/api/v1/users',
    get: (id: string) => `/api/v1/users/${id}`,
    update: (id: string) => `/api/v1/users/${id}`,
    delete: (id: string) => `/api/v1/users/${id}`,
  },
  tenants: {
    list: '/api/v1/tenants',
    get: (id: string) => `/api/v1/tenants/${id}`,
    create: '/api/v1/tenants',
    update: (id: string) => `/api/v1/tenants/${id}`,
  },
}

export const SHIPMENT_STATUSES = [
  { value: 'pending', label: 'Pending', color: 'gray' },
  { value: 'assigned', label: 'Assigned', color: 'blue' },
  { value: 'in_transit', label: 'In Transit', color: 'yellow' },
  { value: 'delayed', label: 'Delayed', color: 'orange' },
  { value: 'arrived', label: 'Arrived', color: 'purple' },
  { value: 'delivered', label: 'Delivered', color: 'green' },
  { value: 'issue', label: 'Issue', color: 'red' },
  { value: 'cancelled', label: 'Cancelled', color: 'gray' },
] as const

export const DELAY_REASONS = [
  'road_conditions',
  'weather',
  'security_checkpoint',
  'mechanical_issue',
  'traffic',
  'other',
] as const

export const ETHIOPIAN_CITIES = [
  'Addis Ababa',
  'Dire Dawa',
  'Mekelle',
  'Gondar',
  'Bahir Dar',
  'Hawassa',
  'Jimma',
  'Dessie',
  'Jijiga',
  'Shashamane',
  'Adama',
  'Arba Minch',
  'Hosaena',
  'Nekemte',
  'Debre Markos',
] as const

export const VEHICLE_STATUSES = [
  { value: 'active', label: 'Active', color: 'green' },
  { value: 'maintenance', label: 'Maintenance', color: 'yellow' },
  { value: 'inactive', label: 'Inactive', color: 'gray' },
] as const
