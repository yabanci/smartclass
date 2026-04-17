import { FormEvent, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { SplashScreen } from '@/components/SplashScreen';
import { useAuth } from '@/stores/auth';
import { errorMessage } from '@/api/client';

export function LoginPage() {
  const { t } = useTranslation();
  const status = useAuth((s) => s.status);
  const login = useAuth((s) => s.login);

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  if (status === 'bootstrapping') return <SplashScreen />;
  if (status === 'authenticated') return <Navigate to="/" replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr('');
    setLoading(true);
    try {
      await login(email, password);
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto w-full h-full flex flex-col" style={{ maxWidth: 480 }}>
      <div
        className="flex-1 flex flex-col items-center justify-center px-8 py-10"
        style={{ background: 'linear-gradient(160deg, #1E3A8A 0%, #0f2560 40%, #06B6D4 100%)' }}
      >
        <div className="w-20 h-20 rounded-3xl bg-white/20 flex items-center justify-center mb-4 animate-glow text-white">
          <Sparkles size={36} />
        </div>
        <h1 className="text-2xl font-bold text-white">{t('home.title')}</h1>
      </div>
      <form onSubmit={submit} className="bg-white p-6 rounded-t-3xl -mt-6 flex flex-col gap-4">
        <h2 className="text-xl font-bold text-primary">{t('auth.login')}</h2>
        <Input type="email" autoComplete="email" label={t('auth.email')} value={email} onChange={(e) => setEmail(e.target.value)} required />
        <Input type="password" autoComplete="current-password" label={t('auth.password')} value={password} onChange={(e) => setPassword(e.target.value)} required />
        {err && <p className="text-sm text-danger">{err}</p>}
        <Button type="submit" disabled={loading}>{loading ? t('common.loading') : t('auth.login')}</Button>
        <p className="text-center text-sm text-slate-500">
          {t('auth.noAccount')}{' '}
          <Link to="/register" className="text-primary font-semibold">
            {t('auth.register')}
          </Link>
        </p>
      </form>
    </div>
  );
}
