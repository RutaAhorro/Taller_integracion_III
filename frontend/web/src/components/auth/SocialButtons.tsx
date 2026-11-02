import { useGoogleAuth } from "@/hooks/useGoogleAuth"
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
        <GoogleLogin
          onSuccess={handleGoogleSuccess}
          onError={handleGoogleError}
          useOneTap={false}
          theme="outline"
          shape="pill"
          text="signin_with"
          size="large"
          // GSI solo acepta un ancho en píxeles (200-400); "100%" lo invalida.
          width={320}
        />
      </div>
    </div>
  )
}
