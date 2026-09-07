import pytest
from pydantic import ValidationError

from scheduler_api.schemas.common import GateShiftRequirement, SolverConfig


def test_default_config_is_valid():
    SolverConfig()  # should not raise


def test_rejects_mismatched_requirements_and_shift_hours_gates():
    with pytest.raises(ValidationError, match="must match"):
        SolverConfig(
            requirements={"A": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)}},
            shift_hours={"B": {"sang": 8, "dem": 8}},
        )


def test_rejects_lead_gates_not_subset_of_gates():
    with pytest.raises(ValidationError, match="subset"):
        SolverConfig(
            requirements={"A": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)}},
            shift_hours={"A": {"sang": 8, "dem": 8}},
            lead_gates={"B"},
        )


def test_rejects_requirements_missing_a_shift_type():
    with pytest.raises(ValidationError, match="missing requirements"):
        SolverConfig(
            requirements={"A": {"sang": GateShiftRequirement(nv=1)}},
            shift_hours={"A": {"sang": 8, "dem": 8}},
            lead_gates=set(),
        )


def test_rejects_shift_hours_missing_a_shift_type():
    with pytest.raises(ValidationError, match="missing shift_hours"):
        SolverConfig(
            requirements={"A": {"sang": GateShiftRequirement(nv=1), "dem": GateShiftRequirement(nv=1)}},
            shift_hours={"A": {"sang": 8}},
            lead_gates=set(),
        )


def test_rejects_lead_requirement_on_a_non_lead_gate():
    with pytest.raises(ValidationError, match="not in lead_gates"):
        SolverConfig(
            requirements={
                "A": {
                    "sang": GateShiftRequirement(nv=1, lead=1),
                    "dem": GateShiftRequirement(nv=1),
                }
            },
            shift_hours={"A": {"sang": 8, "dem": 8}},
            lead_gates=set(),
        )
