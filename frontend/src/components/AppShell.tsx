import { NavLink, Outlet } from 'react-router-dom';
import { Home, Cpu, Calendar, BarChart2, User } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';

export function AppShell() {
  const { t } = useTranslation();
  const items = [
    { to: '/', icon: Home, label: t('nav.home'), end: true },
    { to: '/devices', icon: Cpu, label: t('nav.devices') },
    { to: '/schedule', icon: Calendar, label: t('nav.schedule') },
    { to: '/analytics', icon: BarChart2, label: t('nav.analytics') },
    { to: '/profile', icon: User, label: t('nav.profile') },
  ];

  return (
    <div className="mx-auto w-full h-full flex flex-col relative overflow-hidden" style={{ maxWidth: 480 }}>
      <div className="flex-1 overflow-y-auto overflow-x-hidden pb-20">
        <Outlet />
      </div>
      <nav className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-[480px] border-t border-slate-200 bg-white/90 backdrop-blur px-2 py-1.5">
        <div className="flex items-center justify-between">
          {items.map(({ to, icon: Icon, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                clsx(
                  'flex-1 flex flex-col items-center gap-0.5 py-1.5 rounded-lg text-xs transition-colors',
                  isActive ? 'text-primary' : 'text-slate-400',
                )
              }
            >
              <Icon size={20} />
              <span>{label}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </div>
  );
}
