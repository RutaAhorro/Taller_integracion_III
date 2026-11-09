#!/usr/bin/env python3
"""
Pruebas unitarias para el validador de nomenclatura scripts/check_single_letter_identifiers.py.
Verifica:
1. Deteccion de casos comunes (arrows tipados, desestructuracion, Go methods/:=, Python multilinea).
2. Ausencia de falsos positivos en comentarios, docstrings y plantillas multilinea.
3. Hostile Corpus JS/TS: regexes con comillas, escapes de backticks, expresiones interpoladas ${...}.
4. Comportamiento estricto ante bases no validas o ejecucion fuera de git.
5. Soporte de modo advertencia (--warning) con retorno 0.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

# Importar funciones del validador
from check_single_letter_identifiers import (
    analizar_archivo,
    analizar_go,
    analizar_js_ts,
    analizar_python,
    enmascarar_comentarios_y_cadenas,
    extraer_lineas_modificadas_por_archivo,
    obtener_diff,
)


class PruebaValidadorNomenclatura(unittest.TestCase):
    """Bateria de pruebas unitarias para detecciones de una sola letra y Hostile Corpus."""

    # --- Casos JS / TS Base ---

    def test_javascript_arrow_sin_parentesis(self) -> None:
        codigo = "const procesar = r => r.id;"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones))

    def test_javascript_arrow_con_parentesis_y_tipos(self) -> None:
        codigo_sin_tipo = "const procesar = (r) => r.id;"
        codigo_con_tipo = "const procesar = (r: Respuesta) => r.id;"

        violaciones_sin_tipo = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_sin_tipo, ".ts"))
        violaciones_con_tipo = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_con_tipo, ".ts"))

        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones_sin_tipo))
        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones_con_tipo))

    def test_javascript_desestructuracion_arreglo_y_objeto(self) -> None:
        codigo_arreglo = "const [r, actualizarRespuesta] = useState(null);"
        codigo_objeto = "const { r, datos } = propiedades;"
        codigo_objeto_alias = "const { respuesta: r } = propiedades;"

        violaciones_arreglo = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_arreglo, ".tsx"))
        violaciones_objeto = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_objeto, ".tsx"))
        violaciones_alias = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_objeto_alias, ".tsx"))

        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones_arreglo))
        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones_objeto))
        self.assertTrue(any(identificador == "r" for _, identificador, _ in violaciones_alias))

    def test_javascript_excepcion_guion_bajo_permitida(self) -> None:
        codigo_descarte = "const [_, setRespuesta] = useState(null);"
        violaciones = analizar_js_ts(enmascarar_comentarios_y_cadenas(codigo_descarte, ".tsx"))
        self.assertEqual(len(violaciones), 0)

    def test_javascript_comentarios_y_plantillas_ignorados(self) -> None:
        codigo = """// const r = 1;
/*
   const w = 2;
   const procesar = (r) => r.id;
*/
const plantilla = `
   ejemplo con const c = 3;
   y llamada con (r) => r.id
`;
const respuestaCorrecta = true;
"""
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        self.assertEqual(len(violaciones), 0)

    # --- Hostile Corpus JS / TS (Problemas P2 Resueltos) ---

    def test_javascript_regex_con_comillas_no_oculta_variables_posteriores(self) -> None:
        codigo = """const patron = /["']/g;
const r = 1;"""
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("r", identificadores)

    def test_javascript_plantilla_con_backtick_escapado_no_genera_falsos_positivos(self) -> None:
        codigo = """const plantilla = `comilla escapada: \\`
const r = ejemplo
`;
const respuestaValida = true;"""
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertNotIn("r", identificadores)
        self.assertEqual(len(violaciones), 0)

    def test_javascript_codigo_ejecutable_dentro_de_interpolacion(self) -> None:
        codigo = "const texto = `${elementos.map((r) => r.id)}`;"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("r", identificadores)

    def test_javascript_interpolacion_con_llaves_y_objetos_anidados(self) -> None:
        codigo = "const texto = `${{ prop: (r) => r.id }}`; const r = 10;"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("r", identificadores)

    def test_javascript_division_aritmetica_no_se_confunde_con_regex(self) -> None:
        codigo = "const division = total / 2 / factor;"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".ts")
        violaciones = analizar_js_ts(codigo_enmascarado)
        self.assertEqual(len(violaciones), 0)

    # --- Casos Go ---

    def test_go_asignacion_corta_multiple(self) -> None:
        codigo_asignacion = "c, err := cargarConfiguracion()"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo_asignacion, ".go")
        violaciones = analizar_go(codigo_enmascarado)
        self.assertTrue(any(identificador == "c" for _, identificador, _ in violaciones))
        self.assertFalse(any(identificador == "err" for _, identificador, _ in violaciones))

    def test_go_firma_metodo_receptor_y_parametros(self) -> None:
        codigo_metodo = "func (servidor *Servidor) Atender(w http.ResponseWriter, r *http.Request) {}"
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo_metodo, ".go")
        violaciones = analizar_go(codigo_enmascarado)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("w", identificadores)
        self.assertIn("r", identificadores)
        self.assertNotIn("servidor", identificadores)

    def test_go_receptor_invalido_de_una_letra(self) -> None:
        codigo_receptor = "func (s *Servidor) Metodo() {}"
        violaciones = analizar_go(enmascarar_comentarios_y_cadenas(codigo_receptor, ".go"))
        self.assertTrue(any(identificador == "s" for _, identificador, _ in violaciones))

    def test_go_comentarios_y_cadenas_raw_ignorados(self) -> None:
        codigo = """// c, err := cargarConfiguracion()
/*
   func (s *Servidor) Atender(w http.ResponseWriter) {}
*/
const consulta = `
   SELECT c, r FROM tabla WHERE x = 1
`
func FuncionCorrecta(servidor *Servidor, peticion *http.Request) {}
"""
        codigo_enmascarado = enmascarar_comentarios_y_cadenas(codigo, ".go")
        violaciones = analizar_go(codigo_enmascarado)
        self.assertEqual(len(violaciones), 0)

    # --- Casos Python ---

    def test_python_parametros_multilinea(self) -> None:
        codigo = """
def funcion_multilinea(
    x,
    y: int,
    respuesta_valida: str,
):
    pass
"""
        violaciones = analizar_python(codigo)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("x", identificadores)
        self.assertIn("y", identificadores)
        self.assertNotIn("respuesta_valida", identificadores)

    def test_python_comentarios_y_docstrings_ignorados(self) -> None:
        codigo = '''# c, err = cargar()
"""
Docstring de modulo con ejemplos:
x = 1
for r in lista:
    c = 2
"""
def funcion_ejecutar(configuracion: dict) -> None:
    """Docstring de funcion con x = 10"""
    nombre_usuario = "renato"
'''
        violaciones = analizar_python(codigo)
        self.assertEqual(len(violaciones), 0)

    def test_python_asignaciones_y_bucles(self) -> None:
        codigo = """
c, error_obtenido = cargar_datos()
for r in elementos:
    _ = r
resultado = [p for p in lista_elementos]
"""
        violaciones = analizar_python(codigo)
        identificadores = [identificador for _, identificador, _ in violaciones]
        self.assertIn("c", identificadores)
        self.assertIn("r", identificadores)
        self.assertIn("p", identificadores)
        self.assertNotIn("_", identificadores)

    # --- Integracion y Manejo de Errores de Git ---

    def test_extraccion_lineas_diff(self) -> None:
        diff_simulado = """--- a/archivo.go
+++ b/archivo.go
@@ -10,0 +11,3 @@
+linea1
+linea2
+linea3
@@ -20,1 +24 @@
+lineaModificada
"""
        lineas_por_archivo = extraer_lineas_modificadas_por_archivo(diff_simulado)
        self.assertIn("archivo.go", lineas_por_archivo)
        self.assertEqual(lineas_por_archivo["archivo.go"], {11, 12, 13, 24})

    def test_filtro_por_lineas_modificadas(self) -> None:
        with tempfile.NamedTemporaryFile("w+", suffix=".go", delete=False) as archivo_temporal:
            archivo_temporal.write("""package main
func ServidorLegado(c int) {}
func ServidorNuevo(w http.ResponseWriter) {}
""")
            ruta_temporal = archivo_temporal.name

        try:
            reportes = analizar_archivo(ruta_temporal, lineas_filtradas={3})
            self.assertEqual(len(reportes), 1)
            self.assertIn(":3: identificador 'w'", reportes[0])
            self.assertNotIn("identificador 'c'", reportes[0])
        finally:
            Path(ruta_temporal).unlink(missing_ok=True)

    def test_base_no_existente_falla_estrictamente(self) -> None:
        argumentos = argparse.Namespace(base="referencia_que_no_existe_12345", staged=False)
        with self.assertRaises(SystemExit) as contexto_exit:
            obtener_diff(argumentos)
        self.assertEqual(contexto_exit.exception.code, 2)

    def test_modo_advertencia_retorna_cero(self) -> None:
        with tempfile.NamedTemporaryFile("w+", suffix=".go", delete=False) as archivo_temporal:
            archivo_temporal.write("package main\nfunc ServidorInvalido(c int) {}\n")
            ruta_temporal = archivo_temporal.name

        try:
            ruta_script = Path(__file__).resolve().parent / "check_single_letter_identifiers.py"
            resultado_estricto = subprocess.run(
                [sys.executable, str(ruta_script), "--file", ruta_temporal],
                capture_output=True,
                text=True
            )
            resultado_advertencia = subprocess.run(
                [sys.executable, str(ruta_script), "--file", ruta_temporal, "--warning"],
                capture_output=True,
                text=True
            )
            self.assertEqual(resultado_estricto.returncode, 1)
            self.assertEqual(resultado_advertencia.returncode, 0)
            self.assertIn("⚠️ Advertencia", resultado_advertencia.stdout)
        finally:
            Path(ruta_temporal).unlink(missing_ok=True)


if __name__ == "__main__":
    unittest.main()
