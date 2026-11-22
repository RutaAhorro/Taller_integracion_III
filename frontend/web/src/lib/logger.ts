// Centraliza registros: en producción conserva errores y silencia los mensajes de desarrollo.
const esDesarrollo = import.meta.env.DEV;

export const logger = {
  info: (mensaje: string, datos?: any) => {
    if (esDesarrollo) {
      if (datos) console.info(`📘 [INFO]: ${mensaje}`, datos);
      else console.info(`📘 [INFO]: ${mensaje}`);
    }
  },
  warn: (mensaje: string, datos?: any) => {
    if (esDesarrollo) {
      if (datos) console.warn(`📙 [WARN]: ${mensaje}`, datos);
      else console.warn(`📙 [WARN]: ${mensaje}`);
    }
  },
  error: (mensaje: string, errorObtenido?: any) => {
    // Conserva el error en producción para permitir el diagnóstico.
    if (errorObtenido) console.error(`📕 [ERROR]: ${mensaje}`, errorObtenido);
    else console.error(`📕 [ERROR]: ${mensaje}`);
  },
  debug: (mensaje: string, datos?: any) => {
    if (esDesarrollo) {
      if (datos) console.debug(`🐛 [DEBUG]: ${mensaje}`, datos);
      else console.debug(`🐛 [DEBUG]: ${mensaje}`);
    }
  }
};
