"""Generic response envelope: {"success": bool, "data": T | None, "error": str | None}."""

from __future__ import annotations

from typing import Generic, TypeVar

from pydantic import BaseModel

T = TypeVar("T")


class ApiResponse(BaseModel, Generic[T]):
    success: bool
    data: T | None = None
    error: str | None = None

    @classmethod
    def ok(cls, data: T) -> ApiResponse[T]:
        return cls(success=True, data=data, error=None)

    @classmethod
    def fail(cls, error: str) -> ApiResponse[T]:
        return cls(success=False, data=None, error=error)
