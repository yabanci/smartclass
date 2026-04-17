import clsx from 'clsx';
import { ButtonHTMLAttributes, forwardRef } from 'react';

type Variant = 'primary' | 'ghost' | 'danger' | 'soft';

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

const styles: Record<Variant, string> = {
  primary: 'neu-btn-active text-white font-semibold',
  ghost: 'neu-btn text-primary font-medium',
  danger: 'bg-danger text-white font-semibold hover:bg-red-600',
  soft: 'bg-white/70 text-primary font-medium border border-primary/10 hover:bg-white',
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = 'primary', className, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      className={clsx(
        'rounded-xl px-4 py-2.5 text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed',
        styles[variant],
        className,
      )}
      {...rest}
    />
  );
});
