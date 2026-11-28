// Muestra la cuenta de la sesión y limita el perfil al MVP.
import { User, LogOut } from 'lucide-react';
import { Navigate, useNavigate } from 'react-router-dom';
import { useAuth } from '@/features/autenticacion/useAuth';

export default function Profile() {
    const { user, isGuest, logout, isLoggingOut } = useAuth();
    const navegar = useNavigate();

    if (isGuest || !user) {
        return <Navigate to="/login" replace />;
    }

    return (
        <div className="flex flex-col bg-background font-sans text-foreground relative">

            <main className="p-4 pb-24 sm:px-8 sm:pt-8 md:pb-16 bg-background">
                <div className="max-w-[1600px] mx-auto w-full mb-8">
                    <h1 className="text-3xl font-extrabold tracking-tight text-foreground">Perfil y Configuración</h1>
                </div>

                <div className="max-w-[1600px] mx-auto w-full">
                    <div className="bg-card border border-border rounded-3xl p-6 sm:p-10 shadow-sm space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-300">
                                <h2 className="text-2xl font-bold">Información de Cuenta</h2>
                                
                                {/* Mantiene fondo y texto del mismo par de colores para conservar el contraste del perfil. */}
                                <section className="bg-primary rounded-2xl p-6 text-primary-foreground shadow-lg flex flex-col sm:flex-row sm:items-center gap-5">
                                    <div className="h-16 w-16 bg-white/20 rounded-full flex items-center justify-center shrink-0 border-2 border-white/30">
                                        <User className="h-8 w-8 text-primary-foreground" />
                                    </div>
                                    <div className="flex-1">
                                        <h3 className="text-xl font-bold mb-1">{user.nombre_completo}</h3>
                                        <p className="text-sm text-primary-foreground/90">{user.correo}</p>
                                    </div>
                                </section>

                                <section className="space-y-6 pt-6 border-t border-border">
                                    <h3 className="text-sm font-bold uppercase tracking-wider text-muted-foreground">Datos Personales</h3>
                                    <div className="space-y-4">
                                        <div className="grid gap-4 sm:grid-cols-2">
                                            <div className="space-y-1.5">
                                                <label className="text-xs font-bold text-muted-foreground">Nombre Completo</label>
                                                <input type="text" value={user.nombre_completo} readOnly className="w-full bg-background border border-border rounded-xl px-4 py-2.5 text-sm outline-none focus:ring-2 focus:ring-ring" />
                                            </div>
                                            <div className="space-y-1.5">
                                                <label className="text-xs font-bold text-muted-foreground">Correo Electrónico</label>
                                                <input type="email" defaultValue={user.correo} disabled className="w-full bg-background border border-border rounded-xl px-4 py-2.5 text-sm outline-none focus:ring-2 focus:ring-ring opacity-70 cursor-not-allowed" />
                                            </div>
                                        </div>
                                        <p className="text-xs text-muted-foreground">Tu acceso se gestiona con tu cuenta de Google. La edición de datos personales no está disponible en esta vista.</p>
                                        <div className="pt-2">
                                            <button
                                                type="button"
                                                disabled={Boolean(isLoggingOut)}
                                                onClick={async () => { await logout(); navegar('/login'); }}
                                                className="inline-flex items-center gap-2 rounded-xl border-2 border-destructive/40 bg-destructive/10 px-4 py-2.5 text-sm font-bold text-destructive hover:bg-destructive/20 transition-colors disabled:opacity-50 cursor-pointer"
                                            >
                                                <LogOut className="h-4 w-4" aria-hidden="true" />
                                                {isLoggingOut ? 'Cerrando sesión…' : 'Cerrar sesión'}
                                            </button>
                                        </div>
                                    </div>
                                </section>
                    </div>
                </div>
            </main>
        </div>
    );
}
