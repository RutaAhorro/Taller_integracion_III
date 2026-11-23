// Integración con PostgreSQL real tras 06_propiedad_permisos.sql y 07_runtime_credentials.sh.
//
// Se omite salvo que existan RUTAS_TEST_DB_URL (app_rutas) y RUTAS_TEST_ADMIN_URL
// (operador) apuntando a una base desechable inicializada con infrastructure/db.
// Nunca apuntarlas a una base compartida.
package integration_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/db"
	"github.com/danirc2024/Taller_integracion_III/backend/motor_rutas/internal/listas"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func preparar(prueba *testing.T) (*pgxpool.Pool, *listas.Postgres, string) {
	prueba.Helper()
	urlRutas, admin := os.Getenv("RUTAS_TEST_DB_URL"), os.Getenv("RUTAS_TEST_ADMIN_URL")
	if urlRutas == "" || admin == "" {
		prueba.Skip("requiere RUTAS_TEST_DB_URL y RUTAS_TEST_ADMIN_URL")
	}
	pool, errorObtenido := db.Abrir(context.Background(), urlRutas, 2)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(pool.Close)
	return pool, listas.NuevoPostgres(pool), admin
}

func TestPermisosYCRUDAisladoPorUsuario(prueba *testing.T) {
	pool, repo, _ := preparar(prueba)
	contexto := context.Background()
	usuarioA, usuarioB := uuid.NewString(), uuid.NewString()
	if !db.Ping(contexto, pool, 2*time.Second) {
		prueba.Fatal("ping falló")
	}

	lista, errorObtenido := repo.Crear(contexto, usuarioA, "Semana ñandú")
	if errorObtenido != nil || lista.Estado != "activa" || lista.TotalArticulos != 0 || lista.CreadaEl.Location() != time.UTC {
		prueba.Fatalf("crear: %+v %v", lista, errorObtenido)
	}
	if cantidad, _ := repo.Contar(contexto, usuarioA); cantidad != 1 {
		prueba.Fatalf("contar: %d", cantidad)
	}
	if detalle, _ := repo.Obtener(contexto, usuarioB, lista.ID); detalle != nil {
		prueba.Fatal("B lee la lista de A")
	}
	robado := "robada"
	if modificada, _ := repo.Actualizar(contexto, usuarioB, lista.ID, &robado, nil); modificada != nil {
		prueba.Fatal("B modifica la lista de A")
	}
	if eliminada, _ := repo.Eliminar(contexto, usuarioB, lista.ID); eliminada {
		prueba.Fatal("B borra la lista de A")
	}
	if listasUsuario, _ := repo.Listar(contexto, usuarioB, 100); len(listasUsuario) != 0 {
		prueba.Fatal("B ve listas de A")
	}

	completada := "completada"
	actualizada, errorObtenido := repo.Actualizar(contexto, usuarioA, lista.ID, nil, &completada)
	if errorObtenido != nil || actualizada.Nombre != "Semana ñandú" || actualizada.Estado != "completada" || actualizada.ActualizadaEl.Before(lista.ActualizadaEl) {
		prueba.Fatalf("actualizar: %+v %v", actualizada, errorObtenido)
	}
	if listasUsuario, _ := repo.Listar(contexto, usuarioA, 100); len(listasUsuario) != 1 || listasUsuario[0].ID != lista.ID {
		prueba.Fatalf("listar: %+v", listasUsuario)
	}
	if eliminada, errorObtenido := repo.Eliminar(contexto, usuarioA, lista.ID); !eliminada || errorObtenido != nil {
		prueba.Fatalf("eliminar: %v %v", eliminada, errorObtenido)
	}
	if detalle, _ := repo.Obtener(contexto, usuarioA, lista.ID); detalle != nil {
		prueba.Fatal("la lista sigue tras eliminar")
	}
}

// limitesPrueba son topes chicos para probar los límites con pocas filas.
var limitesPrueba = listas.Limites{MaxArticulos: 2, MaxCantidad: 10}

func TestArticulosPersistidosYAisladosPorUsuario(prueba *testing.T) {
	_, repo, _ := preparar(prueba)
	contexto := context.Background()
	usuarioA, usuarioB := uuid.NewString(), uuid.NewString()
	productoLeche, productoPan, productoArroz := uuid.NewString(), uuid.NewString(), uuid.NewString()
	lista, errorObtenido := repo.Crear(contexto, usuarioA, "Con artículos")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repo.Eliminar(context.Background(), usuarioA, lista.ID) })

	// --- Alta y "sumar si ya existe" -----------------------------------------
	articulo, creado, errorObtenido := repo.AgregarArticulo(contexto, usuarioA, lista.ID, productoLeche, 2, limitesPrueba)
	if errorObtenido != nil || !creado || articulo.Cantidad != 2 || *articulo.ProductoID != productoLeche || articulo.EstaComprado {
		prueba.Fatalf("alta: %+v %v %v", articulo, creado, errorObtenido)
	}
	sumado, creado, errorObtenido := repo.AgregarArticulo(contexto, usuarioA, lista.ID, productoLeche, 3, limitesPrueba)
	if errorObtenido != nil || creado || sumado.ID != articulo.ID || sumado.Cantidad != 5 {
		prueba.Fatalf("suma: %+v %v %v", sumado, creado, errorObtenido)
	}

	// --- Topes ---------------------------------------------------------------
	if _, _, errorObtenido := repo.AgregarArticulo(contexto, usuarioA, lista.ID, productoLeche, 6, limitesPrueba); !errors.Is(errorObtenido, listas.ErrMaxCantidad) {
		prueba.Fatalf("suma sobre el tope: %v", errorObtenido)
	}
	if _, _, errorObtenido := repo.AgregarArticulo(contexto, usuarioA, lista.ID, productoPan, 1, limitesPrueba); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, _, errorObtenido := repo.AgregarArticulo(contexto, usuarioA, lista.ID, productoArroz, 1, limitesPrueba); !errors.Is(errorObtenido, listas.ErrMaxArticulos) {
		prueba.Fatalf("tope de artículos: %v", errorObtenido)
	}

	// --- Otro usuario no alcanza la lista ni sus artículos -------------------
	if ajeno, _, errorObtenido := repo.AgregarArticulo(contexto, usuarioB, lista.ID, productoArroz, 1, limitesPrueba); ajeno != nil || errorObtenido != nil {
		prueba.Fatalf("B agrega a la lista de A: %+v %v", ajeno, errorObtenido)
	}
	nueve := 9
	if ajeno, _ := repo.ActualizarArticulo(contexto, usuarioB, lista.ID, articulo.ID, &nueve, nil); ajeno != nil {
		prueba.Fatal("B cambia un artículo de A")
	}
	if borrado, _ := repo.EliminarArticulo(contexto, usuarioB, lista.ID, articulo.ID); borrado {
		prueba.Fatal("B borra un artículo de A")
	}

	// --- Cambio y detalle recuperable ----------------------------------------
	comprado := true
	cambiado, errorObtenido := repo.ActualizarArticulo(contexto, usuarioA, lista.ID, articulo.ID, nil, &comprado)
	if errorObtenido != nil || cambiado.Cantidad != 5 || !cambiado.EstaComprado {
		prueba.Fatalf("cambio: %+v %v", cambiado, errorObtenido)
	}
	detalle, errorObtenido := repo.Obtener(contexto, usuarioA, lista.ID)
	if errorObtenido != nil || detalle.TotalArticulos != 2 || detalle.Articulos[0].ID != articulo.ID ||
		!detalle.Articulos[0].EstaComprado || detalle.ActualizadaEl.Before(lista.ActualizadaEl) {
		prueba.Fatalf("detalle: %+v %v", detalle, errorObtenido)
	}

	// --- Baja ----------------------------------------------------------------
	if borrado, errorObtenido := repo.EliminarArticulo(contexto, usuarioA, lista.ID, articulo.ID); !borrado || errorObtenido != nil {
		prueba.Fatalf("baja: %v %v", borrado, errorObtenido)
	}
	if borrado, _ := repo.EliminarArticulo(contexto, usuarioA, lista.ID, articulo.ID); borrado {
		prueba.Fatal("baja repetida")
	}
}

func TestAltasSimultaneasSumanSinDuplicar(prueba *testing.T) {
	_, repo, _ := preparar(prueba)
	contexto := context.Background()
	usuario, producto := uuid.NewString(), uuid.NewString()
	lista, errorObtenido := repo.Crear(contexto, usuario, "Concurrente")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repo.Eliminar(context.Background(), usuario, lista.ID) })

	// El pool de preparar tiene 2 conexiones: las altas compiten por el bloqueo de la lista.
	const altas = 8
	errores := make(chan error, altas)
	var grupo sync.WaitGroup
	for range altas {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			_, _, errorAlta := repo.AgregarArticulo(contexto, usuario, lista.ID, producto, 1, listas.Limites{MaxArticulos: 5, MaxCantidad: 99})
			errores <- errorAlta
		}()
	}
	grupo.Wait()
	close(errores)
	for errorAlta := range errores {
		if errorAlta != nil {
			prueba.Fatal(errorAlta)
		}
	}
	detalle, errorObtenido := repo.Obtener(contexto, usuario, lista.ID)
	if errorObtenido != nil || len(detalle.Articulos) != 1 || detalle.Articulos[0].Cantidad != altas {
		prueba.Fatalf("se esperaba un artículo con cantidad %d: %+v %v", altas, detalle, errorObtenido)
	}
}

func TestBorrarListaBorraSusArticulos(prueba *testing.T) {
	_, repo, adminURL := preparar(prueba)
	contexto := context.Background()
	usuario := uuid.NewString()
	lista, errorObtenido := repo.Crear(contexto, usuario, "Cascada")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if _, _, errorObtenido := repo.AgregarArticulo(contexto, usuario, lista.ID, uuid.NewString(), 1, limitesPrueba); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	if eliminada, _ := repo.Eliminar(contexto, usuario, lista.ID); !eliminada {
		prueba.Fatal("no se eliminó")
	}
	// El conteo lo hace el operador, para no depender de lo que app_rutas puede ver.
	admin, errorObtenido := pgx.Connect(contexto, adminURL)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	defer admin.Close(contexto)
	var restantes int
	_ = admin.QueryRow(contexto, `SELECT count(*) FROM rutas.articulos_lista_compras WHERE lista_id = $1`, lista.ID).Scan(&restantes)
	if restantes != 0 {
		prueba.Fatalf("quedaron %d artículos", restantes)
	}
}

func TestMigracionRechazaArticulosInvalidos(prueba *testing.T) {
	pool, repo, _ := preparar(prueba)
	contexto := context.Background()
	usuario := uuid.NewString()
	lista, errorObtenido := repo.Crear(contexto, usuario, "Restricciones")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	prueba.Cleanup(func() { _, _ = repo.Eliminar(context.Background(), usuario, lista.ID) })

	// Los CHECK de la migración 11 aplican a filas nuevas aunque sean NOT VALID.
	for _, sentencia := range []string{
		"INSERT INTO rutas.articulos_lista_compras (lista_id, producto_id, cantidad) VALUES ($1, NULL, 1)",
		"INSERT INTO rutas.articulos_lista_compras (lista_id, producto_id, cantidad) VALUES ($1, gen_random_uuid(), 0)",
	} {
		_, errorObtenido := pool.Exec(contexto, sentencia, lista.ID)
		var pgErr *pgconn.PgError
		if !errors.As(errorObtenido, &pgErr) || pgErr.Code != codigoViolacionCheck {
			prueba.Errorf("%s: se esperaba violación de CHECK (%s), se obtuvo %v", sentencia, codigoViolacionCheck, errorObtenido)
		}
	}
}

// Códigos SQLSTATE que esperan estas pruebas.
const (
	codigoViolacionCheck = "23514"
	codigoSinPermiso     = "42501"
)

func TestAppRutasNoSaleDeSuMatriz(prueba *testing.T) {
	pool, _, _ := preparar(prueba)
	contexto := context.Background()
	for _, sentencia := range []string{
		"SELECT 1 FROM api.usuarios LIMIT 1",
		"SELECT 1 FROM scraper.productos_crudos LIMIT 1",
		"TRUNCATE rutas.listas_compras",
		"TRUNCATE rutas.articulos_lista_compras",
		"CREATE TABLE rutas.intrusa (id int)",
	} {
		_, errorObtenido := pool.Exec(contexto, sentencia)
		var pgErr *pgconn.PgError
		if !errors.As(errorObtenido, &pgErr) || pgErr.Code != codigoSinPermiso {
			prueba.Errorf("%s: se esperaba permiso denegado (%s), se obtuvo %v", sentencia, codigoSinPermiso, errorObtenido)
		}
	}
}
