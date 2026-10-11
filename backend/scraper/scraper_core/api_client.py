from __future__ import annotations

import json
from urllib.error import HTTPError, URLError
from urllib.request import Request, HTTPRedirectHandler, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class ScraperAPIError(RuntimeError):
    """Indica que la API no aceptó o no pudo procesar una ingesta."""

    def __init__(self, message, *, status=None, code=None):
        super().__init__(message)
        self.status = status
        self.code = code

    @property
    def permanent(self):
        return self.status in (400, 404, 413) or self.code in ("idempotency_conflict", "entrega_fallida", "lote_rechazado")


class ScraperAPIClient:
    """Cliente HTTP síncrono para enviar lotes al API Gateway."""

    def __init__(self, endpoint: str, timeout: float = 30.0, opener=None):
        self.endpoint = endpoint.rstrip("/")
        self.timeout = timeout
        self.opener = opener or build_opener(NoRedirect()).open

    def ingest_batch(
        self,
        products: list[dict],
        supermarket: str | None = None,
        branch_id: int | None = None,
        branch_code: str | None = None,
        work_id: str | None = None,
        batch_id: str | None = None,
    ) -> dict:
        if not products:
            return {"total_recibidos": 0}

        payload = {"productos": products}
        if supermarket:
            payload["supermercado"] = supermarket
        if branch_id is not None:
            payload["sucursal_id"] = branch_id
        if branch_code:
            payload["codigo_sucursal"] = branch_code

        endpoint = self._ingestion_endpoint(work_id)

        request = Request(
            endpoint,
            data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
            },
            method="POST",
        )

        if batch_id:
            request.add_header("Idempotency-Key", batch_id)

        try:
            with self.opener(request, timeout=self.timeout) as response:
                body = response.read().decode("utf-8")
        except HTTPError as error:
            detail = error.read().decode("utf-8", errors="replace")
            try:
                code = json.loads(detail).get("codigo")
            except (json.JSONDecodeError, AttributeError):
                code = None
            raise ScraperAPIError(
                f"La API rechazó la ingesta ({error.code}): {code or error.reason}", status=error.code, code=code
            ) from error
        except (URLError, TimeoutError, OSError) as error:
            raise ScraperAPIError("No se pudo conectar con la API") from error

        if not body:
            if batch_id:
                raise ScraperAPIError("La API no confirmó el lote")
            return {}
        try:
            result = json.loads(body)
            if not isinstance(result, dict):
                raise ScraperAPIError("La API no confirmó el lote")
            if result.get("codigo") == "entrega_pendiente":
                raise ScraperAPIError("Entrega pendiente de confirmación", status=202, code="entrega_pendiente")
            if batch_id and (result.get("total_recibidos") != len(products) or
                             not all(type(result.get(field)) is int for field in ("total_recibidos", "precios_registrados", "insertados", "actualizados")) or
                             result["precios_registrados"] != result.get("insertados", -1) + result.get("actualizados", -1)):
                raise ScraperAPIError("La API devolvió un recibo de lote inconsistente")
            return result
        except json.JSONDecodeError as error:
            raise ScraperAPIError("La API respondió con JSON inválido") from error

    def finalize_work(
        self,
        work_id: str,
        status: str,
        extracted_items: int | None = None,
        error_message: str | None = None,
    ) -> dict:
        payload = {
            "estado": status,
        }
        if extracted_items is not None:
            payload["elementos_extraidos"] = extracted_items
        if error_message:
            payload["registro_errores"] = error_message

        endpoint = self.endpoint
        if endpoint.endswith("/productos"):
            endpoint = endpoint[: -len("/productos")]
        endpoint = f"{endpoint}/trabajos/{work_id}/finalizar"
        request = Request(
            endpoint,
            data=json.dumps(payload).encode("utf-8"),
            headers={
                "Accept": "application/json",
                "Content-Type": "application/json",
            },
            method="PUT",
        )
        try:
            with self.opener(request, timeout=self.timeout) as response:
                body = response.read().decode("utf-8")
        except HTTPError as error:
            detail = error.read().decode("utf-8", errors="replace")
            raise ScraperAPIError(
                f"La API rechazó el cierre del trabajo ({error.code})", status=error.code
            ) from error
        except (URLError, TimeoutError, OSError) as error:
            raise ScraperAPIError("No se pudo conectar con la API") from error

        try:
            result = json.loads(body)
            if not isinstance(result, dict) or result.get("id") != work_id or result.get("estado") != status:
                raise ScraperAPIError("La API no confirmó el cierre del trabajo")
            return result
        except json.JSONDecodeError as error:
            raise ScraperAPIError("La API respondió con JSON inválido al cerrar el trabajo") from error

    def _ingestion_endpoint(self, work_id: str | None) -> str:
        if work_id:
            endpoint = self.endpoint
            if endpoint.endswith("/productos"):
                endpoint = endpoint[: -len("/productos")]
            return f"{endpoint}/trabajos/{work_id}/productos"
        if self.endpoint.endswith("/productos"):
            return self.endpoint
        return f"{self.endpoint}/productos"

    def get_work(self, work_id: str) -> dict:
        endpoint = self.endpoint.removesuffix("/productos")
        request = Request(f"{endpoint}/trabajos/{work_id}", headers={"Accept": "application/json"})
        try:
            with self.opener(request, timeout=self.timeout) as response:
                result = json.loads(response.read().decode("utf-8"))
                if not isinstance(result, dict) or result.get("id") != work_id or result.get("estado") not in ("en_progreso", "completado", "fallido"):
                    raise ScraperAPIError("Estado de trabajo inconsistente")
                return result
        except HTTPError as error:
            raise ScraperAPIError(f"No se pudo consultar trabajo ({error.code})", status=error.code) from error
        except (URLError, TimeoutError, OSError, json.JSONDecodeError) as error:
            raise ScraperAPIError("No se pudo consultar trabajo") from error
