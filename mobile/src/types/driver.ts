export interface DriverStats {
  deliveriesToday: number;
  deliveriesWeek: number;
  deliveriesMonth: number;
  totalDistance?: number;
  averageRating?: number;
}

export interface Vehicle {
  id: string;
  plateNumber: string;
  type: string;
  capacity?: string;
}

export interface Driver {
  id: string;
  name: string;
  phone: string;
  email?: string;
  vehicle?: Vehicle;
}
