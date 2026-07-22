#!/usr/bin/env python3
"""API e2e: custom agents visibility, multi-create, builtin hidden.

Requires a running backend (default http://127.0.0.1:8080).
Accounts (password Test1234!):
  e2e-admin@test.local   tenant admin in 赤峰市本级
  e2e-viewer@test.local  viewer in same org unit
  b@qq.com               contributor in 赤峰市 (parent, different unit)
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from typing import Any

BASE = os.environ.get("WEKNORA_E2E_HOST", "http://127.0.0.1:8080").rstrip("/")
API = f"{BASE}/api/v1"
PASSWORD = os.environ.get("WEKNORA_E2E_PASSWORD", "Test1234!")
ORG_UNIT_HEADER = "X-Org-Unit-ID"
CFBJ = "2b04ec32-9da7-4714-b4da-df3df7411a66"  # 赤峰市本级
CF = "d60fec9c-f805-4c45-a900-f2df87eead8b"  # 赤峰市

PASS = 0
FAIL = 0

# Bypass local HTTP proxies for loopback.
proxy_handler = urllib.request.ProxyHandler({})
opener = urllib.request.build_opener(proxy_handler)
urllib.request.install_opener(opener)


def req(
    method: str,
    path: str,
    token: str | None = None,
    body: dict[str, Any] | None = None,
    tenant: int | str | None = None,
    org_unit: str | None = None,
) -> tuple[int, Any]:
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if tenant is not None:
        headers["X-Tenant-ID"] = str(tenant)
    if org_unit:
        headers[ORG_UNIT_HEADER] = org_unit
    request = urllib.request.Request(
        API + path, data=data, headers=headers, method=method
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            raw = response.read().decode()
            payload = json.loads(raw) if raw else {}
            return response.status, payload
    except urllib.error.HTTPError as error:
        raw = error.read().decode()
        try:
            payload = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            payload = {"raw": raw}
        return error.code, payload


def check(name: str, ok: bool, detail: str = "") -> None:
    global PASS, FAIL
    if ok:
        PASS += 1
        print(f"PASS: {name}")
    else:
        FAIL += 1
        print(f"FAIL: {name} {detail}".rstrip())


def login(email: str) -> tuple[str, int]:
    status, payload = req(
        "POST", "/auth/login", body={"email": email, "password": PASSWORD}
    )
    if status != 200 or not payload.get("token"):
        raise RuntimeError(f"login failed for {email}: {status} {payload}")
    tenant_id = (payload.get("active_tenant") or {}).get("id")
    return payload["token"], int(tenant_id)


def agent_ids(payload: Any) -> list[str]:
    return [item.get("id", "") for item in (payload.get("data") or [])]


def agent_names(payload: Any) -> list[str]:
    return [item.get("name", "") for item in (payload.get("data") or [])]


def first_builtin_knowledge_qa(
    token: str, tenant: int, org_unit: str
) -> str:
    """Resolve a builtin KnowledgeQA model.

    Tenant admins may be forbidden from listing models; fall back to an
    env override or a system-admin login when needed.
    """
    env_model = os.environ.get("WEKNORA_E2E_BUILTIN_MODEL_ID", "").strip()
    if env_model:
        return env_model

    candidates: list[tuple[str, int, str | None]] = [
        (token, tenant, org_unit),
    ]
    try:
        sa_token, sa_tenant = login("421099982@qq.com")
        candidates.append((sa_token, sa_tenant, None))
    except RuntimeError:
        pass

    for cand_token, cand_tenant, cand_org in candidates:
        status, payload = req(
            "GET",
            "/models",
            token=cand_token,
            tenant=cand_tenant,
            org_unit=cand_org,
        )
        if status != 200:
            continue
        models = payload.get("data") or []
        for model in models:
            if (
                model.get("is_builtin")
                and model.get("type") == "KnowledgeQA"
                and model.get("id")
            ):
                check("resolve builtin KnowledgeQA", True)
                return str(model["id"])
    raise RuntimeError(
        "no builtin KnowledgeQA model found "
        "(set WEKNORA_E2E_BUILTIN_MODEL_ID or mark one model is_builtin=true)"
    )


def main() -> int:
    stamp = int(time.time())
    name_a = f"e2e-agent-a-{stamp}"
    name_b = f"e2e-agent-b-{stamp}"
    created_ids: list[str] = []

    admin_token, admin_tenant = login("e2e-admin@test.local")
    viewer_token, viewer_tenant = login("e2e-viewer@test.local")
    other_token, other_tenant = login("b@qq.com")

    model_id = first_builtin_knowledge_qa(admin_token, admin_tenant, CFBJ)

    # Chat list hides builtins
    status, chat_list = req(
        "GET",
        "/agents?purpose=chat",
        token=admin_token,
        tenant=admin_tenant,
        org_unit=CFBJ,
    )
    ids = agent_ids(chat_list)
    check(
        "chat list has no builtins",
        status == 200 and not any(item.startswith("builtin-") for item in ids),
        f"status={status} ids={ids}",
    )

    # Viewer cannot create
    status, _ = req(
        "POST",
        "/agents",
        token=viewer_token,
        tenant=viewer_tenant,
        org_unit=CFBJ,
        body={
            "name": f"viewer-denied-{stamp}",
            "config": {"model_id": model_id, "agent_mode": "quick-answer"},
        },
    )
    check("viewer create denied", status in (401, 403), f"status={status}")

    # Admin creates two agents (multi-create)
    for agent_name in (name_a, name_b):
        status, payload = req(
            "POST",
            "/agents",
            token=admin_token,
            tenant=admin_tenant,
            org_unit=CFBJ,
            body={
                "name": agent_name,
                "description": "e2e multi-create",
                "config": {
                    "model_id": model_id,
                    "agent_mode": "quick-answer",
                    "kb_selection_mode": "none",
                },
            },
        )
        agent_id = (payload.get("data") or {}).get("id")
        check(
            f"admin create {agent_name}",
            status in (200, 201) and bool(agent_id),
            f"status={status} payload={payload}",
        )
        if agent_id:
            created_ids.append(agent_id)
            org_unit_id = (payload.get("data") or {}).get("org_unit_id")
            check(
                f"{agent_name} stamped org_unit",
                org_unit_id == CFBJ,
                f"org_unit_id={org_unit_id}",
            )

    status, manage_list = req(
        "GET",
        "/agents?purpose=manage",
        token=admin_token,
        tenant=admin_tenant,
        org_unit=CFBJ,
    )
    manage_names = agent_names(manage_list)
    check(
        "manage list contains both agents",
        status == 200 and name_a in manage_names and name_b in manage_names,
        f"names={manage_names}",
    )
    check(
        "manage list has no builtins",
        not any(
            (item.get("id") or "").startswith("builtin-")
            or item.get("is_builtin")
            for item in (manage_list.get("data") or [])
        ),
    )

    # Same-unit viewer sees both in chat
    status, viewer_chat = req(
        "GET",
        "/agents?purpose=chat",
        token=viewer_token,
        tenant=viewer_tenant,
        org_unit=CFBJ,
    )
    viewer_names = agent_names(viewer_chat)
    check(
        "same-unit viewer sees both agents",
        status == 200 and name_a in viewer_names and name_b in viewer_names,
        f"names={viewer_names}",
    )

    # Different-unit member does not see them
    status, other_chat = req(
        "GET",
        "/agents?purpose=chat",
        token=other_token,
        tenant=other_tenant,
        org_unit=CF,
    )
    other_names = agent_names(other_chat)
    check(
        "other org unit cannot see agents",
        status == 200 and name_a not in other_names and name_b not in other_names,
        f"names={other_names}",
    )

    # Reject non-builtin model on create
    status, _ = req(
        "POST",
        "/agents",
        token=admin_token,
        tenant=admin_tenant,
        org_unit=CFBJ,
        body={
            "name": f"bad-model-{stamp}",
            "config": {"model_id": "not-a-builtin-model", "agent_mode": "quick-answer"},
        },
    )
    check("reject non-builtin model", status in (400, 422), f"status={status}")

    # Cleanup created agents
    for agent_id in created_ids:
        status, _ = req(
            "DELETE",
            f"/agents/{agent_id}",
            token=admin_token,
            tenant=admin_tenant,
            org_unit=CFBJ,
        )
        check(f"cleanup delete {agent_id[:8]}", status in (200, 204), f"status={status}")

    print(f"\nResult: {PASS} passed, {FAIL} failed")
    return 1 if FAIL else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:  # noqa: BLE001 — top-level e2e reporter
        print(f"ERROR: {exc}", file=sys.stderr)
        sys.exit(2)
