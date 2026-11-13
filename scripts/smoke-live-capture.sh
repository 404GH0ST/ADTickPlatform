#!/usr/bin/env bash
# Builds production artifacts, then exercises only disposable challenge resources.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
CONTROLLER_CAPTURE_MODE=guarded "${ROOT_DIR}/scripts/preflight-capture-host.sh" /dev/null
prefix="adcap-test-$(python3 -c 'import uuid;print(uuid.uuid4().hex[:10])')"
work="$(mktemp -d /tmp/adplatform-capture-test.XXXXXX)"
cleanup() {
  docker rm -f "${prefix}-control" "${prefix}-service" "${prefix}-bad-service" >/dev/null 2>&1 || true
  docker volume rm "${prefix}-bootstrap" "${prefix}-state" "${prefix}-empty-bootstrap" "${prefix}-bad-state" >/dev/null 2>&1 || true
  while IFS= read -r network; do
    [[ "${network}" == "${prefix}"* ]] && docker network rm "${network}" >/dev/null 2>&1 || true
  done < <(docker network ls --format '{{.Name}}')
  docker image rm "${prefix}-controller" "${prefix}-bootstrap" "${prefix}-sample" >/dev/null 2>&1 || true
  rm -rf "${work}"
}
trap cleanup EXIT
# Choose an unused /24; never reuse or alter a live game network.
docker network inspect $(docker network ls -q) >"${work}/networks.json"
test_ip="$(python3 - "${work}/networks.json" <<'PY'
import ipaddress,json,sys
used=[ipaddress.ip_network(c['Subnet']) for n in json.load(open(sys.argv[1])) for c in (n['IPAM']['Config'] or []) if c.get('Subnet')]
for i in range(240,100,-1):
    candidate=ipaddress.ip_network(f'10.253.{i}.0/24')
    if not any(candidate.overlaps(n) for n in used if n.version==4):
        print(f'10.253.{i}.10');break
else: raise SystemExit('no disposable capture subnet available')
PY
)"
docker build -q -f deploy/docker/controller-service.Dockerfile -t "${prefix}-controller" .
docker build -q -f deploy/docker/capture-bootstrap.Dockerfile -t "${prefix}-bootstrap" .
docker build -q -t "${prefix}-sample" examples/sample-http-challenge/service
# Seed an image-controlled fake gate and healthcheck. Neither may execute,
# including when an operator accidentally supplies an empty bootstrap volume.
docker build -q -t "${prefix}-sample" - <<DOCKERFILE
FROM ${prefix}-sample
RUN mkdir -p /opt/adplatform-capture && printf '#!/bin/sh\\necho unsafe > /opt/ad/state/unsafe-start\\nexec sleep 60\\n' > /opt/adplatform-capture/start && chmod 755 /opt/adplatform-capture/start
HEALTHCHECK --interval=1s CMD echo unsafe > /opt/ad/state/unsafe-healthcheck
DOCKERFILE
CGO_ENABLED=0 go test -c -o "${work}/controller.test" ./services/controller-service
docker volume create --label adplatform.capture-bootstrap=true "${prefix}-bootstrap" >/dev/null
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  --mount "type=volume,src=${prefix}-bootstrap,dst=/capture" "${prefix}-bootstrap"
docker run --rm --name "${prefix}-control" --network host --cgroupns host \
  --cap-drop ALL --cap-add BPF --cap-add PERFMON --cap-add SYS_ADMIN --cap-add NET_ADMIN \
  --security-opt no-new-privileges:true \
  --mount type=bind,src=/sys/fs/cgroup,dst=/sys/fs/cgroup \
  --mount type=bind,src=/proc,dst=/host/proc,readonly \
  --mount type=bind,src=/var/run/docker/netns,dst=/host/docker-netns,readonly,bind-propagation=rslave \
  --mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
  --mount "type=bind,src=${work}/controller.test,dst=/controller.test,readonly" \
  -e CAPTURE_LIFECYCLE_TEST=1 -e "CAPTURE_TEST_PREFIX=${prefix}" \
  -e "CAPTURE_TEST_IMAGE=${prefix}-sample" -e "CAPTURE_TEST_IP=${test_ip}" \
  --entrypoint /controller.test "${prefix}-controller" \
  -test.run '^TestCaptureDockerLifecycle$' -test.v -test.timeout 3m
