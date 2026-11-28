// Muestra solo geometrías confirmadas por Motor; Leaflet se carga en el navegador.
import { useEffect, useRef, useState } from "react";
import type { RespuestaOptimizacion } from "./rutas-api";
import { decodificarPolilinea } from "./geometria-ruta";
import "leaflet/dist/leaflet.css";

/** Renderiza calles, origen y paradas; limpia la instancia al cambiar el resultado. */
export function MapaRuta({
  datos,
  precisionMetros = null,
  origenManual = false,
}: {
  datos: RespuestaOptimizacion;
  precisionMetros?: number | null;
  origenManual?: boolean;
}) {
  const contenedor = useRef<HTMLDivElement>(null);
  const accionesMapa = useRef<{
    centrarOrigen: () => void;
    verRecorrido: () => void;
  } | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelado = false;
    let limpiar: (() => void) | undefined;
    setError("");
    if (datos.fuente_recorrido !== "ors_osm") return;
    void import("leaflet")
      .then((leaflet) => {
        if (cancelado || !contenedor.current) return;
        // El círculo de precisión y el tooltip permanente necesitan una vista inicial.
        const mapa = leaflet
          .map(contenedor.current, {
            scrollWheelZoom: false,
            zoomAnimation: false,
            fadeAnimation: false,
            markerZoomAnimation: false,
          })
          .setView([datos.origen.lat, datos.origen.lon], 14);
        // El carrito flotante ocupa la esquina inferior; no debe tapar la atribución.
        mapa.attributionControl.setPosition("topright");
        limpiar = () => {
          mapa.stop();
          mapa.remove();
        };
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
          const marcadorOrigen = leaflet
            .marker([datos.origen.lat, datos.origen.lon], {
              title: "Origen",
              icon: iconoMarcador("O", "#1961b1"),
            })
            .addTo(mapa)
            .bindTooltip(
              datos.origen.por_defecto
                ? "Centro de Temuco (origen por defecto)"
                : origenManual
                  ? "Origen elegido en el mapa"
                  : "Origen indicado por el navegador",
              { permanent: true, direction: "top", offset: [0, -15] },
            );
          // La precisión es una estimación del navegador, no una garantía de GPS.
          if (
            !datos.origen.por_defecto &&
            !origenManual &&
            precisionMetros !== null &&
            Number.isFinite(precisionMetros) &&
            precisionMetros > 0
          ) {
            leaflet
              .circle([datos.origen.lat, datos.origen.lon], {
                radius: precisionMetros,
                color: "#1961b1",
                fillOpacity: 0.08,
                weight: 1,
              })
              .addTo(mapa);
          }
          const limites = trazado
            .getBounds()
            .extend(marcadorOrigen.getLatLng());
          for (const parada of datos.ruta.paradas) {
            limites.extend([parada.lat, parada.lon]);
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
          /** Incluye el origen aunque ORS haya ajustado el recorrido a la red vial. */
          const verRecorrido = () =>
            mapa.fitBounds(limites, {
              padding: [45, 45],
              maxZoom: 17,
              animate: false,
            });
          accionesMapa.current = {
            centrarOrigen: () => {
              mapa.setView(marcadorOrigen.getLatLng(), 17, { animate: false });
              marcadorOrigen.openTooltip();
            },
            verRecorrido,
          };
          verRecorrido();
          mapa.invalidateSize();
        } catch {
          accionesMapa.current = null;
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
      accionesMapa.current = null;
      limpiar?.();
    };
  }, [datos, precisionMetros, origenManual]);
  if (datos.fuente_recorrido !== "ors_osm") return null;
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
      <p className="text-xs text-muted-foreground">
        Recorrido:{" "}
        <a
          href="https://openrouteservice.org/"
          target="_blank"
          rel="noopener noreferrer"
          className="underline"
        >
          openrouteservice / HeiGIT
        </a>
        .
      </p>
      <p className="text-sm text-muted-foreground">
        Origen O, paradas numeradas y retorno. Distancias y tiempos estimados
        sobre OpenStreetMap, sin tráfico en vivo.
      </p>
      <p className="text-xs text-muted-foreground">
        {datos.origen.por_defecto
          ? "O representa el centro de Temuco, no tu ubicación actual."
          : origenManual
            ? "O es el punto que confirmaste en el mapa. El trazado puede comenzar en una calle cercana por las restricciones de acceso del modo elegido."
            : "O es el punto indicado por tu navegador. El trazado puede comenzar en una calle cercana por las restricciones de acceso del modo elegido."}
        {!datos.origen.por_defecto && precisionMetros !== null && (
          <>
            {" "}
            El círculo indica una precisión aproximada de{" "}
            {Math.round(precisionMetros)} m; no garantiza tu posición exacta.
          </>
        )}
      </p>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className="min-h-10 rounded-lg border border-border px-3 py-2 text-sm font-semibold"
          onClick={() => accionesMapa.current?.centrarOrigen()}
        >
          Centrar en origen
        </button>
        <button
          type="button"
          className="min-h-10 rounded-lg border border-border px-3 py-2 text-sm font-semibold"
          onClick={() => accionesMapa.current?.verRecorrido()}
        >
          Ver recorrido completo
        </button>
      </div>
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
