import { buttonVariants } from "@/components/ui/button"
import { Link } from "react-router-dom"
import { ArrowLeft, CheckCircle2 } from "lucide-react"

export default function Terms() {
  return (
    <div className="min-h-screen bg-background flex flex-col items-center py-10 px-4 sm:px-6">
      <div className="w-full max-w-3xl space-y-8">
        <Link to="/login" className="inline-flex items-center text-sm font-medium text-muted-foreground hover:text-foreground transition-colors">
          <ArrowLeft className="mr-2 h-4 w-4" />
          Volver al registro
        </Link>

        <div className="rounded-xl border-4 border-border bg-card p-6 sm:p-10 shadow-[8px_8px_0px_var(--color-border)]">
          <h1 className="text-3xl font-display font-black tracking-tight mb-2 text-foreground">
            Términos y Condiciones de Uso — RutaAhorro
          </h1>
          <p className="text-sm font-semibold text-muted-foreground mb-8">
            *Última actualización: 04-10-2026
          </p>

          <div className="prose prose-sm sm:prose-base dark:prose-invert max-w-none space-y-6 text-foreground/90">
            <p>
              Antes de crear tu cuenta, te pedimos leer estos términos. Al marcar la casilla <strong>"He leído y acepto los Términos y Condiciones"</strong> confirmas que estás de acuerdo con lo siguiente.
            </p>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                1. Qué es RutaAhorro
              </h2>
              <p>
                RutaAhorro te ayuda a comparar precios entre supermercados y a calcular si vale la pena desplazarte para aprovechar un ahorro, considerando el costo de transporte. También puedes usar nuestro asistente con Inteligencia Artificial para pedir ideas de recetas y listas de compra.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                2. Quién puede usar RutaAhorro
              </h2>
              <p>
                Para crear una cuenta debes ser mayor de 18 años y tener capacidad legal para aceptar estos términos. Si eres menor de edad, no está permitido registrarte ni usar las funciones que requieren cuenta (listas de compra, asistente de IA, cálculo de rutas). La consulta y comparación de precios sin cuenta no tiene esta restricción.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                3. Tipos de cuenta
              </h2>
              <ul className="list-disc pl-5 space-y-2">
                <li><strong>Invitado:</strong> puedes buscar y comparar precios sin crear una cuenta.</li>
                <li><strong>Usuario registrado:</strong> además puedes guardar listas de compra, usar el asistente de IA, calcular rutas y recibir alertas.</li>
                <li><strong>Colaborador:</strong> usuarios registrados que reportan precios o stock pueden acceder a cuotas ampliadas de IA como recompensa.</li>
              </ul>
              <p className="mt-2">
                Cada tipo de cuenta tiene límites de uso (por ejemplo, cantidad de consultas al asistente de IA o de cálculos de ruta por día). Si superas estos límites, algunas funciones podrían bloquearse temporalmente.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                4. Uso permitido
              </h2>
              <p>
                Puedes usar RutaAhorro para consultar precios, armar tus listas de compra y planificar tus trayectos de compra personales.
              </p>
              <p className="mt-2 font-semibold">No está permitido:</p>
              <ul className="list-disc pl-5 space-y-2 mt-2">
                <li>Usar bots, scrapers, scripts automatizados o herramientas similares para extraer masivamente nuestro catálogo, precios o resultados.</li>
                <li>Intentar copiar, reconstruir o revender la información consolidada de nuestra base de datos.</li>
                <li>Hacer ingeniería inversa de la plataforma o de nuestra API para eludir límites de uso o controles de seguridad.</li>
                <li>Usar credenciales de otra persona o compartir tu cuenta para evadir los límites de tu perfil.</li>
              </ul>
              <p className="mt-2">
                El incumplimiento de estas reglas puede derivar en la suspensión o cierre de tu cuenta, y en el bloqueo de tu acceso a la plataforma.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                5. Aportes de usuarios colaboradores
              </h2>
              <p>
                Si reportas precios, stock u otra información como usuario Colaborador, nos autorizas a usar, mostrar, almacenar y modificar esa información dentro de la plataforma (por ejemplo, para validar, corregir formato o combinarla con otros reportes), con el fin de mantener el catálogo actualizado para todos los usuarios. Esta autorización es gratuita y no exclusiva: seguirás pudiendo usar esa información como quieras.
              </p>
              <p className="mt-2">
                Nos reservamos el derecho de revisar, descartar o eliminar reportes que parezcan falsos, maliciosos o hechos con el fin de manipular precios o dañar la confiabilidad del catálogo, y de suspender la condición de Colaborador en estos casos.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                6. Precios y disponibilidad
              </h2>
              <p>
                Los precios y el stock que ves en RutaAhorro provienen de la información pública de cada supermercado (y de los reportes de usuarios Colaboradores) y se actualizan periódicamente, pero pueden quedar desactualizados en el tiempo que transcurre entre nuestra última actualización y tu compra. El precio final y la disponibilidad real los determina siempre el supermercado al momento de pagar. RutaAhorro no garantiza que un precio mostrado se mantenga igual en tienda.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                7. El asistente de Inteligencia Artificial
              </h2>
              <p>
                El asistente de IA genera sugerencias de recetas y productos a partir de nuestro catálogo real, pero es una herramienta de apoyo, no una fuente infalible. Siempre revisa el detalle del producto (precio, stock, ingredientes) antes de confirmar tu decisión de compra, especialmente si tienes restricciones alimentarias.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                8. Rutas y desplazamientos
              </h2>
              <p>
                RutaAhorro te sugiere una ruta y un costo estimado de desplazamiento (combustible, tarifa de transporte público, tiempo) en base a información de mapas y transporte público disponible. Esta sugerencia es una estimación informativa: el desplazamiento físico hacia cualquier supermercado es decisión y responsabilidad exclusiva del usuario.
              </p>
              <p className="mt-2">
                RutaAhorro no se hace responsable por accidentes de tránsito, condiciones del tráfico, hechos delictuales, cambios en el transporte público ni ningún otro incidente que ocurra durante tu trayecto. Tampoco garantiza que la ruta sugerida sea la más segura, solo la más conveniente según precio y tiempo estimado. Usa tu propio criterio y respeta siempre las normas de tránsito y seguridad vigentes.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                9. Tu ubicación y datos personales
              </h2>
              <p>
                Para calcular rutas usamos tu ubicación solo mientras estás usando esa función; no la guardamos de forma permanente en tu historial. Tu cuenta, listas de compra y preferencias se almacenan de forma segura y no se comparten con terceros para fines comerciales sin tu autorización. Más detalles en nuestra Política de Privacidad.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                10. Planes de la plataforma
              </h2>
              <p>
                El uso básico de RutaAhorro (comparar precios, calcular ahorro y costo de desplazamiento) es y seguirá siendo gratuito. Algunas funciones adicionales (como historial extendido de precios o uso ampliado de la IA) pueden ofrecerse bajo un plan Premium pagado, que se detallará por separado si decides contratarlo.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                11. Suspensión y cierre de cuentas
              </h2>
              <p>
                Podemos suspender o cerrar tu cuenta si detectamos uso abusivo, intentos de extracción automatizada de datos, reportes maliciosos, incumplimiento de estas condiciones o actividad que ponga en riesgo la plataforma o a otros usuarios. Antes de un bloqueo permanente, salvo casos graves, buscaremos evaluar la situación en lugar de actuar solo con una señal aislada.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                12. Legislación aplicable y jurisdicción
              </h2>
              <p>
                Estos Términos y Condiciones se rigen por las leyes de la República de Chile. Cualquier controversia relacionada con el uso de RutaAhorro se someterá a los tribunales competentes de Chile, con exclusión de cualquier otra jurisdicción.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                13. Cambios a estos términos
              </h2>
              <p>
                Podemos actualizar estos Términos y Condiciones en el futuro. Si hay cambios relevantes, te lo notificaremos y, de ser necesario, te pediremos aceptar la nueva versión para seguir usando la plataforma.
              </p>
            </section>

            <section>
              <h2 className="text-xl font-bold text-foreground flex items-center gap-2 mt-8 mb-3">
                <CheckCircle2 className="h-5 w-5 text-primary shrink-0" />
                14. Contacto
              </h2>
              <p>
                Si tienes dudas sobre estos términos, puedes escribirnos a <a href="mailto:viaahorroapp@gmail.com" className="text-primary hover:underline font-medium">viaahorroapp@gmail.com</a>
              </p>
            </section>
          </div>
        </div>

        <div className="flex justify-center pb-8 pt-4">
          <Link
            to="/login"
            state={{ tab: 'signup' }}
            className={buttonVariants({ size: 'lg', className: 'w-full sm:w-auto font-bold shadow-[4px_4px_0px_currentColor] text-base h-12 px-8' })}
          >
            He leído y entendido, volver al registro
          </Link>
        </div>
      </div>
    </div>
  )
}
