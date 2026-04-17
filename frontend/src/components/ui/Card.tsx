import clsx from 'clsx';
import { HTMLAttributes } from 'react';

export function Card({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={clsx('glass rounded-2xl p-4 soft-shadow', className)} {...rest} />;
}
