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

function setup_macos_colima_route() {
  # Only run on macOS with Colima
  if [ "$(uname)" != "Darwin" ] || ! command -v colima >/dev/null 2>&1; then
    return 0
  fi
  
  # Check if Colima is running
  if ! colima status >/dev/null 2>&1; then
    echo "Warning: Colima is not running. Route setup skipped."
    return 0
  fi
  
  echo "Detected macOS with Colima - setting up static route for kind network..."
  
  # Get Colima VM IP
  local colima_ip=$(colima ssh -- ip addr show eth0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1)
  
  if [ -z "$colima_ip" ]; then
    echo "Warning: Could not determine Colima VM IP. Route setup skipped."
    return 0
  fi
  
  echo "Adding route: 172.30.0.0/16 via $colima_ip"
  
  # Add the route (ignore errors if route already exists)
  if ! sudo route add -net 172.30.0.0/16 "$colima_ip" 2>/dev/null; then
    echo "Route may already exist or failed to add"
  else
    echo "Static route added successfully"
  fi
}

function ensure_kind_network() {
  local expected_subnet="172.30.0.0/16"
  local expected_gateway="172.30.0.1"
  
  # Check if kind network exists
  if docker network inspect kind >/dev/null 2>&1; then
    # Network exists, validate subnet
    local current_subnet=$(docker network inspect kind --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}')
    
    if [ "$current_subnet" != "$expected_subnet" ]; then
      echo "ERROR: kind network exists with incorrect subnet: $current_subnet"
      echo ""
      echo "Expected subnet: $expected_subnet"
      echo ""
      echo "Please delete the kind network and try again:"
      echo "  1. Delete any existing kind clusters: kind delete cluster --name <cluster-name>"
      echo "  2. Delete the kind network: docker network rm kind"
      echo "  3. Re-run this script"
      exit 1
    fi
    
    echo "kind network validated with correct subnet: $expected_subnet"
  else
    # Network doesn't exist, create it with correct subnet
    echo "Creating kind network with subnet $expected_subnet..."
    docker network create kind \
      --driver=bridge \
      --subnet="$expected_subnet" \
      --gateway="$expected_gateway"
    echo "kind network created successfully"
  fi
}

function start_uaa_docker() {
  echo "Starting UAA with nginx SSL termination proxy..."
  
  docker stop uaa nginx-ssl 2>/dev/null || true
  docker rm uaa nginx-ssl 2>/dev/null || true
  
  # Get kind network gateway IP (static IP for UAA access from kind cluster)
  local kind_gateway_ip="172.30.0.1"
  
  # Ensure SSL certificate directory exists
  mkdir -p "${TEMPLATES_DIR}/uaa-cert"
  
  # Generate SSL certificate with SAN including both hostname and gateway IP
  if [ ! -f "${TEMPLATES_DIR}/uaa-cert/uaa.crt" ]; then
    echo "Generating SSL certificate for UAA with SAN (including kind gateway IP 172.30.0.1)..."
    cat > "${TEMPLATES_DIR}/uaa-cert/uaa.cnf" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = req_ext
x509_extensions = v3_ca

[dn]
CN = 172.30.0.1

[req_ext]
subjectAltName = @alt_names

[v3_ca]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
IP.2 = 172.30.0.1
EOF
    
    openssl req -new -x509 -nodes -days 365 \
      -config "${TEMPLATES_DIR}/uaa-cert/uaa.cnf" \
      -keyout "${TEMPLATES_DIR}/uaa-cert/uaa.key" \
      -out "${TEMPLATES_DIR}/uaa-cert/uaa.crt"
  fi
  
  # Start UAA on HTTP only (nginx will handle SSL termination)
  # Configure Tomcat to trust X-Forwarded-* headers from nginx proxy
  docker run -d \
    --name uaa \
    --hostname uaa-172-19-0-1.local \
    -p 8080:8080 \
    -v "${TEMPLATES_DIR}/uaa:/etc/config:ro" \
    -e JAVA_OPTS="-Dspring_profiles=hsqldb -Djava.security.egd=file:/dev/./urandom -DCLOUDFOUNDRY_CONFIG_PATH=/etc/config" \
    -e SERVER_TOMCAT_REMOTEIP_PROTOCOL_HEADER="X-Forwarded-Proto" \
    -e SERVER_TOMCAT_REMOTEIP_PORT_HEADER="X-Forwarded-Port" \
    -e SERVER_TOMCAT_REMOTEIP_INTERNAL_PROXIES=".*" \
    cloudfoundry/uaa@sha256:7f080becfe62a71fe0429c62ad8afdf4f24e0aac94d9f226531ab3001fa35880
  
  # Wait for UAA to be ready on HTTP
  echo "Waiting for UAA HTTP server to be ready..."
  for i in {1..30}; do
    if curl -s http://localhost:8080/login > /dev/null 2>&1; then
      echo "UAA is ready on HTTP!"
      break
    fi
    echo "Waiting for UAA to start (attempt $i/30)..."
    sleep 2
  done
  
  if ! curl -s http://localhost:8080/login > /dev/null 2>&1; then
    echo "ERROR: UAA failed to start within 60 seconds"
    docker logs uaa
    exit 1
  fi
  
  # Start nginx SSL termination proxy
  echo "Starting nginx SSL termination proxy..."
  docker run -d \
    --name nginx-ssl \
    -p 8443:8443 \
    -v "${TEMPLATES_DIR}/nginx.conf:/etc/nginx/nginx.conf:ro" \
    -v "${TEMPLATES_DIR}/uaa-cert/uaa.crt:/etc/ssl/certs/uaa.crt:ro" \
    -v "${TEMPLATES_DIR}/uaa-cert/uaa.key:/etc/ssl/private/uaa.key:ro" \
    --add-host=host.docker.internal:host-gateway \
    nginx:alpine
  
  # Wait for HTTPS to be ready at gateway IP
  echo "Waiting for nginx HTTPS proxy to be ready at 172.30.0.1:8443..."
  for i in {1..15}; do
    if curl -k -s https://172.30.0.1:8443/login > /dev/null 2>&1; then
      echo "UAA is ready on HTTPS at https://172.30.0.1:8443!"
      return 0
    fi
    echo "Waiting for UAA HTTPS access (attempt $i/15)..."
    sleep 2
  done
  
  echo "ERROR: nginx SSL proxy failed to start"
  docker logs nginx-ssl
  exit 1
}

function prepare_uaa_oidc_config() {
  echo "Preparing UAA OIDC configuration for Kubernetes API server..."
  
  # Create directory for OIDC certificates
  mkdir -p /tmp/uaa-oidc
  
  # Copy UAA SSL certificate for K8s API server to trust
  cp "${TEMPLATES_DIR}/uaa-cert/uaa.crt" /tmp/uaa-oidc/uaa-ca.crt
  
  echo "UAA OIDC configuration prepared with CA certificate."
}

function connect_uaa_to_kind_network() {
  echo "Connecting UAA containers to kind network for 172.30.0.1 access..."
  
  # Ensure kind network exists
  if ! docker network inspect kind > /dev/null 2>&1; then
    echo "ERROR: kind network does not exist. Create kind cluster first."
    exit 1
  fi
  
  # Connect nginx-ssl to kind network (this provides the 172.30.0.1:8443 endpoint)
  if docker ps -q -f name=nginx-ssl > /dev/null 2>&1; then
    docker network connect kind nginx-ssl 2>/dev/null || echo "nginx-ssl already connected to kind network"
  fi
  
  # Connect UAA container as well
  if docker ps -q -f name=uaa > /dev/null 2>&1; then
    docker network connect kind uaa 2>/dev/null || echo "uaa already connected to kind network"
  fi
  
  echo "UAA accessible on kind network at 172.30.0.1:8443 (nginx-ssl proxy)"
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
  - containerPath: /etc/uaa-oidc
    hostPath: /tmp/uaa-oidc
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
        - name: uaa-oidc
          hostPath: /etc/uaa-oidc
          mountPath: /etc/uaa-oidc
          readOnly: true
      extraArgs:
        oidc-issuer-url: https://172.30.0.1:8443/oauth/token
        oidc-client-id: cf
        oidc-username-claim: user_name
        oidc-username-prefix: "uaa:"
        oidc-ca-file: /etc/uaa-oidc/uaa-ca.crt
        "enable-admission-plugins": "NodeRestriction"
    controllerManager:
      extraArgs:
        "bind-address": "0.0.0.0"
    scheduler:
      extraArgs:
        "bind-address": "0.0.0.0"
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-ip: "::"
  - |
    kind: KubeletConfiguration
    cgroupDriver: systemd
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localregistry-docker-registry.default.svc.cluster.local:30050"]
        endpoint = ["http://127.0.0.1:30050"]
    [plugins."io.containerd.grpc.v1.cri".registry.configs]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."127.0.0.1:30050".tls]
        insecure_skip_verify = true
EOF
    kind create cluster --name "$CLUSTER_NAME" --config /tmp/kind-config.yaml --wait 5m
    
    # Connect UAA containers to kind network for direct IP access
    connect_uaa_to_kind_network
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

function configure_uaa_rbac() {
  echo "Configuring RBAC for UAA admin user..."
  
  kubectl apply -f "$TEMPLATES_DIR/uaa-admin-rolebinding.yaml"
  
  echo "RBAC configuration completed!"
}

function main() {
  parse_cmdline_args "$@"
  ensure_kind_network
  setup_macos_colima_route
  start_uaa_docker
  ensure_kind_cluster "$CLUSTER_NAME"
  deploy_korifi
  configure_uaa_rbac

  echo ""
  echo "✅ Korifi with UAA deployment completed successfully!"
  echo ""
  echo "UAA Access:"
  echo "  - UAA URL: https://172.30.0.1:8443/uaa"
  echo "  - Admin user: admin/admin_secret"
  echo ""
  echo "Korifi Access:"
  echo "  - API URL: https://localhost:443"
  echo "  - Test login: echo -e \"admin\\nadmin_secret\" | CF_TRACE=true cf login -a https://localhost:443 --skip-ssl-validation"
  echo ""
}

main "$@"