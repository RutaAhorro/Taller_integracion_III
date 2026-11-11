// Contratos de sesión, datos del dominio y modelos usados por los componentes de interfaz.
export type Usuario = {
  id: string;
  google_id?: string | null;
  correo: string;
  nombre_completo: string;
  url_avatar?: string | null;
  rol: string;
  cuota_tokens_ia: number;
  esta_activo: boolean;
  token_verificacion?: string | null;
  creado_el: string;
  actualizado_el: string;
  plan?: 'Usuario' | 'Plus';
  // JWT devuelto por el backend al autenticar
  token?: string;
};

export interface LoginPayload {
  correo: string;
  password: string;
}

export interface RegisterPayload {
  correo: string;
  password: string;
  nombre_completo: string;
}

export type PreferenciasDieteticas = {
  usuarioId: string;
  etiquetas: string[]; // Por ejemplo: 'vegan', 'celiac', 'keto', 'none'.
};

export type CadenaSupermercado = {
  id: number;
  nombre: string;
  urlSitioWeb: string | null;
  urlLogo: string;
  configScraperJson: any;
  estaActiva: boolean;
};

export type SucursalSupermercado = {
  id: number;
  cadenaId: number;
  codigoSucursal: string;
  nombre: string;
  direccion: string;
  comuna: string;
  ciudad: string;
  lat: number;
  lon: number;
  horaApertura: string | null;
  horaCierre: string | null;
  estaActiva: boolean;
};

export type Categoria = {
  id: number;
  padreId: number | null;
  nombre: string;
  slug: string;
};

export type Marca = {
  id: number;
  nombre: string;
};

export type ProductoNormalizado = {
  id: string;
  categoriaId: number | null;
  marcaId: number | null;
  codigoBarrasEan: string | null;
  nombreEstandar: string;
  contenidoNeto: number | null;
  unidadMedida: string | null;
  esSinGluten: boolean;
  esVegano: boolean;
  esSinLactosa: boolean;
};

export type ProductoCrudo = {
  id: string;
  sucursalId: number;
  sku: string;
  tituloCrudo: string;
  marcaCruda: string | null;
  categoriaCruda: string | null;
  urlProducto: string | null;
  urlImagen: string | null;
  enStock: boolean;
  ultimaExtraccionEl: string;
};

export type CapturaPrecio = {
  id: number;
  productoCrudoId: string;
  precioNormal: number | null;
  precioOferta: number | null;
  precioTarjeta: number | null;
  precioPorUnidad: number | null;
  metricaUnidad: string | null;
  estaDisponible: boolean;
  capturadoEl: string;
};

// Modelos de interfaz adaptados a partir de las respuestas del servidor.

export type UiSupermarket = {
  id: string;
  name: string;
};

export type UiProduct = {
  id: string;
  name: string;
  brand: string;
  image: string;
  category?: string;
  unit: string;
  price: number;
  originalPrice: number;
  supermarketId: string; // Referencia al identificador de UiSupermarket.
  supermarketName?: string;
  inStock?: boolean;
};
