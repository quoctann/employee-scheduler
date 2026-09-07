import logging

from fastapi import FastAPI, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.middleware.base import BaseHTTPMiddleware

from scheduler_api.api.v1 import health
from scheduler_api.api.v1.router import api_router
from scheduler_api.config import settings
from scheduler_api.errors import ReferentialIntegrityError
from scheduler_api.schemas.envelope import ApiResponse

logging.basicConfig(level=settings.log_level)
logger = logging.getLogger(__name__)


class BodySizeLimitMiddleware(BaseHTTPMiddleware):
    """Reject oversized request bodies before they reach pydantic validation.

    Only a fast Content-Length check — a client that omits it or lies can still stream more, but
    this is a cheap first line of defense for an internal service, not the only one (see also
    SolveRequest's field-level and total-variable-count caps).
    """

    async def dispatch(self, request: Request, call_next):
        content_length = request.headers.get("content-length")
        if content_length is not None and int(content_length) > settings.max_body_bytes:
            return JSONResponse(
                status_code=413,
                content=ApiResponse.fail(f"request body exceeds {settings.max_body_bytes} bytes").model_dump(
                    mode="json"
                ),
            )
        return await call_next(request)


app = FastAPI(
    title="Employee Scheduler — Solver API",
    version="0.1.0",
    docs_url="/docs" if settings.enable_docs else None,
    redoc_url="/redoc" if settings.enable_docs else None,
    openapi_url="/openapi.json" if settings.enable_docs else None,
)
app.add_middleware(BodySizeLimitMiddleware)
app.include_router(health.router)  # unprefixed, unauthenticated: /healthz for k8s probes
app.include_router(api_router, prefix="/api/v1")


@app.exception_handler(ReferentialIntegrityError)
async def referential_integrity_handler(request: Request, exc: ReferentialIntegrityError) -> JSONResponse:
    return JSONResponse(status_code=400, content=ApiResponse.fail(str(exc)).model_dump(mode="json"))


@app.exception_handler(RequestValidationError)
async def validation_error_handler(request: Request, exc: RequestValidationError) -> JSONResponse:
    return JSONResponse(status_code=422, content=ApiResponse.fail(str(exc)).model_dump(mode="json"))


@app.exception_handler(HTTPException)
async def http_exception_handler(request: Request, exc: HTTPException) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code, content=ApiResponse.fail(str(exc.detail)).model_dump(mode="json")
    )


@app.exception_handler(Exception)
async def unhandled_exception_handler(request: Request, exc: Exception) -> JSONResponse:
    logger.exception("Unhandled error while processing %s", request.url.path)
    return JSONResponse(
        status_code=500, content=ApiResponse.fail("internal server error").model_dump(mode="json")
    )
