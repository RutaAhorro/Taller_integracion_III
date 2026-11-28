// Muestra solo geometrías confirmadas por Motor; Leaflet se carga en el navegador.
import { useEffect, useRef, useState } from "react";
import type { RespuestaOptimizacion } from "./rutas-api";
import { decodificarPolilinea } from "./geometria-ruta";
import "leaflet/dist/leaflet.css";

/** Renderiza calles, origen y paradas; limpia la instancia al cambiar el resultado. */
export function MapaRuta({ datos }: { datos: RespuestaOptimizacion }) {
  const contenedor = useRef<HTMLDivElement>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelado = false;
    let limpiar: (() => void) | undefined;
    setError("");
    if (datos.fuente_recorrido !== "otp_osm") return;
    void import("leaflet")
      .then((leaflet) => {
        if (cancelado || !contenedor.current) return;
        const mapa = leaflet.map(contenedor.current, {
          scrollWheelZoom: false,
        });
        // El carrito flotante ocupa la esquina inferior; no debe tapar la atribución.
        mapa.attributionControl.setPosition("topright");
        limpiar = () => mapa.remove();
        /** Usa nodos de texto para no interpretar nombres o etiquetas como HTML. */
        function iconoMarcador(texto: string, color: string) {
          const etiqueta = document.createElement("span");
          etiqueta.textContent = texto;
          Object.assign(etiqueta.style, {
            display: "grid",
            placeItems: "center",
            background: color,
            color: "white",
            border: "2px solid white",
            borderRadius: "50%",
            width: "30px",
            height: "30px",
          });
          return leaflet.divIcon({
            className: "",
            html: etiqueta,
            iconSize: [30, 30],
            iconAnchor: [15, 15],
          });
        }
        try {
          const lineas = datos.ruta.tramos.flatMap((tramo) => {
            if (!tramo.geometrias?.length) throw new Error("Falta geometría.");
            return tramo.geometrias.map(decodificarPolilinea);
          });
          if (!lineas.length) throw new Error("Sin recorrido.");
          const fondo = leaflet
            .tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
              attribution:
                '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
              maxZoom: 19,
            })
            .addTo(mapa);
          fondo.on("tileerror", () => {
            if (!cancelado)
              setError(
                "No se pudo cargar parte del fondo del mapa. El detalle de las paradas sigue disponible.",
              );
          });
          const trazado = leaflet
            .featureGroup(
              lineas.map((puntos) =>
                leaflet.polyline(puntos, { color: "#1961b1", weight: 5 }),
              ),
            )
            .addTo(mapa);
          leaflet
            .marker([datos.origen.lat, datos.origen.lon], {
              title: "Origen",
              icon: iconoMarcador("O", "#1961b1"),
            })
            .addTo(mapa)
            .bindTooltip(
              datos.origen.por_defecto
                ? "Centro de Temuco (origen por defecto)"
                : "Origen indicado",
            );
          for (const parada of datos.ruta.paradas) {
            const etiqueta = document.createElement("span");
            etiqueta.textContent = `${parada.orden}. ${parada.nombre}`;
            leaflet
              .marker([parada.lat, parada.lon], {
                title: `Parada ${parada.orden}: ${parada.nombre}`,
                icon: iconoMarcador(String(parada.orden), "#212529"),
              })
              .addTo(mapa)
              .bindTooltip(etiqueta);
          }
          mapa.fitBounds(trazado.getBounds(), { padding: [30, 30] });
          mapa.invalidateSize();
        } catch {
          limpiar();
          limpiar = undefined;
          setError(
            "La geometría recibida no permite dibujar el recorrido. No se sustituyó por líneas rectas.",
          );
        }
      })
      .catch(() => {
        if (!cancelado) setError("No se pudo cargar el mapa.");
      });
    return () => {
      cancelado = true;
      limpiar?.();
    };
  }, [datos]);
  if (datos.fuente_recorrido !== "otp_osm") return null;
  return (
    <section
      id="mapa-ruta"
      tabIndex={-1}
      aria-label="Mapa del recorrido"
      className="scroll-mt-4 space-y-3 rounded-2xl border border-border bg-card p-4"
    >
      <h2 className="font-display text-xl font-bold">
        Tu recorrido por calles
      </h2>
      <p className="text-sm text-muted-foreground">
        Origen O, paradas numeradas y retorno. Distancias y tiempos estimados
        sobre OpenStreetMap, sin tráfico en vivo.
      </p>
      {error && (
        <p role="alert" className="text-sm">
          {error}
        </p>
      )}
      <div
        ref={contenedor}
        className="relative z-0 h-[380px] w-full rounded-xl"
      />
    </section>
  );
}
