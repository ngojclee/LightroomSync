# Wails UI Cutover Plan (GUI thật)

> Scope: Chuyển `LightroomSyncUI.exe` từ temporary Windows Forms harness sang Wails UI thực thụ.
>  
> Status: In Progress (planning complete, implementation pending)

## Goal

- Ship GUI thật bằng Wails nhưng không phá luồng automation/testing hiện tại dùng `--action`.
- Reuse tối đa IPC contract đã ổn định ở Agent.
- Giữ kiến trúc: UI không đụng SMB trực tiếp.

## Current Gap Summary

- Backend IPC + business flow đã sẵn sàng; thiếu lớp Wails runtime và frontend thật.
- `cmd/ui/main.go` đang chứa cả CLI actions lẫn temporary Windows Forms harness nên cần tách để tái sử dụng.
- `frontend/` hiện chưa có code, nên milestone M3/M4 là phần khối lượng chính.
- Wave-level specs now exist for Wave 1 and Wave 2, with dependency/timeline map for execution pacing.
- Wave 3 frontend shell contract and tab-command mapping are now explicitly documented for implementation alignment.
- Wave 1 scaffold implementation is in place (runtime switch + frontend scaffold), with remaining unblock on Wails CLI preflight/module availability.

## Execution Order (6 Waves)

### Wave 1 (M1) — Bootstrap Wails Runtime

Deliverables:
- `wails.json` + frontend package scaffold.
- Backend binding entrypoint cho Wails app.
- Không làm gãy mode CLI `--action`.

Gate:
- `LightroomSyncUI.exe --action ping` vẫn chạy bình thường.
- Wails dev runtime mở được cửa sổ shell trống.

### Wave 2 (M2) — Extract Shared UI API

Deliverables:
- Shared package `internal/uiapi` chứa command wrappers hiện tại.
- `cmd/ui` chỉ còn orchestration (CLI mode + app mode), không chứa logic IPC chi tiết.

Gate:
- Unit tests map command/code pass.
- CLI actions cũ trả kết quả JSON tương đương trước refactor.

### Wave 3 (M3) — Frontend Shell

Deliverables:
- Layout + tabs: Status/Settings/Backups/Logs/Update.
- Shared notification/error banner.

Gate:
- Tất cả tab render không crash khi agent offline.

### Wave 4 (M4) — Data Flow & UX Reliability

Deliverables:
- Polling status/logs + cursor management.
- In-flight protection cho mutation actions (save/sync/update).

Gate:
- Không duplicate request khi user bấm nhanh nhiều lần.
- UI giữ responsive khi Agent tạm offline rồi online lại.

### Wave 5 (M5) — Build/Installer Integration

Deliverables:
- Build pipeline output Wails UI binary chính thức.
- Flag fallback để build harness trong giai đoạn chuyển tiếp.

Gate:
- Metadata/version injection vẫn đúng `x.y.z.k`.
- Installer đóng gói đúng artifact theo mode.

### Wave 6 (M6) — Validation & Default Switch

Deliverables:
- Smoke script riêng cho Wails UI.
- Full manual matrix với Wails UI.
- Chuyển mặc định `LightroomSyncUI.exe` sang Wails runtime.

Gate:
- Pass toàn bộ checklist Phase 8.3 liên quan UI/tray/installer.

## Milestones

### M1. Runtime Bootstrap

- Add Wails app skeleton + config files.
- Define backend service object for frontend bindings.
- Keep CLI action mode for test scripts.

### M2. Backend Bridge Refactor

- Move action handlers from `cmd/ui/main.go` into shared package (`internal/uiapi`).
- Add typed request/response models for frontend bindings.
- Preserve existing error codes (`ok`, `bad_request`, `agent_offline`, ...).

### M3. Frontend UI Shell

- Build tabbed shell: Status, Settings, Backups, Logs, Update.
- Add connection indicator and global error toast/banner.
- Add loading skeletons for first render.

### M4. Data Flow

- Poll status on interval with jitter-safe timer.
- Poll logs using cursor (`after_id`).
- Wire settings save + sync commands with optimistic/disable-state UX.

### M5. Build Pipeline Integration

- Add Wails build target in scripts/Makefile.
- Keep harness build path as fallback.
- Validate version injection behavior still matches `x.y.z.k`.

### M6. Validation & Cutover

- Add Wails smoke script.
- Run manual E2E matrix for tray open/focus, backup sync, update download, installer upgrade.
- Switch default `LightroomSyncUI.exe` to Wails after signoff.

## File Ownership Map

- `cmd/ui/main.go`: giữ mode CLI + startup mode switch (harness/wails) trong giai đoạn chuyển tiếp.
- `internal/uiapi/*`: shared action handlers + typed models cho CLI/Wails backend.
- `frontend/*`: Wails frontend source (tabs, state, actions, error UI).
- `scripts/build_windows.ps1`: thêm Wails build path + fallback harness mode.
- `scripts/e2e_tray_ui_smoke.ps1` hoặc script mới Wails: smoke automation evidence.
- `installer/*`: đảm bảo artifact đúng cho installer/release.

## Quality Gates (must pass before default switch)

1. Functional parity:
   - All existing IPC flows from harness are available from Wails UI.
2. Stability:
   - No UI freeze when network share latency/disconnect events happen (UI remains local IPC only).
3. Regression:
   - Existing automation (`--action`, snapshot/latency/compare scripts) still works.
4. Packaging:
   - Release and installer artifacts reproducible, versioned, and signable.

## Exit Criteria

- Wails UI opens consistently from tray on Windows 10/11.
- All current IPC commands reachable via Wails backend bindings.
- Existing Phase 8.3 scripts still pass (or have Wails equivalents with same evidence quality).

## Related Planning Docs

- [Wave 1 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave1-bootstrap-spec.md)
- [Wave 2 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave2-uiapi-refactor-spec.md)
- [Wave 3 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave3-frontend-shell-spec.md)
- [UI Command Contract Map](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/ui-command-contract-map.md)
- [Timeline & Dependencies](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/timeline-and-dependencies.md)
