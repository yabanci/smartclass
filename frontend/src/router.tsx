import { createBrowserRouter, Navigate } from 'react-router-dom';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { AppShell } from '@/components/AppShell';
import { LoginPage } from '@/features/auth/LoginPage';
import { RegisterPage } from '@/features/auth/RegisterPage';
import { HomePage } from '@/features/home/HomePage';
import { DevicesPage } from '@/features/devices/DevicesPage';
import { SchedulePage } from '@/features/schedule/SchedulePage';
import { ScenesPage } from '@/features/scenes/ScenesPage';
import { AnalyticsPage } from '@/features/analytics/AnalyticsPage';
import { NotificationsPage } from '@/features/notifications/NotificationsPage';
import { ProfilePage } from '@/features/profile/ProfilePage';

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  { path: '/register', element: <RegisterPage /> },
  {
    path: '/',
    element: (
      <ProtectedRoute>
        <AppShell />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: <HomePage /> },
      { path: 'devices', element: <DevicesPage /> },
      { path: 'schedule', element: <SchedulePage /> },
      { path: 'scenes', element: <ScenesPage /> },
      { path: 'analytics', element: <AnalyticsPage /> },
      { path: 'notifications', element: <NotificationsPage /> },
      { path: 'profile', element: <ProfilePage /> },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
]);
