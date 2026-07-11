# ADTickPlatform — Product & Engineering Findings

**Date:** 2026-07-11  
**Scope:** Feature gaps, polish opportunities, and prioritization for ADTickPlatform  
**Out of scope (explicit):** team-bot / automation WireGuard peer (not needed)  
**Implementation status:** Core P0–P3 product items landed in-tree (2026-07-11), then review fixes applied. **Jury score adjust was removed** — organizers will handle post-event scoring manually. Full admin dashboard monolith split remains deferred.

---

## 1. Executive summary

ADTickPlatform is **event-capable** for a well-run single-host Attack-Defense tick CTF. Core gameplay, scoring, runtime isolation, WireGuard access, unlock/SSH, realtime UI, and ops validation are in place.

Remaining work is not “make it run an AD CTF.” It is:

1. **Reliability / operator trust** under live pressure  
2. **Participant day-of friction** (human UX on top of a strong API)  
3. **Organizer / jury product surface**  
4. **Codebase and docs maintainability**

---

## 2. Current strengths (baseline)

| Area | Evidence |
|------|----------|
| Match lifecycle | Start / stop / pause / resume, scoreboard freeze, scheduler, pre-match warmup |
| Scoring | Faust attack / defense / SLA; scoring audit after ticks and manual advance |
| Runtime | Per `team × service` Docker isolation; unlock → SSH; factory reset / restart |
| Network access | Per-player WireGuard; host access policy; trusted reconcile path |
| Realtime UX | Scoreboard SSE, attack map / globe, attack feed, presentation controls |
| Ops maturity | Smokes, Grafana/Prometheus, backup/restore drills, release-candidate gates |
| Contracts | Versioned `/api/v2`, OpenAPI, admin API, challenge runtime contract + samples |
| Design system | Documented tactical workroom language (`DESIGN.md`, `PRODUCT.md`); theme tokens in place |

Ops polish tracked in `docs/ops-polish-todo.md` is complete. Trusted reconcile is implemented (`controllerTrustedReconciler`, runbook, host smoke).

---

## 3. Findings by category

### 3.1 Reliability & runtime trust

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| R1 | Architecture describes multi **runtime workers** and placement; production path is effectively **single-host Docker** | High (scale) | Ceiling for large team × service counts |
| R2 | Separate **maintenance subnet** for SSH is documented as future hardening only | Medium | SSH shares service IP with attack surface after unlock |
| R3 | SSH credentials are **stable** root passwords; no ephemeral/rotating creds | Low–Medium | Acceptable for many events; weaker audit/rotation story |
| R4 | Operator may still juggle multiple status/reconcile surfaces | Medium | Prefer one “trusted state” panel: runtime + access + WG + unlock SSH |
| R5 | Host-real smokes and runbooks exist; value is **discipline on event host**, not missing scripts | Medium | Operational, not a code gap |

### 3.2 Participant experience

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| P1 | **No in-browser flag submit UI** | High | `POST /api/v2/submit` exists; humans must use API/docs |
| P2 | **No password change / forgot-password** | Medium | Profile update exists; password lifecycle is organizer/script-driven |
| P3 | **No first-hour onboarding checklist** | Medium | Flow is WG → unlock → SSH → patch; UI is dense control center |
| P4 | Limited **tick-by-tick SLA / checker history** for players | Medium | `slaMessage` on services; organizers get richer checker history |
| P5 | Copy/share affordances for SSH connection details can be clearer | Low | One-click copy of connection string / password |
| P6 | Match paused / frozen messaging exists; “what still works” can be clearer | Low | Stress UX polish |

**Explicit non-finding:** team-bot WireGuard peer is **not required**. Members reuse normal player peers for automation.

### 3.3 Organizer / jury surface

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| O1 | No **manual score adjust / bonus / jury penalty** | Medium | Deactivate team/player exists; no fine-grained jury scoring |
| O2 | **Bulk team/player import** only via scripts (`create-teams-from-csv.py`) | Medium | Not productized in admin UI |
| O3 | No match **announcements / bulletin** to participants | Medium | Common day-of ops channel |
| O4 | No first-class **scoreboard / attack export** (CSV/JSON) | Medium | Harder post-event archive and external displays |
| O5 | **Factory-reset cooldown** is an open policy choice, not a clear product control | Low | Listed in `docs/game-rules.md` open choices |
| O6 | Single organizer auth model; no **jury vs infra vs read-only** roles | Low–Medium | Fine for small staff; weak for large orgs |

### 3.4 Scale & architecture honesty

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| S1 | Multi-worker topology is aspirational relative to current controller model | High (if scaling) | Document single-host limits or invest in placement |
| S2 | Checker path is queue-oriented; horizontal runtime placement is not | Medium | Checkers can scale more easily than container hosts |

### 3.5 Challenge author experience

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| C1 | Validate/deploy + samples are solid; no polished **package import wizard** | Low–Medium | Manifest + images still form/image-ref oriented |
| C2 | Checker validation failures mid-event could surface richer diagnostics | Low | Ops under stress |

### 3.6 Code maintainability

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| E1 | Admin UI is a **monolith**: `organizer-dashboard-sections.tsx` ~5.9k LOC; `use-organizer-dashboard.ts` ~2.6k LOC | High (dev risk) | Hotfix risk during events |
| E2 | Participant control center is large but more modular than admin | Low | Acceptable for now |

### 3.7 Docs & packaging polish

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| D1 | README doc links use broken `file:///home/jergal/ADTickPlatform/...` paths | Medium | Breaks external readers / GitHub |
| D2 | README claims **MIT**; **no `LICENSE` file** | Medium | Legal/clarity gap |
| D3 | `docs/implementation-plan.md` status lags reality (still “in progress” language) | Low | Misleads contributors |
| D4 | Scoring docs mix **English / Indonesian** | Low | Prefer one operator language or explicit locales |
| D5 | Architecture still mentions optional team-bot | Info | Keep as optional design note or trim; not a product hole |

### 3.8 Design system polish

| ID | Finding | Severity | Notes |
|----|---------|----------|-------|
| U1 | Tokens/theme align with tactical workroom direction | Positive | IBM Plex, warm paper / umber, hard borders |
| U2 | Admin density vs participant density can diverge under stress | Low | Split admin pages help both UX and code |
| U3 | Presentation / projector modes for scoreboard can be stronger | Low | Attack map presentation controls already exist |
| U4 | Reduced-motion should stay enforced on map/SFX motion | Low | Already in design principles |

---

## 4. Explicit non-goals (confirmed)

| Item | Decision |
|------|----------|
| Team-bot / automation WireGuard peer | **Not needed** — normal player peers are enough |
| Scoring formula redesign | Do not prioritize; Faust model + audit path exist |
| Attack-map visual spectacle | Already a showcase; low ROI |
| Full Kubernetes / multi-cloud rewrite | Premature until single-host ops are boringly reliable |

---

## 5. Prioritized backlog

### P0 — Event reliability / trust

1. Single operator **trusted state** view (runtime + access + WG + unlocked SSH)  
2. Run host-real smokes on the **actual event host** as a discipline gate  
3. Document **single-host capacity limits** honestly (or start multi-host only if scale demands it)

### P1 — Participant day-of friction

4. In-UI **flag submit** (calls existing submit API)  
5. First-login **onboarding checklist** (WG → unlock → SSH)  
6. **Password change** for participants  
7. Clearer **SLA / service history** for players  

### P2 — Organizer event ops

8. **Bulk CSV import** in admin UI  
9. **Score / attack export** (CSV or JSON)  
10. Optional **jury manual score adjust**  
11. Match **announcements**  

### P3 — Engineering & packaging health

12. **Split admin monolith** by tab / domain  
13. Fix **README links**, add **LICENSE**, refresh implementation-plan status  
14. Challenge packaging author UX improvements  

---

## 6. What not to prioritize next

- More globe/map theatrical effects  
- Scoring model churn without a fairness bug  
- Generic UI restyles without operator workflow gains  
- Team-bot peer implementation  
- Multi-host runtime before single-host failure modes are fully operationalized  

---

## 7. Success criteria (how to know the next slice worked)

| Lane | Done when |
|------|-----------|
| Reliability | Operator can answer “is host access truth good?” from one surface; trusted reconcile smoke green on event host |
| Participant | New player completes WG + unlock + first flag path without reading OpenAPI |
| Organizer | Teams importable without shell scripts; post-match scores exportable in one action |
| Engineering | Admin tabs editable in isolation; README works from GitHub clone |

---

## 8. Related docs

- `PRODUCT.md` — product UX principles  
- `DESIGN.md` — design system  
- `docs/architecture.md` — system design (includes future options)  
- `docs/game-rules.md` — rules + open policy choices  
- `docs/ops-polish-todo.md` — completed ops polish  
- `docs/trusted-reconcile-runbook.md` — reconcile operator path  
- `.omx/plans/next-reliability-lane-deployment-reconcile.md` — reliability lane design  

---

## 9. Finding index (quick scan)

| ID | One-liner |
|----|-----------|
| R1 | Single-host reality vs multi-worker docs |
| R2 | Maintenance subnet still future |
| R3 | Stable SSH passwords only |
| R4 | Fragmented operator truth surfaces |
| R5 | Event-host smoke discipline |
| P1 | No browser flag submit |
| P2 | No password change/reset |
| P3 | No onboarding checklist |
| P4 | Thin player SLA history |
| P5 | SSH copy affordances |
| P6 | Pause/freeze “what works” copy |
| O1 | No jury score adjust |
| O2 | CSV import is script-only |
| O3 | No announcements |
| O4 | No score/attack export |
| O5 | Reset cooldown not productized |
| O6 | No multi-role admin |
| S1 | Scale ceiling is single host |
| S2 | Checkers scale easier than runtimes |
| C1 | No challenge package wizard |
| C2 | Checker fail diagnostics |
| E1 | Admin UI monolith |
| E2 | Participant UI large but OK |
| D1 | Broken README `file://` links |
| D2 | Missing LICENSE file |
| D3 | Stale implementation-plan status |
| D4 | EN/ID docs mix |
| D5 | Team-bot docs noise only |
| U1–U4 | Design consistency polish |

---

*Team-bot peer intentionally excluded from the backlog by product decision (2026-07-11).*
