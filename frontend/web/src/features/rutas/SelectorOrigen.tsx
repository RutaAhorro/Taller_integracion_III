// Selección manual local: no pide GPS ni consulta Motor hasta confirmar el punto.
import { useEffect, useRef, useState } from "react";
import type { Marker as MarcadorLeaflet } from "leaflet";
import { Button } from "@/components/ui/button";
import "leaflet/dist/leaflet.css";

/** Coordenadas del punto elegido por el usuario, sin identidad ni persistencia. */
export type PuntoOrigen = { lat: number; lon: number };
// Solo encuadra el selector si aún no hay origen; nunca confirma este punto por sí solo.
const REFERENCIA_TEMUCO: PuntoOrigen = { lat: -38.7359, lon: -72.5904 };

/** Permite marcar con clic/arrastre o usar el centro del mapa mediante teclado. */
export function SelectorOrigen({
  inicial,
  confirmar,
  cancelar,
}: {
  inicial: PuntoOrigen | null;
  confirmar: (punto: PuntoOrigen) => void;
  cancelar: () => void;
}) {
  const contenedor = useRef<HTMLDivElement>(null);
  const seleccionarCentro = useRef<(() => void) | null>(null);
  const referenciaInicial = useRef(inicial ?? REFERENCIA_TEMUCO);
  const [candidato, setCandidato] = useState<PuntoOrigen | null>(null);
  const [listo, setListo] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let abandonado = false;
    let limpiar: (() => void) | undefined;
    void import("leaflet")
      .then((leaflet) => {
        if (abandonado || !contenedor.current) return;
        const referencia = referenciaInicial.current;
        const mapa = leaflet
          .map(contenedor.current, {
            scrollWheelZoom: false,
            zoomAnimation: false,
            fadeAnimation: false,
            markerZoomAnimation: false,
          })
          .setView([referencia.lat, referencia.lon], 14);
        mapa.attributionControl.setPosition("topright");
        limpiar = () => {
          mapa.stop();
          mapa.remove();
        };
        let marcador: MarcadorLeaflet | undefined;
        /** Mantiene el borrador en este componente; moverlo no recalcula la compra. */
        function elegir(latitud: number, longitud: number) {
          const punto = leaflet.latLng(latitud, longitud).wrap();
          setCandidato({ lat: punto.lat, lon: punto.lng });
          if (marcador) {
            marcador.setLatLng(punto);
            return;
          }
          const etiqueta = document.createElement("span");
          etiqueta.textContent = "O";
          etiqueta.style.cssText =
            "display:grid;place-items:center;width:30px;height:30px;background:#1961b1;color:white;border:2px solid white;border-radius:50%";
          marcador = leaflet
            .marker(punto, {
              draggable: true,
              title: "Punto de partida propuesto",
              icon: leaflet.divIcon({
                className: "",
                html: etiqueta,
                iconSize: [30, 30],
                iconAnchor: [15, 15],
              }),
            })
            .addTo(mapa);
          marcador.on("dragend", () => {
            if (abandonado || !marcador) return;
            const punto = marcador.getLatLng().wrap();
            setCandidato({ lat: punto.lat, lon: punto.lng });
          });
        }
        seleccionarCentro.current = () => {
          const punto = mapa.getCenter();
          elegir(punto.lat, punto.lng);
        };
        mapa.on("click", (evento) =>
          elegir(evento.latlng.lat, evento.latlng.lng),
        );
        leaflet
          .tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
            attribution:
              '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
            maxZoom: 19,
          })
          .on("tileerror", () => {
            if (!abandonado)
              setError(
                "No se pudo cargar parte del fondo. No confirmes el punto si no puedes reconocer el lugar.",
              );
          })
          .addTo(mapa);
        setListo(true);
        mapa.invalidateSize();
      })
      .catch(() => {
        if (!abandonado) setError("No se pudo abrir el selector de ubicación.");
      });
    return () => {
      abandonado = true;
      seleccionarCentro.current = null;
      limpiar?.();
    };
  }, []);

  return (
    <section
      aria-label="Elegir punto de partida en el mapa"
      className="space-y-3 rounded-xl border border-border p-3"
    >
      <h3 className="font-semibold">Marca tu punto de partida</h3>
      <p className="text-sm text-muted-foreground">
        Haz clic en el mapa y ajusta el marcador arrastrándolo. También puedes
        mover el mapa con las flechas del teclado y pulsar Usar centro del mapa.
        La ruta no cambia hasta confirmar.
      </p>
      <div
        ref={contenedor}
        aria-label="Mapa para elegir origen"
        className="relative z-0 h-[340px] w-full rounded-xl"
      />
      {error && (
        <p role="alert" className="text-sm">
          {error}
        </p>
      )}
      <p role="status" className="text-sm">
        {candidato
          ? "Punto marcado. Comprueba el lugar antes de confirmar."
          : "Todavía no has marcado un punto."}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          disabled={!listo}
          onClick={() => seleccionarCentro.current?.()}
        >
          Usar centro del mapa
        </Button>
        <Button
          disabled={!candidato}
          onClick={() => {
            if (candidato) confirmar(candidato);
          }}
        >
          Confirmar punto de partida
        </Button>
        <Button variant="outline" onClick={cancelar}>
          Cancelar selección
        </Button>
      </div>
    </section>
  );
}
