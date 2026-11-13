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

func TestDetalleConArticulosYBorradoEnCascada(prueba *testing.T) {
	_, repo, adminURL := preparar(prueba)
	contexto := context.Background()
	usuario := uuid.NewString()
	lista, errorObtenido := repo.Crear(contexto, usuario, "Con artículos")
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	// Los artículos los escribe el operador: app_rutas todavía no tiene INSERT (SUP-241).
	admin, errorObtenido := pgx.Connect(contexto, adminURL)
	if errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	defer admin.Close(contexto)
	var producto string
	if errorObtenido := admin.QueryRow(contexto, `INSERT INTO api.productos_normalizados (nombre_estandar)
        VALUES ('Leche prueba SUP-348') RETURNING id::text`).Scan(&producto); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}
	defer admin.Exec(contexto, `DELETE FROM api.productos_normalizados WHERE id = $1`, producto)
	if _, errorObtenido := admin.Exec(contexto, `INSERT INTO rutas.articulos_lista_compras (lista_id, producto_normalizado_id, cantidad)
        VALUES ($1, $2, 2)`, lista.ID, producto); errorObtenido != nil {
		prueba.Fatal(errorObtenido)
	}

	detalle, errorObtenido := repo.Obtener(contexto, usuario, lista.ID)
	if errorObtenido != nil || detalle.TotalArticulos != 1 || detalle.Articulos[0].Cantidad != 2 || detalle.Articulos[0].ProductoNormalizadoID != producto {
		prueba.Fatalf("detalle: %+v %v", detalle, errorObtenido)
	}
	if eliminada, _ := repo.Eliminar(contexto, usuario, lista.ID); !eliminada {
		prueba.Fatal("no se eliminó")
	}
	var restantes int
	_ = admin.QueryRow(contexto, `SELECT count(*) FROM rutas.articulos_lista_compras WHERE lista_id = $1`, lista.ID).Scan(&restantes)
	if restantes != 0 {
		prueba.Fatalf("quedaron %d artículos", restantes)
	}
}

func TestAppRutasNoSaleDeSuMatriz(prueba *testing.T) {
	pool, _, _ := preparar(prueba)
	contexto := context.Background()
	for _, sentencia := range []string{
		"SELECT 1 FROM api.usuarios LIMIT 1",
		"SELECT 1 FROM scraper.productos_crudos LIMIT 1",
		"INSERT INTO rutas.articulos_lista_compras (lista_id, producto_normalizado_id) VALUES ('" +
			uuid.NewString() + "', '" + uuid.NewString() + "')",
		"TRUNCATE rutas.listas_compras",
		"CREATE TABLE rutas.intrusa (id int)",
	} {
		_, errorObtenido := pool.Exec(contexto, sentencia)
		var pgErr *pgconn.PgError
		if !errors.As(errorObtenido, &pgErr) || pgErr.Code != "42501" {
			prueba.Errorf("%s: se esperaba permiso denegado (42501), se obtuvo %v", sentencia, errorObtenido)
		}
	}
}
