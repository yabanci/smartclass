import { FormEvent, useState } from 'react';
import { Link, Navigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { useAuth } from '@/stores/auth';
import { errorMessage } from '@/api/client';

export function RegisterPage() {
  const { t, i18n } = useTranslation();
  const status = useAuth((s) => s.status);
  const register = useAuth((s) => s.register);

  const [email, setEmail] = useState('');
  const [fullName, setFullName] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [role, setRole] = useState('teacher');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  if (status === 'authenticated') return <Navigate to="/" replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      setErr(t('auth.passwordMismatch'));
      return;
    }
    setErr('');
    setLoading(true);
    try {
      await register({ email, password, fullName, role, language: i18n.language });
    } catch (e) {
      setErr(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto w-full h-full flex flex-col" style={{ maxWidth: 480 }}>
      <div
        className="flex flex-col items-center justify-center px-8 py-10"
        style={{ background: 'linear-gradient(160deg, #1E3A8A 0%, #0f2560 40%, #06B6D4 100%)' }}
      >
        <div className="w-16 h-16 rounded-3xl bg-white/20 flex items-center justify-center mb-3 animate-glow text-white">
          <Sparkles size={28} />
        </div>
        <h1 className="text-xl font-bold text-white">{t('auth.register')}</h1>
      </div>
      <form onSubmit={submit} className="bg-white p-6 rounded-t-3xl -mt-6 flex flex-col gap-3 overflow-y-auto">
        <Input label={t('auth.fullName')} value={fullName} onChange={(e) => setFullName(e.target.value)} required minLength={2} />
        <Input type="email" autoComplete="email" label={t('auth.email')} value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label className="block">
          <span className="mb-1 block text-xs font-semibold text-slate-600">{t('auth.role')}</span>
          <select className="input-field" value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="teacher">{t('auth.roleTeacher')}</option>
            <option value="admin">{t('auth.roleAdmin')}</option>
            <option value="technician">{t('auth.roleTechnician')}</option>
          </select>
        </label>
        <Input type="password" autoComplete="new-password" label={t('auth.password')} value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
        <Input type="password" autoComplete="new-password" label={t('auth.confirmPassword')} value={confirm} onChange={(e) => setConfirm(e.target.value)} required minLength={8} />
        {err && <p className="text-sm text-danger">{err}</p>}
        <Button type="submit" disabled={loading}>{loading ? t('common.loading') : t('auth.register')}</Button>
        <p className="text-center text-sm text-slate-500">
          {t('auth.haveAccount')}{' '}
          <Link to="/login" className="text-primary font-semibold">
            {t('auth.login')}
          </Link>
        </p>
      </form>
    </div>
  );
}
