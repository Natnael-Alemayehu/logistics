export interface POD {
  id: string;
  shipmentId: string;
  driverId: string;
  photos: PODPhoto[];
  signature?: PODSignature;
  notes?: string;
  recipientName?: string;
  recipientSignature?: boolean;
  deliveredAt: Date;
  createdAt: Date;
}

export interface PODPhoto {
  id: string;
  uri: string;
  timestamp: Date;
  location?: {
    latitude: number;
    longitude: number;
  };
}

export interface PODSignature {
  id: string;
  uri: string;
  timestamp: Date;
}
