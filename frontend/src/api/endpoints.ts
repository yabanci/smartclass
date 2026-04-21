import { client, extract, Envelope } from './client';
import type {
  AuthResponse, User, Classroom, Device, Lesson, WeekSchedule,
  Scene, SensorReading, Notification, TimePoint, DeviceUsage, CommandType,
} from './types';

const unwrap = async <T>(p: Promise<{ data: Envelope<T> }>): Promise<T> => extract((await p).data);

export const authApi = {
  register: (body: { email: string; password: string; fullName: string; role: string; language?: string; phone?: string }) =>
    unwrap<AuthResponse>(client.post('/auth/register', body)),
  login: (body: { email: string; password: string }) =>
    unwrap<AuthResponse>(client.post('/auth/login', body)),
  refresh: (refreshToken: string) =>
    unwrap<AuthResponse>(client.post('/auth/refresh', { refreshToken })),
};

export const userApi = {
  me: () => unwrap<User>(client.get('/users/me')),
  update: (body: Partial<Pick<User, 'fullName' | 'language' | 'avatarUrl' | 'phone'>>) =>
    unwrap<User>(client.patch('/users/me', body)),
  changePassword: (body: { currentPassword: string; newPassword: string }) =>
    client.post('/users/me/password', body),
};

export const classroomApi = {
  list: () => unwrap<Classroom[]>(client.get('/classrooms')),
  get: (id: string) => unwrap<Classroom>(client.get(`/classrooms/${id}`)),
  create: (body: { name: string; description?: string }) =>
    unwrap<Classroom>(client.post('/classrooms', body)),
  update: (id: string, body: { name?: string; description?: string }) =>
    unwrap<Classroom>(client.patch(`/classrooms/${id}`, body)),
  remove: (id: string) => client.delete(`/classrooms/${id}`),
};

export const deviceApi = {
  listByClassroom: (classroomID: string) =>
    unwrap<Device[]>(client.get(`/classrooms/${classroomID}/devices`)),
  get: (id: string) => unwrap<Device>(client.get(`/devices/${id}`)),
  create: (body: {
    classroomId: string; name: string; type: string; brand: string; driver: string;
    config?: Record<string, unknown>;
  }) => unwrap<Device>(client.post('/devices', body)),
  update: (id: string, body: Partial<{ name: string; type: string; brand: string; driver: string; config: Record<string, unknown> }>) =>
    unwrap<Device>(client.patch(`/devices/${id}`, body)),
  remove: (id: string) => client.delete(`/devices/${id}`),
  command: (id: string, cmd: { type: CommandType; value?: unknown }) =>
    unwrap<Device>(client.post(`/devices/${id}/commands`, cmd)),
};

export const scheduleApi = {
  week: (classroomID: string) =>
    unwrap<WeekSchedule>(client.get(`/classrooms/${classroomID}/schedule`)),
  day: (classroomID: string, day: number) =>
    unwrap<Lesson[]>(client.get(`/classrooms/${classroomID}/schedule/day/${day}`)),
  current: (classroomID: string) =>
    unwrap<Lesson | null>(client.get(`/classrooms/${classroomID}/schedule/current`)),
  create: (body: { classroomId: string; subject: string; dayOfWeek: number; startsAt: string; endsAt: string; notes?: string; teacherId?: string }) =>
    unwrap<Lesson>(client.post('/schedule', body)),
  update: (id: string, body: Partial<{ subject: string; dayOfWeek: number; startsAt: string; endsAt: string; notes: string }>) =>
    unwrap<Lesson>(client.patch(`/schedule/${id}`, body)),
  remove: (id: string) => client.delete(`/schedule/${id}`),
};

export const sceneApi = {
  listByClassroom: (classroomID: string) =>
    unwrap<Scene[]>(client.get(`/classrooms/${classroomID}/scenes`)),
  get: (id: string) => unwrap<Scene>(client.get(`/scenes/${id}`)),
  create: (body: { classroomId: string; name: string; description?: string; steps: Scene['steps'] }) =>
    unwrap<Scene>(client.post('/scenes', body)),
  update: (id: string, body: Partial<{ name: string; description: string; steps: Scene['steps'] }>) =>
    unwrap<Scene>(client.patch(`/scenes/${id}`, body)),
  remove: (id: string) => client.delete(`/scenes/${id}`),
  run: (id: string) => unwrap<{ sceneId: string; steps: Array<{ step: Scene['steps'][0]; success: boolean; error?: string }> }>(client.post(`/scenes/${id}/run`)),
};

export const sensorApi = {
  latest: (classroomID: string) =>
    unwrap<SensorReading[]>(client.get(`/classrooms/${classroomID}/sensors/readings/latest`)),
  history: (deviceID: string, params: { metric?: string; from?: string; to?: string; limit?: number }) =>
    unwrap<SensorReading[]>(client.get(`/devices/${deviceID}/sensors/readings`, { params })),
  ingest: (readings: Array<{ deviceId: string; metric: string; value: number; unit?: string }>) =>
    unwrap<{ accepted: number }>(client.post('/sensors/readings', { readings })),
};

export const notificationApi = {
  list: (opts?: { unread?: boolean; limit?: number }) =>
    unwrap<Notification[]>(client.get('/notifications', {
      params: { unread: opts?.unread ? 'true' : undefined, limit: opts?.limit },
    })),
  unreadCount: () =>
    unwrap<{ count: number }>(client.get('/notifications/unread-count')),
  markRead: (id: string) => client.post(`/notifications/${id}/read`),
  markAllRead: () => client.post('/notifications/read-all'),
};

export type HassFlowHandler = {
  domain: string;
  name: string;
  integration?: string;
  iot_class?: string;
  config_flow: boolean;
};

export type HassSchemaField = {
  name: string;
  type: string;
  required?: boolean;
  optional?: boolean;
  default?: unknown;
  // HA serializes enum options three ways depending on how the integration
  // built the voluptuous schema — flat array (["cn", "sg"]), array of pairs
  // ([["cn", "China"]]), or a dict ({"cn": "China"}). xiaomi_home uses the
  // dict form for `cloud_server`, so we keep this `unknown` and normalize
  // in the SchemaFieldInput renderer.
  options?: unknown;
  // Voluptuous-serialize emits type:"multi_select" for cv.multi_select, and
  // selector-based fields surface `multiple: true`. Either means HA expects
  // a JSON list back on submit (xiaomi_home's `home_infos` is the canonical
  // case — submitting a scalar fails with "Not a list").
  multiple?: boolean;
};

export type HassFlowStep = {
  flow_id?: string;
  handler?: string;
  type: 'form' | 'create_entry' | 'abort' | 'progress' | 'external_step';
  step_id?: string;
  data_schema?: HassSchemaField[];
  errors?: Record<string, string>;
  description?: string;
  description_placeholders?: Record<string, string>;
  reason?: string;
  title?: string;
  result?: Record<string, unknown>;
};

export type HassEntity = {
  entity_id: string;
  state: string;
  domain: string;
  friendly_name: string;
  attributes?: Record<string, unknown>;
};

export type HassStatus = {
  baseUrl: string;
  configured: boolean;
  onboarded: boolean;
  reason?: string;
};

export const hassApi = {
  status: () => unwrap<HassStatus>(client.get('/hass/status')),
  setToken: (token: string) => unwrap<HassStatus>(client.post('/hass/token', { token })),
  integrations: () => unwrap<HassFlowHandler[]>(client.get('/hass/integrations')),
  startFlow: (handler: string) =>
    unwrap<HassFlowStep>(client.post('/hass/flows', { handler })),
  stepFlow: (flowID: string, data: Record<string, unknown>) =>
    unwrap<HassFlowStep>(client.post(`/hass/flows/${flowID}/step`, { data })),
  abortFlow: (flowID: string) => client.delete(`/hass/flows/${flowID}`),
  entities: () => unwrap<HassEntity[]>(client.get('/hass/entities')),
  adopt: (body: { entityId: string; classroomId: string; name?: string; brand?: string }) =>
    unwrap<Device>(client.post('/hass/adopt', body)),
};

export const analyticsApi = {
  sensors: (classroomID: string, params: { metric: string; bucket?: 'hour' | 'day' | 'week' | 'month'; from?: string; to?: string }) =>
    unwrap<TimePoint[]>(client.get(`/classrooms/${classroomID}/analytics/sensors`, { params })),
  usage: (classroomID: string, params?: { from?: string; to?: string }) =>
    unwrap<DeviceUsage[]>(client.get(`/classrooms/${classroomID}/analytics/usage`, { params })),
  energy: (classroomID: string, params?: { from?: string; to?: string }) =>
    unwrap<{ total: number }>(client.get(`/classrooms/${classroomID}/analytics/energy`, { params })),
};
