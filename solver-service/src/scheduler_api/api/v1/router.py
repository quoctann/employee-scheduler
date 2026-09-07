from fastapi import APIRouter

from scheduler_api.api.v1 import candidates, capacity, solve

api_router = APIRouter()
api_router.include_router(solve.router)
api_router.include_router(capacity.router)
api_router.include_router(candidates.router)
