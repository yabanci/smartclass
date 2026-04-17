import { FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { LogOut } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { userApi } from '@/api/endpoints';
import { errorMessage } from '@/api/client';
import { useAuth } from '@/stores/auth';
import { SUPPORTED_LANGS } from '@/i18n';

export function ProfilePage() {
  const { t, i18n } = useTranslation();
  const qc = useQueryClient();
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);
  const setUser = useAuth((s) => s.setUser);

  const [fullName, setFullName] = useState(user?.fullName ?? '');
  const [phone, setPhone] = useState(user?.phone ?? '');
  const [pwMsg, setPwMsg] = useState('');
  const [pwErr, setPwErr] = useState('');
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');

  const saveMut = useMutation({
    mutationFn: () => userApi.update({ fullName, phone }),
    onSuccess: (u) => {
      setUser(u);
      qc.invalidateQueries();
    },
  });

  const langMut = useMutation({
    mutationFn: (lang: string) => userApi.update({ language: lang }),
    onSuccess: (u, lang) => {
      setUser(u);
      i18n.changeLanguage(lang);
    },
  });

  const pwMut = useMutation({
    mutationFn: () => userApi.changePassword({ currentPassword: current, newPassword: next }),
    onSuccess: () => {
      setPwMsg('OK');
      setPwErr('');
      setCurrent('');
      setNext('');
    },
    onError: (e) => {
      setPwErr(errorMessage(e));
      setPwMsg('');
    },
  });

  const submitProfile = (e: FormEvent) => {
    e.preventDefault();
    saveMut.mutate();
  };

  const submitPw = (e: FormEvent) => {
    e.preventDefault();
    pwMut.mutate();
  };

  if (!user) return null;

  return (
    <div className="p-4 flex flex-col gap-4 animate-fadeIn">
      <header className="pt-2">
        <h1 className="text-2xl font-bold text-primary">{t('profile.title')}</h1>
        <p className="text-sm text-slate-500">{user.email}</p>
      </header>

      <Card>
        <form onSubmit={submitProfile} className="flex flex-col gap-3">
          <Input label={t('auth.fullName')} value={fullName} onChange={(e) => setFullName(e.target.value)} minLength={2} />
          <Input label={t('profile.phone')} value={phone} onChange={(e) => setPhone(e.target.value)} />
          <Button type="submit" disabled={saveMut.isPending}>{t('common.save')}</Button>
        </form>
      </Card>

      <Card>
        <p className="font-semibold text-primary mb-2">{t('profile.language')}</p>
        <div className="flex gap-2">
          {SUPPORTED_LANGS.map((l) => (
            <button
              key={l}
              onClick={() => langMut.mutate(l)}
              className={`flex-1 py-2 rounded-xl text-sm font-semibold uppercase ${
                i18n.language === l ? 'neu-btn-active' : 'neu-btn text-primary'
              }`}
            >
              {l}
            </button>
          ))}
        </div>
      </Card>

      <Card>
        <p className="font-semibold text-primary mb-2">{t('profile.changePassword')}</p>
        <form onSubmit={submitPw} className="flex flex-col gap-3">
          <Input type="password" label={t('profile.currentPassword')} value={current} onChange={(e) => setCurrent(e.target.value)} required />
          <Input type="password" label={t('profile.newPassword')} value={next} onChange={(e) => setNext(e.target.value)} required minLength={8} />
          {pwErr && <p className="text-sm text-danger">{pwErr}</p>}
          {pwMsg && <p className="text-sm text-accent">{pwMsg}</p>}
          <Button type="submit" variant="ghost" disabled={pwMut.isPending}>{t('common.save')}</Button>
        </form>
      </Card>

      <Button variant="danger" onClick={logout}>
        <LogOut size={16} className="inline -mt-0.5 mr-2" />
        {t('auth.logout')}
      </Button>
    </div>
  );
}
