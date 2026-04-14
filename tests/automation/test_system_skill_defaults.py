from __future__ import annotations

from agent.service.capability.skills.skill_catalog import SkillCatalog
from agent.service.workspace.workspace_skill_deployer import WorkspaceSkillDeployer


def test_scheduled_task_manager_is_a_system_skill():
    catalog = SkillCatalog()
    record = catalog.get_record("scheduled-task-manager")

    assert record is not None
    assert record.detail.source_type == "system"
    assert record.detail.locked is True


def test_workspace_skill_deployer_installs_scheduled_task_manager_for_all_agents():
    assert "memory-manager" in WorkspaceSkillDeployer.BASE_SKILL_NAMES
    assert "scheduled-task-manager" in WorkspaceSkillDeployer.BASE_SKILL_NAMES
    assert "nexus-manager" not in WorkspaceSkillDeployer.BASE_SKILL_NAMES
