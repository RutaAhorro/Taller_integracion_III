// Dibuja el origen y las paradas recibidas; la línea entre puntos no calcula navegación vial.
'use client'

import { useEffect, useRef } from 'react'
import 'leaflet/dist/leaflet.css'

export type MapStop = {
  id: string
  name: string
  color: string
  coords: [number, number]
  order: number
}

type RouteMapProps = {
  home: { coords: [number, number]; label: string }
  stops: MapStop[]
}

export function RouteMap({ home, stops }: RouteMapProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const mapRef = useRef<any>(null)

  useEffect(() => {
    let cancelled = false

    // Carga Leaflet al montar y evita crear el mapa si la vista ya se desmontó.
    async function init() {
      const L = (await import('leaflet')).default
      if (cancelled || !containerRef.current || mapRef.current) return

      const map = L.map(containerRef.current, {
        zoomControl: true,
        scrollWheelZoom: false,
        attributionControl: true,
      })
      mapRef.current = map

      L.tileLayer(
        'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
        {
          maxZoom: 19,
          attribution:
            '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>',
        },
      ).addTo(map)

      const homeIcon = L.divIcon({
        className: '',
        html: `<div style="display:flex;align-items:center;justify-content:center;width:34px;height:34px;border-radius:50%;background:oklch(0.19 0.01 150);color:#fff;font:600 12px/1 system-ui;box-shadow:0 2px 8px rgba(0,0,0,.25);border:2px solid #fff;">🏠</div>`,
        iconSize: [34, 34],
        iconAnchor: [17, 17],
      })

      L.marker(home.coords, { icon: homeIcon })
        .addTo(map)
        .bindPopup(`<b>${home.label}</b><br/>Inicio de la ruta`)

      stops.forEach((stop) => {
        const icon = L.divIcon({
          className: '',
          html: `<div style="display:flex;align-items:center;justify-content:center;width:30px;height:30px;border-radius:50% 50% 50% 0;transform:rotate(-45deg);background:${stop.color};color:#fff;font:700 13px/1 system-ui;box-shadow:0 2px 8px rgba(0,0,0,.25);border:2px solid #fff;">
            <span style="transform:rotate(45deg)">${stop.order}</span>
          </div>`,
          iconSize: [30, 30],
          iconAnchor: [15, 28],
        })
        L.marker(stop.coords, { icon })
          .addTo(map)
          .bindPopup(`<b>${stop.order}. ${stop.name}</b><br/>Parada de compra`)
      })

      const routeCoords: [number, number][] = [
        home.coords,
        ...stops.map((s) => s.coords),
      ]

      L.polyline(routeCoords, {
        color: 'oklch(0.56 0.13 152)',
        weight: 4,
        opacity: 0.9,
        dashArray: '2 10',
        lineCap: 'round',
      }).addTo(map)

      map.fitBounds(L.latLngBounds(routeCoords).pad(0.25))
    }

    init()

    return () => {
      cancelled = true
      if (mapRef.current) {
        mapRef.current.remove()
        mapRef.current = null
      }
    }
  }, [home, stops])

  return (
    <div
      ref={containerRef}
      className="h-full w-full"
      role="application"
      aria-label="Mapa interactivo con la ruta óptima de compra"
    />
  )
}
