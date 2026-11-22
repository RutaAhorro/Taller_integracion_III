package scraping

// ===== Catálogo de cadenas =====

// Cadena identifica una fuente de supermercado y el rastreador que Control debe
// encolar para ella. Los identificadores y nombres de rastreador forman parte
// del contrato con Control y con el Scraper, por eso se declaran aquí en un
// solo lugar y no repartidos entre los botones y las llamadas HTTP.
type Cadena struct {
	ID               int
	NombreRastreador string
	Nombre           string
}

// Cadenas son las fuentes que el bot ofrece, en el orden en que se muestran y
// se lanzan en el scraping masivo.
var Cadenas = []Cadena{
	{ID: 1, NombreRastreador: "jumbo_rsc", Nombre: "Jumbo"},
	{ID: 2, NombreRastreador: "lider_rsc", Nombre: "Lider"},
	{ID: 4, NombreRastreador: "santa_isabel_rsc", Nombre: "Santa Isabel"},
	{ID: 5, NombreRastreador: "acuenta_rsc", Nombre: "A Cuenta"},
	{ID: 6, NombreRastreador: "cugat_rsc", Nombre: "Cugat"},
}

// NombreCadena devuelve el nombre legible de una cadena o, si el identificador
// no está en el catálogo, el texto alternativo recibido (por ejemplo, el
// nombre del rastreador extraído del botón).
func NombreCadena(cadenaID int, nombreAlternativo string) string {
	for _, cadena := range Cadenas {
		if cadena.ID == cadenaID {
			return cadena.Nombre
		}
	}
	return nombreAlternativo
}
