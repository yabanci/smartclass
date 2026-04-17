import { ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '@/stores/auth';
import { SplashScreen } from './SplashScreen';

export function ProtectedRoute({ children }: { children: ReactNode }) {
  const status = useAuth((s) => s.status);
  if (status === 'bootstrapping') return <SplashScreen />;
  if (status === 'anonymous') return <Navigate to="/login" replace />;
  return <>{children}</>;
}
