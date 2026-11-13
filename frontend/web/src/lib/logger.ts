// Centraliza registros: en producción conserva errores y silencia los mensajes de desarrollo.
const isDev = import.meta.env.DEV;

export const logger = {
  info: (msg: string, data?: any) => {
    if (isDev) {
      if (data) console.info(`📘 [INFO]: ${msg}`, data);
      else console.info(`📘 [INFO]: ${msg}`);
    }
  },
  warn: (msg: string, data?: any) => {
    if (isDev) {
      if (data) console.warn(`📙 [WARN]: ${msg}`, data);
      else console.warn(`📙 [WARN]: ${msg}`);
    }
  },
  error: (msg: string, error?: any) => {
    // Conserva el error en producción para permitir el diagnóstico.
    if (error) console.error(`📕 [ERROR]: ${msg}`, error);
    else console.error(`📕 [ERROR]: ${msg}`);
  },
  debug: (msg: string, data?: any) => {
    if (isDev) {
      if (data) console.debug(`🐛 [DEBUG]: ${msg}`, data);
      else console.debug(`🐛 [DEBUG]: ${msg}`);
    }
  }
};
