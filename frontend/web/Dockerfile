# =========================================================
# Etapa 1: Desarrollo
# =========================================================
FROM node:22-alpine AS development
WORKDIR /app
COPY package*.json ./
RUN --mount=type=secret,id=network_ca,target=/run/secrets/network_ca \
    if [ -f /run/secrets/network_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/network_ca; fi; \
    npm install -g npm@latest
RUN --mount=type=secret,id=network_ca,target=/run/secrets/network_ca \
    if [ -f /run/secrets/network_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/network_ca; fi; \
    npm ci
COPY . .
# Expone Vite con recarga automática durante el desarrollo.
EXPOSE 3000
CMD ["npm", "run", "dev", "--", "--host"]

# =========================================================
# Etapa 2: Construcción
# =========================================================
FROM development AS build
# Genera marcadores de configuración pública que se sustituyen al iniciar el contenedor.
RUN echo "VITE_API_URL=__VITE_API_URL__" > .env.production && \
    echo "VITE_GOOGLE_CLIENT_ID=__VITE_GOOGLE_CLIENT_ID__" >> .env.production && \
    echo "VITE_IA_URL=__VITE_IA_URL__" >> .env.production
# Transpilar el código de React/Vite a estáticos minificados
RUN npm run build

# =========================================================
# Etapa 3: Producción (Nginx)
# =========================================================
FROM nginx:alpine AS production
# Instalar sed (aunque Alpine ya lo trae en busybox, nos aseguramos)
RUN --mount=type=secret,id=network_ca,target=/run/secrets/network_ca \
    if [ -f /run/secrets/network_ca ]; then export SSL_CERT_FILE=/run/secrets/network_ca; fi; \
    apk add --no-cache bash
# Copia los archivos compilados al directorio público de Nginx.
COPY --from=build /app/dist /usr/share/nginx/html
# Copiar configuración personalizada de Nginx (para soporte SPA/React Router)
COPY nginx.conf /etc/nginx/conf.d/default.conf
# Copiar script de inyección de variables en tiempo real
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 3000
# Arrancar el script que inyecta las variables y luego ejecuta nginx
CMD ["/entrypoint.sh"]
