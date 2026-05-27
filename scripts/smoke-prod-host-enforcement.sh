#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"
source "${ROOT_DIR}/scripts/lib/common.sh"

require_bin curl
require_bin jq
require_bin wg
require_bin nft
require_bin iptables
require_bin openssl
if [[ "${EUID}" -ne 0 ]]; then
  require_bin sudo
fi

PROD_ENV="${PROD_ENV:-deploy/compose/prod.env}"
load_env_file "${PROD_ENV}"

EDGE_BASE_URL="${PROD_EDGE_BASE_URL:-$(derive_edge_base_url)}"
ADMIN_TOKEN="$(resolve_admin_api_token "${ROOT_DIR}/.runtime/backend-stack.env")"
EMAIL="${AD_PLATFORM_EMAIL:-alpha.captain@example.com}"
PASSWORD="${AD_PLATFORM_PASSWORD:-alpha-secret}"
TEAM_ID="${AD_PLATFORM_TEAM_ID:-}"
WIREGUARD_INTERFACE="${WIREGUARD_GATEWAY_INTERFACE:-wg0}"
WIREGUARD_FIREWALL_TABLE="${WIREGUARD_GATEWAY_FIREWALL_TABLE:-adplatform_wireguard}"
ACCESS_FIREWALL_TABLE="${CONTROLLER_ACCESS_FIREWALL_TABLE:-adplatform_service_access}"
ACCESS_FIREWALL_BACKEND="${CONTROLLER_ACCESS_FIREWALL_BACKEND:-iptables}"
UNLOCK_SECRET="${UNLOCK_PROOF_SECRET:-}"

if [[ -z "${UNLOCK_SECRET}" ]]; then
  echo "UNLOCK_PROOF_SECRET must be set in ${PROD_ENV} for host enforcement smoke." >&2
  exit 1
fi

admin_get() {
  local path="$1"
  curl_json "admin GET ${path}" -H "Authorization: Bearer ${ADMIN_TOKEN}" "${EDGE_BASE_URL}${path}"
}

admin_post() {
  local path="$1"
  curl_json "admin POST ${path}" -X POST -H "Authorization: Bearer ${ADMIN_TOKEN}" "${EDGE_BASE_URL}${path}"
}

curl_json() {
  local label="$1"
  shift

  local response_file
  response_file="$(mktemp)"

  local status
  status="$(curl -sS -o "${response_file}" -w '%{http_code}' "$@")"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "${label} failed (status=${status})." >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    exit 1
  fi

  cat "${response_file}"
  rm -f "${response_file}"
}

participant_post() {
  local path="$1"
  local token="$2"
  local body="${3:-}"
  if [[ -n "${body}" ]]; then
    curl_json "participant POST ${path}" -X POST \
      -H "Authorization: Bearer ${token}" \
      -H 'Content-Type: application/json' \
      -d "${body}" \
      "${EDGE_BASE_URL}${path}"
    return 0
  fi

  curl_json "participant POST ${path}" -X POST -H "Authorization: Bearer ${token}" "${EDGE_BASE_URL}${path}"
}

participant_get() {
  local path="$1"
  local token="$2"
  curl_json "participant GET ${path}" -H "Authorization: Bearer ${token}" "${EDGE_BASE_URL}${path}"
}

authenticate_participant() {
  local response_file status
  response_file="$(mktemp)"
  status="$(
    curl -sS -o "${response_file}" -w '%{http_code}' -X POST "${EDGE_BASE_URL}/api/v2/authenticate" \
      -H 'Content-Type: application/json' \
      -d "$(jq -nc --arg email "${EMAIL}" --arg password "${PASSWORD}" '{email:$email,password:$password}')"
  )"
  if [[ "${status}" -lt 200 || "${status}" -ge 300 ]]; then
    echo "participant authenticate failed (status=${status})." >&2
    cat "${response_file}" >&2
    rm -f "${response_file}"
    cat >&2 <<EOF
Set AD_PLATFORM_EMAIL and AD_PLATFORM_PASSWORD in ${PROD_ENV} to a real participant account on this host stack.
The default alpha credentials only work on seeded/demo data and will fail when API_GATEWAY_AUTO_SEED=false.
EOF
    exit 1
  fi

  jq -er '.token' < "${response_file}"
  rm -f "${response_file}"
}

assert_nft_table_exists() {
  local table_name="$1"
  host_net_cmd nft list table inet "${table_name}" >/dev/null
}

host_net_cmd() {
  run_as_root_noninteractive "$@"
}

strip_cidr() {
  local value="$1"
  printf '%s' "${value%%/*}"
}

echo "production host enforcement smoke: base_url=${EDGE_BASE_URL}"

echo "wireguard status:"
wg_status="$(admin_get /api/v2/admin/wireguard/status)"
echo "${wg_status}" | jq -c '.'
wg_mode="$(echo "${wg_status}" | jq -r '.mode')"
if [[ "${wg_mode}" != "host" ]]; then
  echo "wireguard gateway is not running in host mode: ${wg_mode}" >&2
  exit 1
fi

echo "controller access status:"
access_status="$(admin_get /api/v2/admin/access/status)"
echo "${access_status}" | jq -c '.'
access_mode="$(echo "${access_status}" | jq -r '.mode')"
if [[ "${access_mode}" != "host" ]]; then
  echo "controller access is not running in host mode: ${access_mode}" >&2
  exit 1
fi

echo "participant authenticate:"
participant_token="$(authenticate_participant)"
printf '  token_prefix=%s...\n' "${participant_token:0:16}"

team_services="$(participant_get /api/v2/team/services "${participant_token}")"
challenge_id="$(printf '%s\n' "${team_services}" | jq -er '.[0].challenge_id')"
derived_team_id="$(printf '%s\n' "${team_services}" | jq -er '.[0].team_id')"
if [[ -n "${TEAM_ID}" && "${TEAM_ID}" != "${derived_team_id}" ]]; then
  cat >&2 <<EOF
AD_PLATFORM_TEAM_ID=${TEAM_ID} does not match the authenticated participant team_id=${derived_team_id}.
Remove AD_PLATFORM_TEAM_ID from ${PROD_ENV} or set it to the authenticated team.
EOF
  exit 1
fi
TEAM_ID="${derived_team_id}"
endpoint="$(printf '%s\n' "${team_services}" | jq -er '.[0].endpoint')"
service_host="${endpoint%:*}"
service_port="${endpoint##*:}"

unlock_digest="$(
  printf '%s:%s' "${challenge_id}" "${TEAM_ID}" |
    openssl dgst -sha256 -hmac "${UNLOCK_SECRET}" -hex |
    awk '{print $2}'
)"
unlock_proof="ADU1.${challenge_id}.${TEAM_ID}.${unlock_digest}"

echo "participant unlock:"
participant_post "/api/v2/services/${challenge_id}/unlock" "${participant_token}" "$(jq -nc --arg proof "${unlock_proof}" '{proof:$proof}')" |
  jq -c '{challenge_id,team_id,unlocked}'

echo "trusted deployment reconcile after unlock:"
trusted_reconcile="$(admin_post /api/v2/admin/deployments/reconcile)"
echo "${trusted_reconcile}" | jq -c '{processed_jobs,processed_instances,completed_jobs}'

echo "host wireguard state:"
wg_allowed_ips="$(host_net_cmd wg show "${WIREGUARD_INTERFACE}" allowed-ips)"
printf '%s\n' "${wg_allowed_ips}"

active_team_peer_addresses="$(
  admin_get /api/v2/admin/players |
    jq -r --argjson team_id "${TEAM_ID}" '.[] | select(.team_id == $team_id and .wireguard_status != "revoked") | .wireguard_address'
)"
if [[ -z "${active_team_peer_addresses}" ]]; then
  echo "no active WireGuard peer addresses found for team ${TEAM_ID}" >&2
  exit 1
fi

unauthorized_peer_address="$(
  admin_get /api/v2/admin/players |
    jq -r --argjson team_id "${TEAM_ID}" 'first(.[] | select(.team_id != $team_id and .role != "organizer" and .wireguard_status != "revoked") | .wireguard_address) // empty'
)"
if [[ -z "${unauthorized_peer_address}" ]]; then
  echo "no active non-organizer unauthorized WireGuard peer address found outside team ${TEAM_ID}" >&2
  exit 1
fi
unauthorized_peer_ip="$(strip_cidr "${unauthorized_peer_address}")"

while IFS= read -r peer_address; do
  [[ -n "${peer_address}" ]] || continue
  peer_ip="$(strip_cidr "${peer_address}")"
  if ! printf '%s\n' "${wg_allowed_ips}" | grep -Fq "${peer_ip}/32"; then
    echo "wireguard interface ${WIREGUARD_INTERFACE} is missing allowed ip ${peer_ip}/32" >&2
    exit 1
  fi
done <<< "${active_team_peer_addresses}"

echo "wireguard nftables table:"
assert_nft_table_exists "${WIREGUARD_FIREWALL_TABLE}"
wireguard_rules="$(host_net_cmd nft list table inet "${WIREGUARD_FIREWALL_TABLE}")"
printf '%s\n' "${wireguard_rules}"
while IFS= read -r peer_address; do
  [[ -n "${peer_address}" ]] || continue
  peer_ip="$(strip_cidr "${peer_address}")"
  if ! printf '%s\n' "${wireguard_rules}" | grep -Fq "${peer_ip}"; then
    echo "wireguard nft table ${WIREGUARD_FIREWALL_TABLE} is missing peer ${peer_ip}" >&2
    exit 1
  fi
done <<< "${active_team_peer_addresses}"

if [[ "${ACCESS_FIREWALL_BACKEND}" == "nftables" ]]; then
  echo "controller access nftables table:"
  assert_nft_table_exists "${ACCESS_FIREWALL_TABLE}"
  access_rules="$(host_net_cmd nft list table inet "${ACCESS_FIREWALL_TABLE}")"
  printf '%s\n' "${access_rules}"
  if ! printf '%s\n' "${access_rules}" | grep -Fq "ip daddr ${service_host} tcp dport ${service_port} accept"; then
    echo "controller access nft table ${ACCESS_FIREWALL_TABLE} is missing the public service accept rule for ${endpoint}" >&2
    exit 1
  fi
  if ! printf '%s\n' "${access_rules}" | grep -Fq "ip daddr ${service_host} tcp dport 22 drop"; then
    echo "controller access nft table ${ACCESS_FIREWALL_TABLE} is missing the ssh default-drop rule for ${service_host}" >&2
    exit 1
  fi
  while IFS= read -r peer_address; do
    [[ -n "${peer_address}" ]] || continue
    peer_ip="$(strip_cidr "${peer_address}")"
    if ! printf '%s\n' "${access_rules}" | grep -Fq "${peer_ip}"; then
      echo "controller access nft table ${ACCESS_FIREWALL_TABLE} is missing allowed peer ${peer_ip} after unlock" >&2
      exit 1
    fi
  done <<< "${active_team_peer_addresses}"
  if printf '%s\n' "${access_rules}" | grep -Fq "${unauthorized_peer_ip}"; then
    echo "controller access nft table ${ACCESS_FIREWALL_TABLE} unexpectedly allows unauthorized peer ${unauthorized_peer_ip}" >&2
    exit 1
  fi
else
  echo "controller access iptables chain:"
  host_forward_rules="$(host_net_cmd iptables -S FORWARD)"
  host_docker_user_rules="$(host_net_cmd iptables -S DOCKER-USER)"
  host_access_rules="$(host_net_cmd iptables -S ADPLATFORM-WG-SERVICES)"
  host_raw_rules="$(host_net_cmd iptables -t raw -S ADPLATFORM-WG-RAW 2>/dev/null || true)"
  printf '%s\n%s\n%s\n' "${host_forward_rules}" "${host_docker_user_rules}" "${host_access_rules}"
  if ! printf '%s\n' "${host_docker_user_rules}" | grep -Fq -- "-j ADPLATFORM-WG-SERVICES"; then
    echo "controller access iptables is missing the DOCKER-USER jump to ADPLATFORM-WG-SERVICES" >&2
    exit 1
  fi
  if ! printf '%s\n' "${host_access_rules}" | grep -Fq -- "-d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport ${service_port} -j ACCEPT"; then
    echo "controller access iptables is missing the public service accept rule for ${endpoint}" >&2
    exit 1
  fi
  if [[ -n "${host_raw_rules}" ]]; then
    printf '%s\n' "${host_raw_rules}"
    if ! printf '%s\n' "${host_raw_rules}" | grep -Fq -- "-A ADPLATFORM-WG-RAW -d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -j ACCEPT" &&
      ! printf '%s\n' "${host_raw_rules}" | grep -Fq -- "-A ADPLATFORM-WG-RAW -i ${WIREGUARD_INTERFACE} -d ${service_host}/32 -j ACCEPT"; then
      echo "controller raw iptables chain exists but is missing the wg ingress bypass rule for ${service_host}" >&2
      exit 1
    fi
  else
    echo "controller raw iptables chain not present; continuing with filter-table enforcement checks only"
  fi
  if ! printf '%s\n' "${host_access_rules}" | grep -Fq -- "-d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport 22 -j DROP"; then
    echo "controller access iptables is missing the ssh default-drop rule for ${service_host}" >&2
    exit 1
  fi
  while IFS= read -r peer_address; do
    [[ -n "${peer_address}" ]] || continue
    peer_ip="$(strip_cidr "${peer_address}")"
    if ! printf '%s\n' "${host_access_rules}" | grep -Fq -- "-s ${peer_ip}/32 -d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport 22 -j ACCEPT"; then
      echo "controller access iptables is missing allowed peer ${peer_ip} after unlock" >&2
      exit 1
    fi
  done <<< "${active_team_peer_addresses}"
  if printf '%s\n' "${host_access_rules}" | grep -Fq -- "-s ${unauthorized_peer_ip}/32 -d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport 22 -j ACCEPT"; then
    echo "controller access iptables unexpectedly allows unauthorized peer ${unauthorized_peer_ip}" >&2
    exit 1
  fi
fi

drift_peer_address="$(printf '%s\n' "${active_team_peer_addresses}" | head -n 1)"
drift_peer_ip="$(strip_cidr "${drift_peer_address}")"

echo "inject drift by deleting one authorized ssh allow rule:"
host_net_cmd iptables -C ADPLATFORM-WG-SERVICES -s "${drift_peer_ip}/32" -d "${service_host}/32" -i "${WIREGUARD_INTERFACE}" -p tcp --dport 22 -j ACCEPT
host_net_cmd iptables -D ADPLATFORM-WG-SERVICES -s "${drift_peer_ip}/32" -d "${service_host}/32" -i "${WIREGUARD_INTERFACE}" -p tcp --dport 22 -j ACCEPT
if host_net_cmd iptables -C ADPLATFORM-WG-SERVICES -s "${drift_peer_ip}/32" -d "${service_host}/32" -i "${WIREGUARD_INTERFACE}" -p tcp --dport 22 -j ACCEPT >/dev/null 2>&1; then
  echo "expected drift injection to remove ssh allow rule for ${drift_peer_ip}" >&2
  exit 1
fi
printf '  deleted_rule=iptables -D ADPLATFORM-WG-SERVICES -s %s/32 -d %s/32 -i %s -p tcp --dport 22 -j ACCEPT\n' "${drift_peer_ip}" "${service_host}" "${WIREGUARD_INTERFACE}"

echo "trusted deployment reconcile after drift:"
trusted_reconcile="$(admin_post /api/v2/admin/deployments/reconcile)"
echo "${trusted_reconcile}" | jq -c '{processed_jobs,processed_instances,completed_jobs}'

echo "post-reconcile controller access iptables chain:"
post_reconcile_access_rules="$(host_net_cmd iptables -S ADPLATFORM-WG-SERVICES)"
printf '%s\n' "${post_reconcile_access_rules}"
if ! printf '%s\n' "${post_reconcile_access_rules}" | grep -Fq -- "-s ${drift_peer_ip}/32 -d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport 22 -j ACCEPT"; then
  echo "trusted reconcile did not restore ssh allow rule for ${drift_peer_ip}" >&2
  exit 1
fi
if printf '%s\n' "${post_reconcile_access_rules}" | grep -Fq -- "-s ${unauthorized_peer_ip}/32 -d ${service_host}/32 -i ${WIREGUARD_INTERFACE} -p tcp -m tcp --dport 22 -j ACCEPT"; then
  echo "trusted reconcile incorrectly allowed unauthorized peer ${unauthorized_peer_ip}" >&2
  exit 1
fi

echo "host enforcement smoke passed:"
printf '  interface=%s challenge_id=%s service=%s wg_table=%s access_table=%s\n' \
  "${WIREGUARD_INTERFACE}" "${challenge_id}" "${endpoint}" "${WIREGUARD_FIREWALL_TABLE}" "${ACCESS_FIREWALL_TABLE}"
