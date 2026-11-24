// Prepara la presentación de SUP-249 sin consumir tokens ni fingir respuestas de SUP-231.
import { ArrowLeft, CircleCheck, CircleX, Clock3, Loader2, Mail, MailCheck, ShoppingBasket } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { ROUTES } from '@/app/routes';
import { buttonVariants } from '@/components/ui/button';
import { AuthVisual } from '@/features/autenticacion/AuthVisual';

// ===== Estados visuales para la futura integración =====

/** Los estados de éxito y correo enviado sólo deberán proceder de una confirmación real de Identidad. */
export type EstadoVerificacionEmail =
  | 'sin_token'
  | 'pendiente'
  | 'verificando'
  | 'confirmado'
  | 'enlace_invalido'
  | 'error'
  | 'correo_enviado';

/** Mantiene los mensajes independientes del futuro contrato HTTP y no contiene datos del token. */
const PRESENTACIONES = {
  sin_token: {
    icono: Mail,
    titulo: 'El enlace está incompleto',
    descripcion: 'Falta el token de verificación. Abre el enlace completo del correo, si lo has recibido.',
    detalle: 'No se ha verificado tu correo ni modificado tu cuenta.',
  },
  pendiente: {
    icono: Clock3,
    titulo: 'Verificación aún no disponible',
    descripcion: 'La pantalla está preparada, pero todavía no está conectada al servicio de verificación.',
    detalle: 'Tu enlace no se ha validado ni consumido. Inténtalo cuando la función esté habilitada.',
  },
  verificando: {
    icono: Loader2,
    titulo: 'Verificando tu correo',
    descripcion: 'Estamos comprobando el enlace con el servicio de Identidad.',
    detalle: 'Espera un momento antes de salir de esta página.',
  },
  confirmado: {
    icono: CircleCheck,
    titulo: 'Correo verificado',
    descripcion: 'Tu correo electrónico se ha confirmado correctamente.',
    detalle: 'Puedes volver al inicio de sesión para acceder a tu cuenta.',
  },
  enlace_invalido: {
    icono: CircleX,
    titulo: 'No se pudo verificar el enlace',
    descripcion: 'El servicio de Identidad ha rechazado este enlace de verificación.',
    detalle: 'Comprueba que has abierto el enlace completo del correo más reciente.',
  },
  error: {
    icono: CircleX,
    titulo: 'No pudimos confirmar la verificación',
    descripcion: 'No se pudo obtener una confirmación del servicio de Identidad.',
    detalle: 'Esto no confirma si tu correo quedó verificado. Vuelve a intentarlo cuando el servicio esté disponible.',
  },
  correo_enviado: {
    icono: MailCheck,
    titulo: 'Revisa tu correo',
    descripcion: 'Te hemos enviado un enlace para confirmar tu dirección de correo electrónico.',
    detalle: 'Revisa también la carpeta de spam. Abre el enlace recibido para completar la verificación.',
  },
} satisfies Record<
  EstadoVerificacionEmail,
  {
    icono: typeof Mail;
    titulo: string;
    descripcion: string;
    detalle: string;
  }
>;

// ===== Presentación reutilizable =====

/** Presenta un estado confirmado por el consumidor; no hace peticiones ni redirecciones automáticas. */
export function EstadoVerificacion({ estado }: { estado: EstadoVerificacionEmail }) {
  const presentacion = PRESENTACIONES[estado];
  const Icono = presentacion.icono;
  const verificando = estado === 'verificando';
  const esError = estado === 'enlace_invalido' || estado === 'error' || estado === 'sin_token';

  return (
    <section
      aria-labelledby="titulo-verificacion"
      aria-busy={verificando}
      className="w-full max-w-md rounded-2xl border-2 border-border bg-card p-6 shadow-[6px_6px_0px_var(--color-border)] sm:p-8"
    >
      <div role={esError ? 'alert' : 'status'} aria-atomic="true" className="flex flex-col items-center text-center">
        <span
          className={`mb-6 flex size-16 items-center justify-center rounded-2xl ${
            esError ? 'bg-destructive/10 text-destructive' : 'bg-primary/10 text-primary'
          }`}
        >
          <Icono aria-hidden="true" className={`size-8 ${verificando ? 'motion-safe:animate-spin' : ''}`} />
        </span>
        <p className="mb-2 text-xs font-bold uppercase tracking-widest text-muted-foreground">Confirmación de correo</p>
        <h1 id="titulo-verificacion" className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
          {presentacion.titulo}
        </h1>
        <p className="mt-4 text-sm leading-relaxed text-foreground">{presentacion.descripcion}</p>
        <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{presentacion.detalle}</p>
      </div>
      <Link
        to={ROUTES.LOGIN}
        referrerPolicy="no-referrer"
        className={`${buttonVariants({ variant: 'outline' })} mt-8 w-full`}
      >
        <ArrowLeft aria-hidden="true" className="size-4" />
        Volver al inicio de sesión
      </Link>
    </section>
  );
}

// ===== Ruta pública =====

/** Sólo distingue la presencia del token; ningún parámetro de URL puede simular éxito ni envío de correo. */
export default function VerificarEmail() {
  const [parametros] = useSearchParams();
  const tieneToken = Boolean(parametros.get('token')?.trim());

  return (
    <main className="grid min-h-dvh grid-cols-1 bg-background lg:grid-cols-2">
      <AuthVisual />
      <div className="flex min-w-0 flex-col items-center justify-center gap-8 px-6 py-12 sm:px-10">
        <Link
          to={ROUTES.HOME}
          referrerPolicy="no-referrer"
          className="flex items-center gap-2 font-display text-lg font-bold lg:hidden"
        >
          <ShoppingBasket aria-hidden="true" className="size-6 text-primary" />
          RutaAhorro
        </Link>
        <EstadoVerificacion estado={tieneToken ? 'pendiente' : 'sin_token'} />
      </div>
    </main>
  );
}
