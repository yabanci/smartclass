import clsx from 'clsx';
import { InputHTMLAttributes, forwardRef } from 'react';

interface Props extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: string;
  error?: string;
}

export const Input = forwardRef<HTMLInputElement, Props>(function Input(
  { label, hint, error, className, ...rest },
  ref,
) {
  return (
    <label className="block">
      {label ? (
        <span className="mb-1 block text-xs font-semibold text-slate-600">{label}</span>
      ) : null}
      <input
        ref={ref}
        className={clsx('input-field', error && 'border-danger focus:border-danger', className)}
        {...rest}
      />
      {error ? <span className="mt-1 block text-xs text-danger">{error}</span> : null}
      {!error && hint ? <span className="mt-1 block text-xs text-slate-400">{hint}</span> : null}
    </label>
  );
});
