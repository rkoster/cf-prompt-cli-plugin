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

function ensure_kind_cluster() {
  if ! kind get clusters | grep -q "$CLUSTER_NAME"; then
    cat > /tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 32080
    hostPort: 80
    protocol: TCP
  - containerPort: 32443
    hostPort: 443
    protocol: TCP
EOF
    kind create cluster --name "$CLUSTER_NAME" --config /tmp/kind-config.yaml --wait 5m
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

function deploy_uaa() {
  echo "Deploying UAA using templates..."
  
  # Create UAA namespace
  kubectl create namespace uaa-system --dry-run=client -o yaml | kubectl apply -f -
  
  # Deploy UAA components using our templates
  kubectl apply -f "$TEMPLATES_DIR/uaa-config-updated.yaml"
  kubectl apply -f "$TEMPLATES_DIR/uaa-deployment-fixed.yaml"
  kubectl apply -f "$TEMPLATES_DIR/uaa-httproute.yaml"
  
  # Wait for UAA to be ready
  echo "Waiting for UAA to be ready..."
  kubectl wait --for=condition=Available=True deployment/uaa -n uaa-system --timeout=10m
  
  echo "UAA deployment completed!"
}

function main() {
  parse_cmdline_args "$@"
  ensure_kind_cluster "$CLUSTER_NAME"
  deploy_korifi
  deploy_uaa

  echo ""
  echo "✅ Korifi with UAA deployment completed successfully!"
  echo ""
  echo "UAA Access:"
  echo "  - UAA URL: http://uaa-127-0-0-1.nip.io/uaa"
  echo "  - Admin user: admin/admin_secret"
  echo ""
  echo "Korifi Access:"
  echo "  - API URL: https://localhost:443"
  echo "  - Test login: echo -e \"admin\\nadmin_secret\" | CF_TRACE=true cf login -a https://localhost:443 --skip-ssl-validation"
  echo ""
}

main "$@"