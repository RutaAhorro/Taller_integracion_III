// Conecta el botón oficial de Google con el flujo de autenticación de la aplicación.
import { useGoogleAuth } from "@/features/autenticacion/useGoogleAuth"
import { GoogleLogin } from '@react-oauth/google';

export function SocialButtons() {
  const { handleGoogleSuccess, handleGoogleError, error } = useGoogleAuth();

  return (
    <div className="grid grid-cols-1 gap-3">
      {error && (
        <div className="rounded-lg bg-destructive/10 p-2 text-xs text-destructive font-semibold border border-destructive text-center">
          {error}
        </div>
      )}
      <div className="flex justify-center w-full">
        <div className="inline-flex rounded-xl border-2 border-border bg-white p-1.5 shadow-[4px_4px_0px_var(--color-border)]">
          <GoogleLogin
            onSuccess={handleGoogleSuccess}
            onError={handleGoogleError}
            useOneTap={false}
            theme="outline"
            shape="rectangular"
            text="signin_with"
            size="large"
            // GSI solo acepta un ancho en píxeles (200-400); "100%" lo invalida.
            width={280}
          />
        </div>
      </div>
    </div>
  )
}
