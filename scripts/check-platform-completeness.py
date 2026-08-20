#!/usr/bin/env python3
import pathlib
import re
import sys


ROOT = pathlib.Path(__file__).resolve().parent.parent
PROTO = ROOT / "api/proto/forge/v1/platform.proto"
ADAPTER_ROOT = ROOT / "internal/adapters/kratosapi"
AUTHZ = ROOT / "internal/platform/authz/kratos.go"
ADMIN_PAGES = ROOT / "web/apps/platform-admin/src/pages"
SHELL_PAGES = ROOT / "web/apps/shell/src/pages"

REQUIRED_ADMIN_PAGES = {
    "AccessReviews.tsx",
    "Approvals.tsx",
    "AuditLogs.tsx",
    "ConfigChanges.tsx",
    "DataGovernance.tsx",
    "Departments.tsx",
    "Menus.tsx",
    "Organization.tsx",
    "Permissions.tsx",
    "Positions.tsx",
    "Roles.tsx",
    "Security.tsx",
    "Sessions.tsx",
    "TemporaryGrants.tsx",
    "UserAssignmentsModal.tsx",
    "UserGroups.tsx",
    "Users.tsx",
}
REQUIRED_SHELL_PAGES = {"AccountSecurity.tsx", "ApiTokens.tsx", "Login.tsx"}


def read(path: pathlib.Path) -> str:
    return path.read_text(encoding="utf-8")


def main() -> int:
    errors: list[str] = []
    proto = read(PROTO)
    adapter = "\n".join(read(path) for path in sorted(ADAPTER_ROOT.glob("*.go")))
    authz = read(AUTHZ)
    rpc_names = re.findall(r"\brpc\s+([A-Za-z0-9_]+)\s*\(", proto)
    if not rpc_names:
        errors.append("PlatformService has no RPC declarations")

    for name in rpc_names:
        operation = f"OperationPlatformService{name}"
        if operation not in authz:
            errors.append(f"missing backend authorization rule: {operation}")
        if not re.search(rf"func\s+\(s\s+\*PlatformService\)\s+{re.escape(name)}\s*\(", adapter):
            errors.append(f"missing concrete PlatformService implementation: {name}")

    for page in sorted(REQUIRED_ADMIN_PAGES):
        if not (ADMIN_PAGES / page).is_file():
            errors.append(f"missing platform-admin page: {page}")
    for page in sorted(REQUIRED_SHELL_PAGES):
        if not (SHELL_PAGES / page).is_file():
            errors.append(f"missing shell security page: {page}")

    if errors:
        for error in errors:
            print(f"platform completeness failed: {error}", file=sys.stderr)
        return 1
    print(f"platform completeness OK: {len(rpc_names)} RPCs, {len(REQUIRED_ADMIN_PAGES)} admin pages, {len(REQUIRED_SHELL_PAGES)} shell security pages")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
