import threading

from fastapi import APIRouter, Depends, HTTPException, status

from scheduler_api.config import settings
from scheduler_api.domain.solver import solve_schedule
from scheduler_api.schemas.envelope import ApiResponse
from scheduler_api.schemas.solve import SolveRequest, SolveResult
from scheduler_api.security import require_api_key

router = APIRouter(tags=["solve"], dependencies=[Depends(require_api_key)])

# Each concurrent solve spawns settings.num_search_workers native OR-Tools threads; without a
# cap, enough simultaneous /solve calls oversubscribe the pod's CPU limit and make every
# in-flight solve's time_limit_s misleading. Threading (not asyncio) semaphore: this route runs
# in FastAPI's sync threadpool, not the event loop.
_solve_semaphore = threading.Semaphore(settings.max_concurrent_solves)


@router.post("/solve", response_model=ApiResponse[SolveResult])
def solve(req: SolveRequest) -> ApiResponse[SolveResult]:
    """Solve (or locally re-solve, via `locked_assignments`) a shift schedule.

    Runs CP-SAT synchronously — defined as a plain `def` so FastAPI dispatches the blocking,
    CPU-bound solve onto its threadpool instead of stalling the event loop (e.g. /healthz stays
    responsive while a solve is in flight).
    """
    time_limit_s = min(req.time_limit_s, settings.max_time_limit_s)

    acquired = _solve_semaphore.acquire(timeout=time_limit_s)
    if not acquired:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="solver is at capacity, please retry",
        )
    try:
        result = solve_schedule(
            start_date=req.start_date,
            num_days=req.num_days,
            employees=req.employees,
            availability=req.availability,
            config=req.config,
            locked_assignments=req.locked_assignments,
            carry_in=req.carry_in,
            time_limit_s=time_limit_s,
            num_search_workers=settings.num_search_workers,
        )
    finally:
        _solve_semaphore.release()
    return ApiResponse.ok(result)
