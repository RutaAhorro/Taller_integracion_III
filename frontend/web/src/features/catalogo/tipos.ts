// Modelos de interfaz del catálogo, adaptados a partir de las respuestas del servidor.

// Cadena de supermercado usada por el filtro lateral.
export type UiSupermarket = {
  id: string;
  name: string;
};

// Producto con el precio de una sucursal concreta.
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
