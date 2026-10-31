# 🧠 Reglas de Negocio (Core Logic)

Estas son las reglas inmutables del sistema. Deben respetarse estrictamente en la lógica del Backend (Go/Python) y reflejarse en el Frontend.

---

## 1. El Motor de Optimización (La Regla de Oro)
La "Mejor Alternativa" SIEMPRE será la que minimice la fórmula:
`Costo Total = (Precio Productos) + (Costo Desplazamiento)`
* **Restricciones inyectadas:** El cálculo fallará o descartará la ruta si no hay stock, si supera el presupuesto del usuario, o si viola sus restricciones dietéticas.

## 2. Vigencia y Validez de Datos
* **Precios Expirados:** Los precios tienen fecha de caducidad. Si el `updated_at` supera el tiempo de frescura (ej. 48 horas), el Backend debe ocultar el precio automáticamente.
* **Falta de Stock:** Si el stock es 0, el producto se excluye de la ruta y el Frontend muestra una alerta roja o deshabilitada.

## 3. Patrón Anti-Alucinación (IA)
La IA no debe inventar stock ni tiendas.
* **Flujo estricto:** 
  1. IA extrae filtros del texto. 
  2. Backend Python consulta la Base de Datos con esos filtros.
  3. Backend **INYECTA** los resultados reales en un nuevo prompt.
  4. IA responde basándose SOLO en lo que el Backend le entregó.
* *Cortocircuito:* Si el paso 2 no devuelve resultados, Python cancela el paso 3 y responde un error estándar para ahorrar tokens de API.

## 4. Viabilidad Logística y de Rendimiento
* **Límite de locales:** Nadie va a 10 supermercados. El backend debe limitar el cálculo a un máximo configurable de paradas (ej. max 3 locales).
* **Umbral de Rentabilidad:** Si el ahorro de ir a 2 supermercados es de solo $100 pesos, no vale la pena. El backend debe abortar la ruta multiparada si el ahorro neto no supera un `MIN_SAVINGS_THRESHOLD`.
* **Monto Mínimo:** Para evitar calcular rutas por una barra de cereal, el carrito debe superar un subtotal mínimo antes de invocar a OpenTripPlanner (OTP).

## 5. Perfil de Transporte
* **Obligatorio:** No se puede calcular el costo de desplazamiento sin saber cómo viaja el usuario (Auto, Bus, Caminando). El endpoint de cálculo exigirá que el `perfil_transporte` esté configurado.
