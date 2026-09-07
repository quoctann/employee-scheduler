"""Domain-level exceptions, mapped to HTTP responses in main.py."""


class ReferentialIntegrityError(ValueError):
    """Raised when a request references an id/date that doesn't exist in the same payload.

    e.g. a locked_assignment pointing at an employee_id not present in `employees`.
    Mapped to HTTP 400 (bad request semantics), distinct from Pydantic 422 shape errors.
    """
