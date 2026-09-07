from fastapi import APIRouter, Depends

from scheduler_api.domain.capacity import check_capacity
from scheduler_api.schemas.capacity import CapacityCheckRequest, CapacityCheckResult
from scheduler_api.schemas.envelope import ApiResponse
from scheduler_api.security import require_api_key

router = APIRouter(tags=["capacity"], dependencies=[Depends(require_api_key)])


@router.post("/capacity-check", response_model=ApiResponse[CapacityCheckResult])
def capacity_check(req: CapacityCheckRequest) -> ApiResponse[CapacityCheckResult]:
    result = check_capacity(num_days=req.num_days, employee_count=req.employee_count, config=req.config)
    return ApiResponse.ok(result)
