// Muestra la cuenta de la sesión y avisa qué ajustes del perfil todavía no tienen integración.
import { useState } from 'react';
import { User, ShieldAlert, Zap, Settings } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Navigate } from 'react-router-dom';
import { useAuth } from '@/features/autenticacion/useAuth';
import { PLAN_PREMIUM } from '@/features/autenticacion/tipos';

export default function Profile() {
    const { user, isGuest } = useAuth();
    const [pestanaActiva, setPestanaActiva] = useState<'cuenta' | 'preferencias'>('cuenta');

    if (isGuest || !user) {
        return <Navigate to="/login" replace />;
    }

    // El plan (basico/premium) es independiente del rol de la cuenta.
    const esPremium = user.plan === PLAN_PREMIUM;

    return (
        <div className="flex flex-col bg-background font-sans text-foreground relative">

            <main className="p-4 sm:p-8 bg-background">
                <div className="max-w-[1600px] mx-auto w-full mb-8">
                    <h1 className="text-3xl font-extrabold tracking-tight text-foreground">Perfil y Configuración</h1>
                </div>

                <div className="max-w-[1600px] mx-auto flex flex-col md:flex-row gap-8 md:gap-12 w-full">
                    
                    <aside className="w-full md:w-64 shrink-0">
                        <nav className="flex flex-row md:flex-col gap-3 overflow-x-auto pb-2 md:pb-0 scrollbar-hide">
                            {[
                                { id: 'cuenta', label: 'Mi Cuenta', icon: User },
                                { id: 'preferencias', label: 'Preferencias de Ruta', icon: Settings }
                            ].map(pestana => (
                                <button 
                                    key={pestana.id} 
                                    onClick={() => setPestanaActiva(pestana.id as any)}
                                    className={cn("flex items-center gap-3 px-5 py-4 rounded-xl font-bold transition-all whitespace-nowrap", pestanaActiva === pestana.id ? "bg-primary text-primary-foreground shadow-[2px_2px_0px_var(--color-border)]" : "hover:bg-accent text-muted-foreground")}
                                >
                                    <pestana.icon className="h-5 w-5" />
                                    {pestana.label}
                                </button>
                            ))}
                        </nav>
                    </aside>

                    <div className="flex-1 min-w-0">
                        
                        {pestanaActiva === 'cuenta' && (
                            <div className="bg-card border border-border rounded-3xl p-6 sm:p-10 shadow-sm space-y-8 animate-in fade-in slide-in-from-bottom-2 duration-300">
                                <h2 className="text-2xl font-bold">Información de Cuenta</h2>
                                
                                {/* Mantiene fondo y texto del mismo par de colores para conservar el contraste del perfil. */}
                                <section className="bg-primary rounded-2xl p-6 text-primary-foreground shadow-lg flex flex-col sm:flex-row sm:items-center gap-5">
                                    <div className={`h-16 w-16 bg-white/20 rounded-full flex items-center justify-center shrink-0 border-2 border-white/30 ${esPremium ? 'border-amber-400 bg-amber-500/20' : ''}`}>
                                        {esPremium ? <Zap className="h-8 w-8 text-amber-500" /> : <User className="h-8 w-8 text-primary-foreground" />}
                                    </div>
                                    <div className="flex-1">
                                        <div className="flex items-center gap-2 mb-1">
                                            <h3 className="text-xl font-bold">{user.nombre_completo}</h3>
                                            <span className={`px-2.5 py-0.5 rounded-full text-[0.65rem] uppercase tracking-wider font-bold ${esPremium ? 'bg-amber-100 text-amber-700' : 'bg-white/20 text-primary-foreground'}`}>
                                                {esPremium ? 'Premium' : 'Básico'}
                                            </span>
                                        </div>
                                        <p className="text-primary-foreground/90 text-sm flex items-center gap-1.5 mb-2">
                                            <ShieldAlert className="h-4 w-4" /> Cuota informada: {user.cuota_tokens_ia} consultas IA
                                        </p>
                                        <p className="text-sm text-primary-foreground/80">{user.correo}</p>
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
                                    </div>
                                </section>
                            </div>
                        )}

                        {pestanaActiva === 'preferencias' && (
                            <div className="bg-card border border-border rounded-3xl p-6 sm:p-10 shadow-sm space-y-4">
                                <h2 className="text-2xl font-bold">Preferencias de Ruta</h2>
                                <p role="status" className="text-muted-foreground text-sm">
                                    Las preferencias de transporte, presupuesto y alimentación aún no están integradas. No hay valores guardados para mostrar o modificar desde esta vista.
                                </p>
                            </div>
                        )}

                        
                    </div>
                </div>
            </main>
        </div>
    );
}
