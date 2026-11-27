// Agrupa únicamente la compra que Motor ya asignó; no decide tiendas ni vuelve a calcular precios.
import type { ArticuloRuta } from "./rutas-api";

/** Conserva las referencias originales al presentar varias líneas como una sola compra. */
export type ArticuloAgrupadoRuta = ArticuloRuta & {
  clave: string;
  producto_ids: string[];
};

/**
 * Une el mismo producto ofertado y precio dentro de una parada, no nombres parecidos.
 * Dos artículos de distintas cadenas pueden acabar en la misma oferta por EAN.
 * Cantidades y subtotales se suman tal como llegaron; nunca se muta la respuesta
 * ni se vuelve a multiplicar el precio para sustituir importes del Motor.
 */
export function agruparArticulosParada(
  articulos: readonly ArticuloRuta[],
): ArticuloAgrupadoRuta[] {
  const agrupados = new Map<string, ArticuloAgrupadoRuta>();
  for (const articulo of articulos) {
    const ofertaID = articulo.producto_id_oferta || articulo.producto_id;
    const clave = JSON.stringify([ofertaID, articulo.precio_unitario]);
    const existente = agrupados.get(clave);
    if (existente) {
      existente.cantidad += articulo.cantidad;
      existente.subtotal += articulo.subtotal;
      if (!existente.producto_ids.includes(articulo.producto_id))
        existente.producto_ids.push(articulo.producto_id);
      if (!existente.nombre && articulo.nombre)
        existente.nombre = articulo.nombre;
    } else {
      agrupados.set(clave, {
        ...articulo,
        clave,
        producto_ids: [articulo.producto_id],
      });
    }
  }
  return [...agrupados.values()];
}
