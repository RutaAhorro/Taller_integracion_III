// Informa que la ruta no existe y ofrece volver al inicio.
import { Link } from 'react-router-dom';
import { ROUTES } from '@/core/routes';
import { Button } from '@/components/ui/button';

export default function NotFound() {
  return (
    <div className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-4 px-6 text-center">
      <span className="font-mono text-6xl font-bold text-muted-foreground">404</span>
      <h1 className="text-xl font-semibold">La página que buscas no existe</h1>
      <p className="text-sm text-muted-foreground">
        Puede que haya sido movida o que el enlace esté mal escrito.
      </p>
      <Link to={ROUTES.HOME}>
        <Button className="mt-2">Volver al inicio</Button>
      </Link>
    </div>
  );
}
