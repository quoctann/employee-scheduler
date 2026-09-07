from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_prefix="", extra="ignore")

    # required, no default: a missing/misconfigured API_KEY must fail the service at startup
    # rather than silently accepting a well-known value from every caller.
    api_key: str = Field(min_length=8)
    max_time_limit_s: int = 60
    num_search_workers: int = 4
    log_level: str = "INFO"
    # off by default: this service has no Ingress in front of it, but /docs and /openapi.json
    # would otherwise be unauthenticated by construction (FastAPI serves them before any router
    # dependency runs) if that assumption is ever broken later. Opt in for local dev.
    enable_docs: bool = False
    max_body_bytes: int = 10_000_000
    # bounds how many /solve requests run CP-SAT at once; each one spawns num_search_workers
    # native threads, so letting an unbounded number run concurrently on a CPU-limited pod
    # oversubscribes cores and makes every in-flight solve's time_limit_s misleading.
    max_concurrent_solves: int = 2


settings = Settings()
