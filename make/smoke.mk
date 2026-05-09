.PHONY: smoke-participant smoke-participant-authz smoke-participant-rate-limits \
	smoke-participant-submission-abuse smoke-public-surface-audit \
	smoke-admin-realtime-authz smoke-realtime-health smoke-admin-runtime \
	smoke-sample-challenge-docker smoke-organizer-created smoke-prod-edge \
	smoke-prod-host-enforcement smoke-prod-host-recovery smoke-prod-short-match \
	smoke-prod-db-restore capture-prod-host-baseline capture-go-live-metrics \
	summarize-validation-artifacts render-validation-report check-validation-alerts \
	go-live-check validate-prod-release-candidate render-event-ready-note \
	verify-event-ready-note verify-release-candidate tag-release

smoke-participant:
	./scripts/smoke-participant-flow.sh

smoke-participant-authz:
	./scripts/smoke-participant-authz-boundaries.sh

smoke-participant-rate-limits:
	./scripts/smoke-participant-rate-limits.sh

smoke-participant-submission-abuse:
	./scripts/smoke-participant-submission-abuse.sh

smoke-public-surface-audit:
	./scripts/smoke-public-surface-audit.sh

smoke-admin-realtime-authz:
	./scripts/smoke-admin-realtime-authz.sh

smoke-realtime-health:
	./scripts/smoke-realtime-health.sh

smoke-admin-runtime:
	./scripts/smoke-admin-runtime-flow.sh

smoke-sample-challenge-docker:
	./scripts/smoke-sample-challenge-docker.sh

smoke-organizer-created:
	./scripts/smoke-organizer-created-flow.sh

smoke-prod-edge:
	./scripts/smoke-prod-edge.sh

smoke-prod-host-enforcement:
	./scripts/smoke-prod-host-enforcement.sh

smoke-prod-host-recovery:
	./scripts/smoke-prod-host-recovery.sh

smoke-prod-short-match:
	./scripts/smoke-prod-short-match.sh

smoke-prod-db-restore:
	./scripts/smoke-prod-db-restore.sh

capture-prod-host-baseline:
	./scripts/capture-prod-host-baseline.sh

capture-go-live-metrics:
	./scripts/capture-go-live-metrics.sh

summarize-validation-artifacts:
	./scripts/summarize-validation-artifacts.sh

render-validation-report:
	./scripts/render-validation-report.sh

check-validation-alerts:
	./scripts/check-validation-alerts.sh

go-live-check:
	./scripts/go-live-check.sh

validate-prod-release-candidate:
	./scripts/validate-prod-release-candidate.sh

render-event-ready-note:
	./scripts/render-event-ready-note.sh

verify-event-ready-note:
	./scripts/verify-event-ready-note.sh

verify-release-candidate:
	./scripts/verify-release-candidate.sh

tag-release:
	./scripts/tag-release.sh
