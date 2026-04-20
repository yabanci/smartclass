import { NavLink, Outlet, Link } from 'react-router-dom';
import { Home, Cpu, Calendar, BarChart2, User, Bell, Moon, Sun } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { notificationApi } from '@/api/endpoints';
import { useTheme } from '@/stores/theme';

export function AppShell() {
  const { t } = useTranslation();
  const mode = useTheme((s) => s.mode);
  const toggle = useTheme((s) => s.toggle);
  const unreadQ = useQuery({ queryKey: ['notif-unread'], queryFn: () => notificationApi.unreadCount() });

  const items = [
    { to: '/', icon: Home, label: t('nav.home'), end: true },
    { to: '/devices', icon: Cpu, label: t('nav.devices') },
    { to: '/schedule', icon: Calendar, label: t('nav.schedule') },
    { to: '/analytics', icon: BarChart2, label: t('nav.analytics') },
    { to: '/profile', icon: User, label: t('nav.profile') },
  ];

  return (
    <div
      className="mx-auto w-full h-full flex flex-col relative overflow-hidden"
      style={{ maxWidth: 480 }}
    >
      <header className="flex items-center justify-between px-5 pt-4 pb-2 flex-shrink-0">
        <div className="min-w-0">
          <h1 className="text-lg font-extrabold text-primary truncate">{t('home.title')}</h1>
          <p className="text-xs text-slate-500 dark:text-slate-400 truncate">IoT</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={toggle}
            className="w-9 h-9 rounded-full bg-white dark:bg-dark-card card-shadow flex items-center justify-center text-slate-600 dark:text-yellow-400"
            aria-label="theme"
          >
            {mode === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
          </button>
          <Link
            to="/notifications"
            className="w-9 h-9 rounded-full bg-white dark:bg-dark-card card-shadow flex items-center justify-center relative text-primary"
            aria-label={t('notifications.title')}
          >
            <Bell size={16} />
            {(unreadQ.data?.count ?? 0) > 0 && (
              <span className="absolute -top-1 -right-1 w-4 h-4 bg-red-500 rounded-full text-[10px] text-white flex items-center justify-center">
                {unreadQ.data?.count}
              </span>
            )}
          </Link>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto overflow-x-hidden pb-20">
        <Outlet />
      </div>

      <nav className="fixed bottom-0 left-1/2 -translate-x-1/2 w-full max-w-[480px] border-t border-slate-200/50 dark:border-slate-700/50 bg-white/90 dark:bg-dark-card/90 backdrop-blur px-2 py-1.5">
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
