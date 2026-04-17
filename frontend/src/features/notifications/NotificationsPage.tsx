import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Info, AlertCircle, Check } from 'lucide-react';
import clsx from 'clsx';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { notificationApi } from '@/api/endpoints';
import type { Notification } from '@/api/types';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useAuth } from '@/stores/auth';
import { useMemo } from 'react';

export function NotificationsPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const user = useAuth((s) => s.user);

  const listQ = useQuery({ queryKey: ['notifications'], queryFn: () => notificationApi.list({ limit: 100 }) });

  const topics = useMemo(() => (user ? [`user:${user.id}:notifications`] : []), [user]);
  useWebSocket(topics, () => {
    qc.invalidateQueries({ queryKey: ['notifications'] });
    qc.invalidateQueries({ queryKey: ['notif-unread'] });
  });

  const markRead = useMutation({
    mutationFn: (id: string) => notificationApi.markRead(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notifications'] });
      qc.invalidateQueries({ queryKey: ['notif-unread'] });
    },
  });

  const markAll = useMutation({
    mutationFn: () => notificationApi.markAllRead(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notifications'] });
      qc.invalidateQueries({ queryKey: ['notif-unread'] });
    },
  });

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="flex items-center justify-between pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('notifications.title')}</h1>
        <Button variant="ghost" onClick={() => markAll.mutate()}>
          {t('notifications.markAllRead')}
        </Button>
      </header>

      {listQ.data && listQ.data.length === 0 && (
        <Card className="text-center text-slate-500">{t('notifications.empty')}</Card>
      )}

      <div className="flex flex-col gap-2">
        {(listQ.data ?? []).map((n) => (
          <NotificationItem key={n.id} n={n} onRead={() => markRead.mutate(n.id)} />
        ))}
      </div>
    </div>
  );
}

function NotificationItem({ n, onRead }: { n: Notification; onRead: () => void }) {
  const Icon = n.type === 'error' ? AlertCircle : n.type === 'warning' ? AlertTriangle : Info;
  const color =
    n.type === 'error' ? 'text-danger' : n.type === 'warning' ? 'text-warn' : 'text-secondary';

  return (
    <Card className={clsx(!n.readAt && 'border-l-4 border-l-secondary')}>
      <div className="flex items-start gap-3">
        <Icon className={clsx('mt-0.5 shrink-0', color)} size={18} />
        <div className="flex-1">
          <p className="font-semibold text-primary">{n.title}</p>
          <p className="text-sm text-slate-600">{n.message}</p>
          <p className="text-xs text-slate-400 mt-1">{new Date(n.createdAt).toLocaleString()}</p>
        </div>
        {!n.readAt && (
          <button onClick={onRead} className="p-1.5 rounded-full text-slate-400 hover:text-accent">
            <Check size={16} />
          </button>
        )}
      </div>
    </Card>
  );
}
