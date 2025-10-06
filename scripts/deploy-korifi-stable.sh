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
  
  # Check if route already exists
  if route -n get -net 192.168.5.0/24 >/dev/null 2>&1; then
    echo "Route 192.168.5.0/24 already exists - skipping"
    return 0
  fi
  
  echo "Adding route: 192.168.5.0/24 via bridge100"
  
  # Add the route
  if sudo route add -net 192.168.5.0/24 -interface bridge100; then
    echo "Static route added successfully"
  else
    echo "Failed to add route"
    return 1
  fi
}



function get_uaa_ip() {
  # Determine UAA IP based on platform
  if [ "$(uname)" = "Darwin" ] && command -v colima >/dev/null 2>&1; then
    # macOS with Colima - use Colima host IP
    echo "192.168.64.2"
  else
    # Linux or other platforms - use kind gateway IP
    echo "172.30.0.1"
  fi
}

function generate_config_files() {
  local uaa_ip=$(get_uaa_ip)
  echo "Generating configuration files with UAA IP: $uaa_ip"
  
  # Generate uaa.yml from template
  if [ -f "${TEMPLATES_DIR}/uaa/uaa.yml.template" ]; then
    echo "Generating uaa.yml from template..."
    sed "s/UAA_IP_PLACEHOLDER/$uaa_ip/g" "${TEMPLATES_DIR}/uaa/uaa.yml.template" > "${TEMPLATES_DIR}/uaa/uaa.yml"
  else
    echo "Warning: uaa.yml.template not found, using existing uaa.yml"
  fi
  
  # Generate korifi_config.yaml from template
  if [ -f "${TEMPLATES_DIR}/korifi_config.yaml.template" ]; then
    echo "Generating korifi_config.yaml from template..."
    sed "s/UAA_IP_PLACEHOLDER/$uaa_ip/g" "${TEMPLATES_DIR}/korifi_config.yaml.template" > "${TEMPLATES_DIR}/korifi_config.yaml"
  else
    echo "Warning: korifi_config.yaml.template not found, using existing korifi_config.yaml"
  fi
}

function start_uaa_docker() {
  echo "Starting UAA with nginx SSL termination proxy..."
  
  docker stop uaa nginx-ssl 2>/dev/null || true
  docker rm uaa nginx-ssl 2>/dev/null || true
  
  # Ensure SSL certificate directory exists
  mkdir -p "${TEMPLATES_DIR}/uaa-cert"
  
  # Generate SSL certificate with SAN including both platform IPs
  if [ ! -f "${TEMPLATES_DIR}/uaa-cert/uaa.crt" ]; then
    echo "Generating SSL certificate for UAA with SAN (including both platform IPs)..."
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
IP.3 = 192.168.5.2
IP.4 = 192.168.64.2
EOF
    
    openssl req -new -x509 -nodes -days 365 \
      -config "${TEMPLATES_DIR}/uaa-cert/uaa.cnf" \
      -keyout "${TEMPLATES_DIR}/uaa-cert/uaa.key" \
      -out "${TEMPLATES_DIR}/uaa-cert/uaa.crt"
  fi
  
  # Generate configuration files with correct UAA IP
  generate_config_files
  
  # Get platform-specific UAA IP
  local uaa_ip=$(get_uaa_ip)
  echo "Using UAA IP: $uaa_ip"
  
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
  
  # Wait for HTTPS to be ready at UAA IP
  echo "Waiting for nginx HTTPS proxy to be ready at $uaa_ip:8443..."
  for i in {1..15}; do
    if curl -k -s https://$uaa_ip:8443/login > /dev/null 2>&1; then
      echo "UAA is ready on HTTPS at https://$uaa_ip:8443!"
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
  local uaa_ip=$(get_uaa_ip)
  echo "Connecting UAA containers to kind network for $uaa_ip access..."
  
  # Only connect to kind network if using Linux (172.30.0.1)
  # Colima/macOS uses the host network bridge
  if [ "$uaa_ip" = "172.30.0.1" ]; then
    # Connect nginx-ssl to kind network (this provides the 172.30.0.1:8443 endpoint)
    if docker ps -q -f name=nginx-ssl > /dev/null 2>&1; then
      docker network connect kind nginx-ssl 2>/dev/null || echo "nginx-ssl already connected to kind network"
    fi
    
    # Connect UAA container as well
    if docker ps -q -f name=uaa > /dev/null 2>&1; then
      docker network connect kind uaa 2>/dev/null || echo "uaa already connected to kind network"
    fi
    
    echo "UAA accessible on kind network at $uaa_ip:8443 (nginx-ssl proxy)"
  else
    echo "Using Colima/macOS - UAA accessible at $uaa_ip:8443 via host network"
  fi
}

function ensure_kind_cluster() {
  # Always prepare UAA OIDC config - needed for both new and existing clusters
  prepare_uaa_oidc_config
  
  if ! kind get clusters | grep -q "$CLUSTER_NAME"; then
    local uaa_ip=$(get_uaa_ip)
    
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
        oidc-issuer-url: https://$uaa_ip:8443/oauth/token
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
  setup_macos_colima_route
  start_uaa_docker
  ensure_kind_cluster "$CLUSTER_NAME"
  deploy_korifi
  configure_uaa_rbac

  local uaa_ip=$(get_uaa_ip)

  echo ""
  echo "✅ Korifi with UAA deployment completed successfully!"
  echo ""
  echo "UAA Access:"
  echo "  - UAA URL: https://$uaa_ip:8443/uaa"
  echo "  - Admin user: admin/admin_secret"
  echo ""
  echo "Korifi Access:"
  echo "  - API URL: https://localhost:443"
  echo "  - Test login: echo -e \"admin\\nadmin_secret\" | CF_TRACE=true cf login -a https://localhost:443 --skip-ssl-validation"
  echo ""
}

main "$@"