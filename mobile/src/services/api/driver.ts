import { api } from './client';
import { API_ENDPOINTS } from '../constants';
import type { DriverStats, Vehicle, Driver } from '@/types/driver';

export async function getDriverStats(): Promise<DriverStats> {
  return api.get<DriverStats>(API_ENDPOINTS.drivers.stats);
}

export async function getDriverVehicle(): Promise<Vehicle | null> {
  return api.get<Vehicle | null>(API_ENDPOINTS.drivers.vehicle);
}

export async function getDriverProfile(): Promise<Driver> {
  return api.get<Driver>(API_ENDPOINTS.drivers.profile);
}
