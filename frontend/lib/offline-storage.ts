import localforage from 'localforage'
import type { Shipment } from '@/types'
import type { Driver } from '@/types'
import type { Vehicle } from '@/types'

const shipmentsStore = localforage.createInstance({
  name: 'ethiopian-logistics',
  storeName: 'shipments',
})

const driversStore = localforage.createInstance({
  name: 'ethiopian-logistics',
  storeName: 'drivers',
})

const vehiclesStore = localforage.createInstance({
  name: 'ethiopian-logistics',
  storeName: 'vehicles',
})

const pendingOperationsStore = localforage.createInstance({
  name: 'ethiopian-logistics',
  storeName: 'pending-operations',
})

export interface PendingOperation {
  id: string
  type: 'create_shipment' | 'update_shipment' | 'update_status' | 'delete_shipment' | 'assign_driver' | 'create_pod'
  endpoint: string
  method: 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  data: unknown
  timestamp: number
  retries: number
}

export interface OfflineQueueStatus {
  pending: number
  oldestTimestamp?: number
}

export const offlineStorage = {
  async getShipments(): Promise<Shipment[]> {
    const shipments = await shipmentsStore.getItem<Shipment[]>('list')
    return shipments || []
  },

  async setShipments(shipments: Shipment[]): Promise<void> {
    await shipmentsStore.setItem('list', shipments)
  },

  async getShipment(id: string): Promise<Shipment | null> {
    return shipmentsStore.getItem<Shipment>(`shipment-${id}`)
  },

  async setShipment(shipment: Shipment): Promise<void> {
    await shipmentsStore.setItem(`shipment-${shipment.id}`, shipment)
  },

  async removeShipment(id: string): Promise<void> {
    await shipmentsStore.removeItem(`shipment-${id}`)
  },

  async getDrivers(): Promise<Driver[]> {
    const drivers = await driversStore.getItem<Driver[]>('list')
    return drivers || []
  },

  async setDrivers(drivers: Driver[]): Promise<void> {
    await driversStore.setItem('list', drivers)
  },

  async getDriver(id: string): Promise<Driver | null> {
    return driversStore.getItem<Driver>(`driver-${id}`)
  },

  async setDriver(driver: Driver): Promise<void> {
    await driversStore.setItem(`driver-${driver.id}`, driver)
  },

  async getVehicles(): Promise<Vehicle[]> {
    const vehicles = await vehiclesStore.getItem<Vehicle[]>('list')
    return vehicles || []
  },

  async setVehicles(vehicles: Vehicle[]): Promise<void> {
    await vehiclesStore.setItem('list', vehicles)
  },

  async getVehicle(id: string): Promise<Vehicle | null> {
    return vehiclesStore.getItem<Vehicle>(`vehicle-${id}`)
  },

  async setVehicle(vehicle: Vehicle): Promise<void> {
    await vehiclesStore.setItem(`vehicle-${vehicle.id}`, vehicle)
  },

  async addPendingOperation(operation: Omit<PendingOperation, 'id' | 'timestamp' | 'retries'>): Promise<string> {
    const id = `${operation.type}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
    const pendingOp: PendingOperation = {
      ...operation,
      id,
      timestamp: Date.now(),
      retries: 0,
    }
    await pendingOperationsStore.setItem(id, pendingOp)
    return id
  },

  async getPendingOperations(): Promise<PendingOperation[]> {
    const operations: PendingOperation[] = []
    await pendingOperationsStore.iterate<PendingOperation, void>((value) => {
      operations.push(value)
    })
    return operations.sort((a, b) => a.timestamp - b.timestamp)
  },

  async getPendingOperationsCount(): Promise<number> {
    let count = 0
    await pendingOperationsStore.iterate(() => {
      count++
    })
    return count
  },

  async getQueueStatus(): Promise<OfflineQueueStatus> {
    const operations = await this.getPendingOperations()
    return {
      pending: operations.length,
      oldestTimestamp: operations[0]?.timestamp,
    }
  },

  async removePendingOperation(id: string): Promise<void> {
    await pendingOperationsStore.removeItem(id)
  },

  async updatePendingOperation(id: string, updates: Partial<PendingOperation>): Promise<void> {
    const existing = await pendingOperationsStore.getItem<PendingOperation>(id)
    if (existing) {
      await pendingOperationsStore.setItem(id, { ...existing, ...updates })
    }
  },

  async clearPendingOperations(): Promise<void> {
    await pendingOperationsStore.clear()
  },

  async clearAllData(): Promise<void> {
    await shipmentsStore.clear()
    await driversStore.clear()
    await vehiclesStore.clear()
    await pendingOperationsStore.clear()
  },
}

export { shipmentsStore, driversStore, vehiclesStore, pendingOperationsStore }
