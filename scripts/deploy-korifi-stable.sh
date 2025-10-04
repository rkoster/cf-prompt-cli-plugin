#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/scripts"
TEMPLATES_DIR="${ROOT_DIR}/templates"
KORIFI_VERSION="0.16.0"

CLUSTER_NAME=""

function usage_text() {
  cat <<EOF
Usage:
  $(basename "$0") <kind cluster name>

flags:
  -v, --verbose
      Verbose output (bash -x).
EOF
  exit 1
}

function parse_cmdline_args() {
  while [[ $# -gt 0 ]]; do
    i=$1
    case $i in
      -v | --verbose)
        set -x
        shift
        ;;
      -h | --help | help)
        usage_text >&2
        exit 0
        ;;
      *)
        if [[ -n "$CLUSTER_NAME" ]]; then
          echo -e "Error: Unexpected argument: ${i/=*/}\n" >&2
          usage_text >&2
          exit 1
        fi
        CLUSTER_NAME=$1
        shift
        ;;
    esac
  done

  if [[ -z "$CLUSTER_NAME" ]]; then
    echo -e "Error: missing argument <kind cluster name>" >&2
    usage_text >&2
    exit 1
  fi
}

function validate_registry_params() {
  local registry_env_vars
  registry_env_vars="\$DOCKER_SERVER \$DOCKER_USERNAME \$DOCKER_PASSWORD \$REPOSITORY_PREFIX \$KPACK_BUILDER_REPOSITORY"

  if [ -z ${DOCKER_SERVER+x} ] &&
    [ -z ${DOCKER_USERNAME+x} ] &&
    [ -z ${DOCKER_PASSWORD+x} ] &&
    [ -z ${REPOSITORY_PREFIX+x} ] &&
    [ -z ${KPACK_BUILDER_REPOSITORY+x} ]; then

    echo "None of $registry_env_vars are set. Assuming local registry."
    DOCKER_SERVER="$LOCAL_DOCKER_REGISTRY_ADDRESS"
    DOCKER_USERNAME=""
    DOCKER_PASSWORD=""
    REPOSITORY_PREFIX="$DOCKER_SERVER/"
    KPACK_BUILDER_REPOSITORY="$DOCKER_SERVER/kpack-builder"

    return
  fi

  echo "The following env vars should either be set together or none of them should be set: $registry_env_vars"
  echo "$DOCKER_SERVER $DOCKER_USERNAME $DOCKER_PASSWORD $REPOSITORY_PREFIX $KPACK_BUILDER_REPOSITORY" >/dev/null
}

function start_uaa_docker() {
  echo "Starting UAA in Docker..."
  
  docker stop uaa 2>/dev/null || true
  docker rm uaa 2>/dev/null || true
  
  docker run -d \
    --name uaa \
    --hostname uaa-127-0-0-1.nip.io \
    -p 8080:8080 \
    -v "${TEMPLATES_DIR}/uaa:/etc/config:ro" \
    -e JAVA_OPTS="-Dspring_profiles=hsqldb -Djava.security.egd=file:/dev/./urandom -DCLOUDFOUNDRY_CONFIG_PATH=/etc/config" \
    cloudfoundry/uaa@sha256:7f080becfe62a71fe0429c62ad8afdf4f24e0aac94d9f226531ab3001fa35880
  
  echo "Waiting for UAA to be ready..."
  for i in {1..30}; do
    if curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
      echo "UAA is ready!"
      return 0
    fi
    echo "Waiting for UAA to start (attempt $i/30)..."
    sleep 2
  done
  
  echo "ERROR: UAA failed to start within 60 seconds"
  docker logs uaa
  exit 1
}

function prepare_uaa_oidc_config() {
  echo "Preparing UAA OIDC configuration for kind cluster..."
  
  mkdir -p /tmp/uaa-certs
  
  cat > /tmp/uaa-certs/jwt-private-key.pem <<'EOF'
-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQC86TXUwHix62GR
WueivzokOU4s9ymEGfehe2BTbiDquv9xW7qaiKp3kvAcjRJ6GOw/bbtIypE46As4
8F+E9pDGLMY0lFD7KOSgBmA9dYsYw+RkxVPNC9BrgjWKExvAUF+5R116dDT8kxud
SnAMafXQ8ayRMg4EbRPJjw6Op9lyyqzw4I0cWP0d3mg/6WTXT/R6L2W2z+M0g6tD
v00JAq/sZqVfEGxruo1K4FFgmWEOLKX0W6OXk/uHDq3r99Rfri5JsrlNPY0dp0Db
678J/eS/BlQKkAFczWzaiFHjXlOMawBhoWNgv4iKdy0GOUYVVl13KIsj10RzhW3Y
pOpIKNrhAgMBAAECggEACCuj/uQmNDfEfAds/k58AsYRugMkogiHe838wA8C0HQv
CSWZAAcKLGrIBMwbMPmz+hhSYdcVCduLZLaMwxDw+QlFt0904zAFF2C/N9lGH1eV
oMAiHDu3E3dJvoOODzbKtRY/lkTBZ+0q5BYsm3TXw2Y4ev0puwpGsVCFJilfT8Ye
NDxk3geKv+XUN8e1IYxUokAuELJFESGN8dAxVVMhf1opGtEOy+L/+fCXHZTvE3u3
8f9PsmOqmD+xr1BSQP04duTAgd5frtwXTXPvK3QlG2GSDzm/uAO84FAfYBcj84NP
AHzKigXlhshcDk5rRGCGWk0UYKrlkvonJik64UA0swKBgQDwkwxvbniAkMWzL/mB
ZQvmWTatubriQ2F+ol3l8py878bk2jAd2HDuhQZ3YPrSilG2JZo5LxRadcz861TB
L64W6AbUAlUCsK2/qmjWTVgpK6RYje5efr0YVWTGfiXnsFj6KKSe70yxk9lxRnzH
SVsYDyyWM3Joe9JhBwPRoAeiAwKBgQDJBh+FMB7TCK+/+wkjxfJksGjoUZZbcn/1
3adiWCa+g8/8Cgg3XoQzzJ5fpnkm1w34uro30DGB6oe8vioAJeMmOmjj7U6MNf4k
Kgn3G3NUv7Y1CVDDcpeK7kQ4yEucEwyaoTr+FNXpXx8ropybcgAsLbyVixSAZhcE
qX6urEbMSwKBgCOFwwdNK5PoTJjp05Csp/YqZC2AyDySsHmvZegHS+eGDDtMkGBH
zl0Z3VuRQVgHPouDv+MDtaCp1kveP9SKwsz1E9UIRx8vkWhEtFg4cXUa0ZiV1IW1
dxx5t3irtdMhMfI2QCCLuypZZ3kXbGNMzJuf2fiPviv5ZJYZIBI67AWbAoGAQO0g
WxUar5Bbq0b6QbqaOlkb2QUY6fpGR/PKLyJHiTrrfv0CgFefnVdWQ5ByCtBkq9Qr
dwFgLBTCuHw29otGHT+6RvuLZg++QJHvXAdaraGpyOF0W1v0hCPGlwxiF0uzw3GV
qyCxoklduOsxZ6dfVOWExkwAWCQhBRl1WBc+WpcCgYA1zFDPzM+LDcqGJdp+XH7c
yHhqAnBgaopRq8pbgp4zvC9SMUIX8Hd/AGqDNQO7FjMfRFrTcrWdIb2W1EWxI3j8
3D8DCRZNPZAoVG1BQcjRsgtPrFvErxFEBAKXxiuIXUw96RiBEX6xNmD4I10EyGK2
Odnbw1oo+AIMC08hBCRbZg==
-----END PRIVATE KEY-----
EOF

  openssl rsa -in /tmp/uaa-certs/jwt-private-key.pem -pubout -out /tmp/uaa-certs/jwt-public-key.pem 2>/dev/null
  
  echo "UAA OIDC configuration prepared."
}

function ensure_kind_cluster() {
  if ! kind get clusters | grep -q "$CLUSTER_NAME"; then
    prepare_uaa_oidc_config
    
    cat > /tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
  - containerPath: /etc/uaa-certs
    hostPath: /tmp/uaa-certs
    readOnly: true
  extraPortMappings:
  - containerPort: 32080
    hostPort: 80
    protocol: TCP
  - containerPort: 32443
    hostPort: 443
    protocol: TCP
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraVolumes:
        - name: uaa-certs
          hostPath: /etc/uaa-certs
          mountPath: /etc/uaa-certs
          readOnly: true
      extraArgs:
        oidc-issuer-url: http://uaa-127-0-0-1.nip.io:8080/uaa/oauth/token
        oidc-client-id: cf
        oidc-username-claim: user_name
        oidc-username-prefix: "uaa:"
        oidc-signing-algs: "RS256"
        "enable-admission-plugins": "NodeRestriction"
    controllerManager:
      extraArgs:
        "bind-address": "0.0.0.0"
    scheduler:
      extraArgs:
        "bind-address": "0.0.0.0"
  - |
    kind: KubeletConfiguration
    cgroupDriver: systemd
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
    endpoint = ["http://host.docker.internal:5001"]
EOF
    kind create cluster --name "$CLUSTER_NAME" --config /tmp/kind-config.yaml --wait 5m
    
    kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: kube-system
data:
  uaa.override: |
    rewrite stop {
      name regex uaa-127-0-0-1.nip.io host.docker.internal
      answer name host.docker.internal uaa-127-0-0-1.nip.io
    }
EOF
    
    kubectl -n kube-system rollout restart deployment/coredns
    kubectl -n kube-system rollout status deployment/coredns --timeout=2m
  fi

  kind export kubeconfig --name "$CLUSTER_NAME"
}

function deploy_korifi() {
  echo "Deploying Korifi v${KORIFI_VERSION} from stable release..."

  # Install Korifi using the install YAML
  kubectl apply -f https://github.com/cloudfoundry/korifi/releases/download/v${KORIFI_VERSION}/install-korifi-kind.yaml
  
  # Wait for the installation job to complete
  echo "Waiting for Korifi installation job to complete..."
  kubectl wait --for=condition=Complete job/install-korifi -n korifi-installer --timeout=15m
  
  # Apply our custom UAA-enabled configuration
  kubectl create configmap korifi-api-config --from-file="$TEMPLATES_DIR/korifi_config.yaml" -n korifi --dry-run=client -o yaml | kubectl apply -f -

  # Restart API to make sure our config get's picked up
  kubectl -n korifi rollout restart deployment korifi-api-deployment
  
  # Wait for Korifi components to be ready
  kubectl wait --for=condition=Available=True deployment/korifi-api-deployment -n korifi --timeout=10m
  kubectl wait --for=condition=Available=True deployment/korifi-controllers-controller-manager -n korifi --timeout=5m
  
  echo "Korifi deployment completed!"
}

function configure_korifi_for_uaa() {
  echo "Configuring Korifi to use Docker UAA..."
  
  kubectl apply -f "$TEMPLATES_DIR/uaa-httproute.yaml"
  
  echo "Korifi UAA configuration completed!"
}

function configure_uaa_rbac() {
  echo "Configuring RBAC for UAA admin user..."
  
  kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    cloudfoundry.org/propagate-cf-role: "true"
  name: uaa-admin-binding
  namespace: cf
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: korifi-controllers-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: "uaa:admin"
EOF

  echo "RBAC configuration completed!"
}

function main() {
  parse_cmdline_args "$@"
  start_uaa_docker
  ensure_kind_cluster "$CLUSTER_NAME"
  deploy_korifi
  configure_korifi_for_uaa
  configure_uaa_rbac

  echo ""
  echo "✅ Korifi with UAA deployment completed successfully!"
  echo ""
  echo "UAA Access:"
  echo "  - UAA URL: http://uaa-127-0-0-1.nip.io:8080/uaa"
  echo "  - Admin user: admin/admin_secret"
  echo ""
  echo "Korifi Access:"
  echo "  - API URL: https://localhost:443"
  echo "  - Test login: echo admin_secret | cf login -a https://localhost:443 --skip-ssl-validation -u admin"
  echo ""
  echo "OIDC Configuration:"
  echo "  - Issuer URL: http://uaa-127-0-0-1.nip.io:8080/uaa/oauth/token"
  echo "  - Username prefix: uaa:"
  echo "  - Admin Kubernetes user: uaa:admin"
  echo ""
  echo "Note: The Kubernetes API server has been configured with OIDC authentication"
  echo "to validate UAA JWT tokens. This resolves the 'Invalid Auth Token' error."
  echo ""
}

main "$@"