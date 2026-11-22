// Tipos de sesión compartidos por el contexto de autenticación y sus pantallas.

// ===== Plan y rol =====

// Plan comercial devuelto por Identidad (`api.usuarios.plan`, CHECK 'basico' | 'premium').
// Es un dato distinto del rol (`admin` | `colaborador` | `registrado`, ver `roles.ts`).
export const PLAN_BASICO = "basico";
export const PLAN_PREMIUM = "premium";
export type PlanUsuario = typeof PLAN_BASICO | typeof PLAN_PREMIUM;

// ===== Usuario =====

// Usuario tal como lo entrega Identidad al iniciar sesión.
export type Usuario = {
  id: string;
  google_id?: string | null;
  correo: string;
  nombre_completo: string;
  url_avatar?: string | null;
  rol: string;
  cuota_tokens_ia: number;
  esta_activo: boolean;
  token_verificacion?: string | null;
  creado_el: string;
  actualizado_el: string;
  plan?: PlanUsuario;
  // JWT devuelto por el backend al autenticar
  token?: string;
};

// ===== Cuerpos de autenticación =====

export interface LoginPayload {
  correo: string;
  password: string;
}

export interface RegisterPayload {
  correo: string;
  password: string;
  nombre_completo: string;
}
