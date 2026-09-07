import os

# must be set before any `scheduler_api.config`/`scheduler_api.main` import anywhere in the
# suite, since Settings() is instantiated at module import time and api_key has no default.
os.environ.setdefault("API_KEY", "test-api-key-not-a-real-secret")
