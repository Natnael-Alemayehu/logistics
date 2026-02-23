export interface User {
  id: string;
  email: string;
  name: string;
  role: 'driver' | 'admin' | 'customer';
  phone?: string;
  avatar?: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}
