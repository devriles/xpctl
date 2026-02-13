#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="xpctl-integration"
FIXTURES_DIR="$(cd "$(dirname "$0")/../testdata/fixtures" && pwd)"

echo "==> Checking prerequisites..."
for cmd in kind kubectl helm; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not found in PATH"
    exit 1
  fi
done

# Create kind cluster (reuse if it already exists)
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "==> Kind cluster '${CLUSTER_NAME}' already exists, reusing"
else
  echo "==> Creating kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --name "$CLUSTER_NAME" --wait 60s
fi

# Point kubectl at the kind cluster
export KUBECONFIG="$(kind get kubeconfig-path --name="$CLUSTER_NAME" 2>/dev/null || echo "")"
kubectl config use-context "kind-${CLUSTER_NAME}"

# Install Crossplane via Helm
echo "==> Installing Crossplane..."
helm repo add crossplane-stable https://charts.crossplane.io/stable 2>/dev/null || true
helm repo update crossplane-stable

if helm status crossplane -n crossplane-system &>/dev/null; then
  echo "    Crossplane already installed, skipping"
else
  helm install crossplane crossplane-stable/crossplane \
    --namespace crossplane-system --create-namespace \
    --wait --timeout 120s
fi

echo "==> Waiting for Crossplane pods to be ready..."
kubectl wait --for=condition=Available deployment/crossplane \
  -n crossplane-system --timeout=120s
kubectl wait --for=condition=Available deployment/crossplane-rbac-manager \
  -n crossplane-system --timeout=120s

# Install provider-nop
echo "==> Installing provider-nop..."
kubectl apply -f "$FIXTURES_DIR/provider-nop.yaml"

echo "==> Waiting for provider-nop to become healthy..."
for i in $(seq 1 60); do
  if kubectl get provider provider-nop -o jsonpath='{.status.conditions[?(@.type=="Healthy")].status}' 2>/dev/null | grep -q "True"; then
    echo "    provider-nop is healthy"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: provider-nop did not become healthy within 120s"
    kubectl get provider provider-nop -o yaml
    exit 1
  fi
  sleep 2
done

# Install function-patch-and-transform (required for Crossplane v2 pipeline compositions)
echo "==> Installing function-patch-and-transform..."
kubectl apply -f "$FIXTURES_DIR/function-patch-and-transform.yaml"

echo "==> Waiting for function-patch-and-transform to become healthy..."
for i in $(seq 1 60); do
  if kubectl get function function-patch-and-transform -o jsonpath='{.status.conditions[?(@.type=="Healthy")].status}' 2>/dev/null | grep -q "True"; then
    echo "    function-patch-and-transform is healthy"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: function-patch-and-transform did not become healthy within 120s"
    kubectl get function function-patch-and-transform -o yaml
    exit 1
  fi
  sleep 2
done

# Apply XRDs and wait for them to be established
echo "==> Applying XRDs..."
kubectl apply -f "$FIXTURES_DIR/xrd.yaml"
kubectl apply -f "$FIXTURES_DIR/nested-xrd.yaml"

echo "==> Waiting for XRD xnopapps to be established..."
for i in $(seq 1 60); do
  if kubectl get xrd xnopapps.test.xpctl.io -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' 2>/dev/null | grep -q "True"; then
    echo "    XRD xnopapps is established"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: XRD xnopapps did not become established within 120s"
    kubectl get xrd xnopapps.test.xpctl.io -o yaml
    exit 1
  fi
  sleep 2
done

echo "==> Waiting for XRD xnopsubnets to be established..."
for i in $(seq 1 60); do
  if kubectl get xrd xnopsubnets.test.xpctl.io -o jsonpath='{.status.conditions[?(@.type=="Established")].status}' 2>/dev/null | grep -q "True"; then
    echo "    XRD xnopsubnets is established"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: XRD xnopsubnets did not become established within 120s"
    kubectl get xrd xnopsubnets.test.xpctl.io -o yaml
    exit 1
  fi
  sleep 2
done

# Apply all Compositions
echo "==> Applying Compositions..."
kubectl apply -f "$FIXTURES_DIR/composition.yaml"
kubectl apply -f "$FIXTURES_DIR/nested-composition-subnet.yaml"
kubectl apply -f "$FIXTURES_DIR/nested-composition-top.yaml"
kubectl apply -f "$FIXTURES_DIR/unhealthy-composition.yaml"

# Apply XR instances
echo "==> Applying XR (test-app)..."
kubectl apply -f "$FIXTURES_DIR/xr.yaml"

echo "==> Applying Claim (test-app-claim)..."
kubectl apply -f "$FIXTURES_DIR/claim.yaml"

echo "==> Applying nested XR (test-app-nested)..."
kubectl apply -f "$FIXTURES_DIR/nested-xr.yaml"

echo "==> Applying unhealthy XR (test-app-unhealthy)..."
kubectl apply -f "$FIXTURES_DIR/unhealthy-xr.yaml"

# Wait for test-app to become Ready
echo "==> Waiting for XR 'test-app' to become Ready..."
for i in $(seq 1 60); do
  if kubectl get xnopapps.test.xpctl.io test-app -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; then
    echo "    XR test-app is Ready"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: XR test-app did not become Ready within 120s"
    kubectl get xnopapps.test.xpctl.io test-app -o yaml
    exit 1
  fi
  sleep 2
done

# Wait for Claim to become Ready
echo "==> Waiting for Claim 'test-app-claim' to become Ready..."
for i in $(seq 1 60); do
  if kubectl get nopapps.test.xpctl.io test-app-claim -n default -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; then
    echo "    Claim test-app-claim is Ready"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: Claim test-app-claim did not become Ready within 120s"
    kubectl get nopapps.test.xpctl.io test-app-claim -n default -o yaml
    exit 1
  fi
  sleep 2
done

# Wait for nested XR to become Ready
echo "==> Waiting for XR 'test-app-nested' to become Ready..."
for i in $(seq 1 60); do
  if kubectl get xnopapps.test.xpctl.io test-app-nested -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; then
    echo "    XR test-app-nested is Ready"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: XR test-app-nested did not become Ready within 120s"
    kubectl get xnopapps.test.xpctl.io test-app-nested -o yaml
    exit 1
  fi
  sleep 2
done

# Wait for unhealthy XR to have composed resources (it won't be Ready, just wait for Synced=False)
echo "==> Waiting for XR 'test-app-unhealthy' to have composed resources..."
for i in $(seq 1 60); do
  refs=$(kubectl get xnopapps.test.xpctl.io test-app-unhealthy -o jsonpath='{.spec.resourceRefs}' 2>/dev/null)
  if [ -n "$refs" ] && [ "$refs" != "[]" ]; then
    echo "    XR test-app-unhealthy has composed resources"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: XR test-app-unhealthy did not get composed resources within 120s"
    kubectl get xnopapps.test.xpctl.io test-app-unhealthy -o yaml
    exit 1
  fi
  sleep 2
done

echo ""
echo "==> Integration environment is ready!"
echo "    Cluster: ${CLUSTER_NAME}"
echo "    Use: kubectl config use-context kind-${CLUSTER_NAME}"
