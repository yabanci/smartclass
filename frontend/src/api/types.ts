export type Role = 'teacher' | 'admin' | 'technician';

export interface User {
  id: string;
  email: string;
  fullName: string;
  role: Role;
  language: string;
  avatarUrl?: string;
  phone?: string;
  birthDate?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Tokens {
  accessToken: string;
  refreshToken: string;
  accessExpiresAt: string;
  refreshExpiresAt: string;
  tokenType: string;
}

export interface AuthResponse {
  user: User;
  tokens: Tokens;
}

export interface Classroom {
  id: string;
  name: string;
  description: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export type DeviceStatus = 'on' | 'off' | 'open' | 'closed' | 'unknown';

export interface Device {
  id: string;
  classroomId: string;
  name: string;
  type: string;
  brand: string;
  driver: string;
  config: Record<string, unknown>;
  status: DeviceStatus;
  online: boolean;
  lastSeenAt?: string;
  createdAt: string;
  updatedAt: string;
}

export type CommandType = 'ON' | 'OFF' | 'OPEN' | 'CLOSE' | 'SET_VALUE';

export interface Lesson {
  id: string;
  classroomId: string;
  subject: string;
  teacherId?: string;
  dayOfWeek: number;
  startsAt: string;
  endsAt: string;
  notes: string;
  createdAt: string;
  updatedAt: string;
}

export type WeekSchedule = Record<string, Lesson[]>;

export interface Scene {
  id: string;
  classroomId: string;
  name: string;
  description: string;
  steps: SceneStep[];
  createdAt: string;
  updatedAt: string;
}

export interface SceneStep {
  deviceId: string;
  command: CommandType;
  value?: unknown;
}

export interface SensorReading {
  id?: number;
  deviceId: string;
  metric: string;
  value: number;
  unit?: string;
  recordedAt: string;
}

export interface Notification {
  id: string;
  userId: string;
  classroomId?: string;
  type: 'info' | 'warning' | 'error';
  title: string;
  message: string;
  metadata?: Record<string, unknown>;
  readAt?: string;
  createdAt: string;
}

export interface TimePoint {
  bucket: string;
  avg: number;
  min: number;
  max: number;
  count: number;
}

export interface DeviceUsage {
  deviceId: string;
  commandCount: number;
}
