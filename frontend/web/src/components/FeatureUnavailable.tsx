import { Link } from 'react-router-dom';
import { ROUTES } from '@/core/routes';

type FeatureUnavailableProps = {
  title: string;
  description: string;
};

export function FeatureUnavailable({ title, description }: FeatureUnavailableProps) {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4 p-6 text-center">
      <h1 className="text-2xl font-bold">{title}</h1>
      <p role="status" className="max-w-lg text-sm text-muted-foreground">{description}</p>
      <Link to={ROUTES.LISTAS} className="text-sm font-semibold text-primary underline">
        Ir a mis listas de compras
      </Link>
    </main>
  );
}
