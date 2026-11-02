import { CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/utils';

interface SuccessCardProps {
  title: string;
  subtitle?: string;
  className?: string;
}

export function SuccessCard({ title, subtitle, className }: SuccessCardProps) {
  return (
    <div className={cn(
      "flex flex-col items-center justify-center p-6 md:p-8 bg-background border-2 border-border rounded-xl shadow-[4px_4px_0px_var(--color-border)] animate-in zoom-in-95 fade-in duration-300",
      className
    )}>
      <div className="bg-emerald-100 dark:bg-emerald-900/40 p-3 rounded-full mb-4">
        <CheckCircle2 className="h-10 w-10 text-emerald-600 dark:text-emerald-400" />
      </div>
      <h4 className="font-bold text-xl text-foreground text-center mb-1">{title}</h4>
      {subtitle && (
        <p className="text-sm md:text-base text-muted-foreground text-center">{subtitle}</p>
      )}
    </div>
  );
}
