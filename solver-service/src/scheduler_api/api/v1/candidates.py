from fastapi import APIRouter, Depends

from scheduler_api.domain.candidate_ranking import rank_replacement_candidates
from scheduler_api.schemas.candidates import ReplacementCandidatesRequest, ReplacementCandidatesResult
from scheduler_api.schemas.envelope import ApiResponse
from scheduler_api.security import require_api_key

router = APIRouter(tags=["candidates"], dependencies=[Depends(require_api_key)])


@router.post("/replacement-candidates", response_model=ApiResponse[ReplacementCandidatesResult])
def replacement_candidates(
    req: ReplacementCandidatesRequest,
) -> ApiResponse[ReplacementCandidatesResult]:
    result = rank_replacement_candidates(req)
    return ApiResponse.ok(result)
