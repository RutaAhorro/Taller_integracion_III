#!/bin/sh

echo "Inyectando variables de entorno en tiempo de ejecución..."

# Rutas de los archivos estáticos minificados
FILES="/usr/share/nginx/html/assets/*.js"

for file in $FILES; do
  if [ -f "$file" ]; then
    # Reemplazar placeholders con valores reales. Si el valor real está vacío, se deja vacío.
    sed -i "s|__VITE_API_URL__|${VITE_API_URL}|g" "$file"
    sed -i "s|__VITE_GOOGLE_CLIENT_ID__|${VITE_GOOGLE_CLIENT_ID}|g" "$file"
    sed -i "s|__VITE_IA_URL__|${VITE_IA_URL}|g" "$file"
  fi
done

echo "Inyección completada. Iniciando Nginx..."
exec nginx -g 'daemon off;'
