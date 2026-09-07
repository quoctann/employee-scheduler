from pathlib import Path

from scheduler_api.config import SERVICE_DIR, Settings


def test_env_file_is_anchored_to_solver_service_directory():
    env_file = Path(Settings.model_config["env_file"])

    assert env_file.is_absolute()
    assert env_file == SERVICE_DIR / ".env"
