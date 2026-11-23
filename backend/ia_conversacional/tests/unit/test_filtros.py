"""
Tests unitarios para el módulo de saneamiento y filtrado determinista (app.filtros).

Verifica:
1. sanitizar_filtros_crudos: conversión de precios/distancias en string a float, descarte de valores no numéricos y normalización de palabras_clave tanto en lista como en string separado por comas.
2. rescatar_filtros_de_error_groq: extracción robusta de JSON embebido dentro del campo failed_generation ante respuestas de error de Groq.
3. filtrar_productos: ordenamiento ascendente por precio, aplicación de filtros combinados (categoría, precio_max, distancia_max) y prevención de falsos positivos en términos cortos ("bajo consumo" no debe coincidir con "Ajo").
"""

from unittest.mock import MagicMock

import httpx

from app.domain.filtros import filtrar_productos, sanitizar_filtros_crudos, vincular_ingredientes_catalogo
from app.repositories.groq import rescatar_filtros_de_error_groq


def test_sanitizar_filtros_crudos_valores_validos_y_strings_numericos():
    """Verifica que números y strings numéricos se conviertan a float correctamente."""
    crudo = {
        "precio_max": "1500.50",
        "distancia_max": "3.5",
        "categoria": " lacteos ",
        "palabras_clave": [" leche ", "yogurt"],
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert limpio["precio_max"] == 1500.50
    assert limpio["distancia_max"] == 3.5
    assert limpio["categoria"] == " lacteos "
    assert limpio["palabras_clave"] == ["leche", "yogurt"]


def test_sanitizar_filtros_crudos_descarta_valores_invalidos():
    """Verifica que valores de texto no numérico se descarten sin romper los demás campos."""
    crudo = {
        "precio_max": "omision",
        "distancia_max": "desconocida",
        "categoria": "verduras",
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert "precio_max" not in limpio
    assert "distancia_max" not in limpio
    assert limpio["categoria"] == "verduras"


def test_sanitizar_filtros_crudos_palabras_clave_string_separado_por_comas():
    """Verifica que palabras_clave entregado como string separado por comas se transforme en lista limpia."""
    crudo = {
        "palabras_clave": "tomate, cebolla , , ajo",
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert limpio["palabras_clave"] == ["tomate", "cebolla", "ajo"]


def test_rescatar_filtros_de_error_groq_json_en_texto():
    """Verifica la recuperación y saneamiento de filtros desde el campo failed_generation como texto."""
    cuerpo_error = {
        "error": {
            "failed_generation": 'Llamada fallida con {"categoria": "frutas", "precio_max": "2000"} en el payload'
        }
    }
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = cuerpo_error

    resultado = rescatar_filtros_de_error_groq(mock_respuesta)
    assert resultado is not None
    assert resultado["categoria"] == "frutas"
    assert resultado["precio_max"] == 2000.0


def test_rescatar_filtros_de_error_groq_json_en_diccionario():
    """Verifica la recuperación cuando failed_generation ya es un diccionario estructurado."""
    cuerpo_error = {"error": {"failed_generation": {"categoria": "panaderia", "distancia_max": 2.0}}}
    mock_respuesta = MagicMock(spec=httpx.Response)
    mock_respuesta.json.return_value = cuerpo_error

    resultado = rescatar_filtros_de_error_groq(mock_respuesta)
    assert resultado == {"categoria": "panaderia", "distancia_max": 2.0}


def test_rescatar_filtros_de_error_groq_retorna_none_si_no_hay_datos_utiles():
    """Verifica retorno None ante respuestas de error sin failed_generation o con JSON inválido."""
    mock_respuesta_sin_error = MagicMock(spec=httpx.Response)
    mock_respuesta_sin_error.json.return_value = {"error": {}}
    assert rescatar_filtros_de_error_groq(mock_respuesta_sin_error) is None

    mock_respuesta_texto_plano = MagicMock(spec=httpx.Response)
    mock_respuesta_texto_plano.json.return_value = {"error": {"failed_generation": "sin llaves"}}
    assert rescatar_filtros_de_error_groq(mock_respuesta_texto_plano) is None


def test_filtrar_productos_orden_ascendente_por_precio():
    """Verifica que los productos resultantes se ordenen estrictamente de menor a mayor precio."""
    catalogo = [
        {"id": 1, "nombre": "Pan Hallulla", "precio": 1800.0, "categoria": "panaderia", "distancia_km": 1.0},
        {"id": 2, "nombre": "Pan Marraqueta", "precio": 1400.0, "categoria": "panaderia", "distancia_km": 1.0},
        {"id": 3, "nombre": "Pan Integral", "precio": 2100.0, "categoria": "panaderia", "distancia_km": 1.0},
    ]
    filtros = {"categoria": "panaderia"}
    filtrados = filtrar_productos(catalogo, filtros)

    assert len(filtrados) == 3
    precios = [producto["precio"] for producto in filtrados]
    assert precios == [1400.0, 1800.0, 2100.0]


def test_filtrar_productos_categoria_precio_y_distancia_combinados():
    """Verifica la aplicación simultánea de filtros de categoría, precio máximo y distancia máxima."""
    catalogo = [
        {"id": 1, "nombre": "Leche Entera", "precio": 1100.0, "categoria": "lacteos", "distancia_km": 1.0},
        {"id": 2, "nombre": "Leche Descremada", "precio": 1600.0, "categoria": "lacteos", "distancia_km": 1.0},
        {"id": 3, "nombre": "Leche Sin Lactosa", "precio": 1200.0, "categoria": "lacteos", "distancia_km": 4.5},
        {"id": 4, "nombre": "Manzana", "precio": 900.0, "categoria": "frutas", "distancia_km": 0.5},
    ]
    filtros = {
        "categoria": "lacteos",
        "precio_max": 1500.0,
        "distancia_max": 2.0,
    }
    filtrados = filtrar_productos(catalogo, filtros)

    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Leche Entera"


def test_filtrar_productos_proteccion_falsos_positivos_cortos():
    """
    Verifica que frases como 'bajo consumo' no coincidan con 'Ajo'.
    Para tokens cortos (< 4 caracteres), debe exigirse igualdad estricta.
    """
    catalogo = [
        {"id": 10, "nombre": "Ajo Chilote", "categoria": "verduras", "precio": 600.0, "distancia_km": 1.0},
        {
            "id": 20,
            "nombre": "Ampolleta bajo consumo",
            "categoria": "iluminacion",
            "precio": 1500.0,
            "distancia_km": 1.0,
        },
    ]
    filtros = {"categoria": "bajo consumo"}
    filtrados = filtrar_productos(catalogo, filtros)

    nombres = [producto["nombre"] for producto in filtrados]
    assert "Ampolleta bajo consumo" in nombres
    assert "Ajo Chilote" not in nombres


def test_filtrar_productos_coincide_ignorando_tildes():
    """Verifica que las tildes se pliegan al comparar: 'limon' calza con 'Limón'."""
    catalogo = [
        {"id": 1, "nombre": "Limón", "categoria": "frutas", "precio": 700.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Manzana", "categoria": "frutas", "precio": 600.0, "distancia_km": 1.0},
    ]
    filtros = {"palabras_clave": ["limon"]}
    filtrados = filtrar_productos(catalogo, filtros)

    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Limón"


def test_filtrar_productos_coincide_con_palabra_corta_del_nombre():
    """Verifica que una palabra corta del nombre ('sal') calza por igualdad exacta."""
    catalogo = [
        {"id": 1, "nombre": "Sal de mar", "categoria": "especias", "precio": 900.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Ajo Chilote", "categoria": "verduras", "precio": 600.0, "distancia_km": 1.0},
    ]
    filtros = {"palabras_clave": ["sal"]}
    filtrados = filtrar_productos(catalogo, filtros)

    nombres = [producto["nombre"] for producto in filtrados]
    assert nombres == ["Sal de mar"]


def test_filtrar_productos_filtra_por_palabras_clave():
    """Verifica que palabras_clave conserva solo los productos que coinciden con la clave."""
    catalogo = [
        {"id": 1, "nombre": "Leche Entera", "categoria": "lacteos", "precio": 1100.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Manzana", "categoria": "frutas", "precio": 600.0, "distancia_km": 1.0},
    ]
    filtros = {"palabras_clave": ["leche"]}
    filtrados = filtrar_productos(catalogo, filtros)

    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Leche Entera"


def test_sanitizar_filtros_crudos_ingredientes_validos_con_defaults():
    """Verifica saneamiento de ingredientes de receta: nombre limpio y cantidad con default."""
    crudo = {
        "ingredientes": [
            {"nombre": " tomate ", "cantidad_envases": 2},
            {"nombre": "cebolla"},
        ],
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert limpio["ingredientes"] == [
        {"nombre": "tomate", "cantidad_envases": 2},
        {"nombre": "cebolla", "cantidad_envases": 1},
    ]


def test_sanitizar_filtros_crudos_ingredientes_invalidos_se_descartan():
    """Verifica descarte de ingredientes mal tipados y default de cantidad invalida."""
    crudo = {
        "ingredientes": [
            {"nombre": "  ", "cantidad_envases": 2},
            {"nombre": "aguacate", "cantidad_envases": True},
            {"nombre": 42, "cantidad_envases": 2},
            {"cantidad_envases": 2},
            {"nombre": "pimiento", "cantidad_envases": 0},
        ],
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert limpio["ingredientes"] == [
        {"nombre": "aguacate", "cantidad_envases": 1},
        {"nombre": "pimiento", "cantidad_envases": 1},
    ]


def test_sanitizar_filtros_crudos_ingredientes_lista_vacia_no_se_guarda():
    """Verifica que una lista de ingredientes vacia no se guarda en el filtro limpio."""
    limpio = sanitizar_filtros_crudos({"ingredientes": []})
    assert "ingredientes" not in limpio


def test_vincular_ingredientes_catalogo_encuentra_candidatos_y_falta():
    """Verifica vinculacion de ingredientes con candidatos del catalogo y nombres faltantes."""
    catalogo = [
        {"id": 1, "nombre": "Tomate", "categoria": "abrupto", "precio": 1500.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Tomate", "categoria": "abrupto", "precio": 900.0, "distancia_km": 1.0},
        {"nombre": "Cilantro", "categoria": "condimentos", "precio": 800.0, "distancia_km": 1.0},
    ]
    ingredientes = [
        {"nombre": "Tomate", "cantidad_envases": 2},
        {"nombre": "Perejil", "cantidad_envases": 1},
    ]

    receta, faltantes = vincular_ingredientes_catalogo(catalogo, ingredientes)

    assert len(receta) == 1
    assert receta[0]["ingrediente"] == "Tomate"
    assert receta[0]["cantidad_sugerida"] == 2
    assert len(receta[0]["candidatos"]) == 2
    assert receta[0]["candidatos"][0]["producto_id"] == "2"
    assert receta[0]["candidatos"][0]["nombre"] == "Tomate"
    assert receta[0]["candidatos"][0]["precio"] == 900.0
    assert receta[0]["candidatos"][1]["producto_id"] == "1"
    assert faltantes == ["Perejil"]


def test_vincular_ingredientes_catalogo_orden_y_limite_de_candidatos():
    """Verifica que los candidatos se ordenen por precio ascendente y se respeten el limite."""
    catalogo = [
        {"id": 1, "nombre": "Tomate", "categoria": "abrupto", "precio": 1500.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Tomate", "categoria": "abrupto", "precio": 900.0, "distancia_km": 1.0},
        {"id": 3, "nombre": "Tomate", "categoria": "abrupto", "precio": 1200.0, "distancia_km": 1.0},
        {"id": 4, "nombre": "Tomate", "categoria": "abrupto", "precio": 700.0, "distancia_km": 1.0},
    ]

    receta, faltantes = vincular_ingredientes_catalogo(
        catalogo, [{"nombre": "Tomate", "cantidad_envases": 1}], limite_candidatos=2
    )

    candidatos = receta[0]["candidatos"]
    assert [candidato["producto_id"] for candidato in candidatos] == ["4", "2"]
    assert faltantes == []


def test_filtrar_productos_coincide_frase_multipalabra_aji_verde():
    """Verifica que 'aji verde' no calza con 'Té Verde Supremo' ni 'Aceitunas Verdes'."""
    catalogo = [
        {"id": 1, "nombre": "Té Verde Supremo", "categoria": "especias", "precio": 900.0, "distancia_km": 1.0},
        {"id": 2, "nombre": "Aceitunas Verdes", "categoria": "conservas", "precio": 700.0, "distancia_km": 1.0},
        {"id": 3, "nombre": "Ají Verde 250g", "categoria": "especias", "precio": 800.0, "distancia_km": 1.0},
    ]
    filtros = {"categoria": "aji verde"}
    filtrados = filtrar_productos(catalogo, filtros)

    assert len(filtrados) == 1
    assert filtrados[0]["nombre"] == "Ají Verde 250g"


def test_sanitizar_filtros_crudos_ingredientes_float_entero_conviene_a_int():
    """Verifica que una cantidad de envases entera en float (2.0) se convierte a int 2."""
    crudo = {
        "ingredientes": [
            {"nombre": "tomate", "cantidad_envases": 2.0},
        ],
    }
    limpio = sanitizar_filtros_crudos(crudo)
    assert limpio["ingredientes"] == [{"nombre": "tomate", "cantidad_envases": 2}]
