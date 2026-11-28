// Adapta las polilíneas 2D de cinco decimales a latitud/longitud, sin inventar tramos.

/** Rechaza polilíneas truncadas, excesivas o fuera de rango antes de dibujarlas. */
export function decodificarPolilinea(texto: string): [number, number][] {
  if (texto.length > 200000) throw new Error("Geometría demasiado extensa.");
  const coordenadas: [number, number][] = [];
  let indice = 0;
  let latitud = 0;
  let longitud = 0;
  /** Lee un entero delta con una cota de longitud defensiva. */
  function leerDelta(): number {
    let resultado = 0;
    let desplazamiento = 0;
    let fragmento: number;
    do {
      if (indice >= texto.length || desplazamiento > 30)
        throw new Error("Geometría incompleta.");
      fragmento = texto.charCodeAt(indice++) - 63;
      if (fragmento < 0 || fragmento > 63)
        throw new Error("Geometría inválida.");
      resultado |= (fragmento & 31) << desplazamiento;
      desplazamiento += 5;
    } while (fragmento >= 32);
    return resultado & 1 ? ~(resultado >>> 1) : resultado >>> 1;
  }
  while (indice < texto.length) {
    latitud += leerDelta();
    longitud += leerDelta();
    const punto: [number, number] = [latitud / 1e5, longitud / 1e5];
    if (Math.abs(punto[0]) > 90 || Math.abs(punto[1]) > 180)
      throw new Error("Coordenadas fuera de rango.");
    coordenadas.push(punto);
  }
  if (coordenadas.length < 2) throw new Error("Geometría sin recorrido.");
  return coordenadas;
}
