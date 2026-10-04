#!/bin/bash
set -euo pipefail

echo "=== Day16 E2E: CloudRelease Operator ==="

# 确认集群连通
echo "[1/8] Checking cluster..."
kubectl get nodes

# 启动 Manager（后台）
echo "[2/8] Starting manager..."
go run ./cmd/main.go &
MANAGER_PID=$!
trap "kill $MANAGER_PID 2>/dev/null || true" EXIT

# 等 Manager 连上集群
sleep 3

# 创建 CloudRelease
echo "[3/8] Creating CloudRelease demo..."
kubectl apply -f config/samples/delivery_v1alpha1_cloudrelease.yaml

# 等待 Deployment Ready
echo "[4/8] Waiting for Deployment 2/2 ready..."
kubectl wait --for=condition=Available deployment/demo --timeout=60s

# 验证 Service 存在
echo "[5/8] Verifying Service..."
kubectl get service demo

# 扩容到 5
echo "[6/8] Scaling to 5 replicas..."
kubectl patch cloudrelease demo --type merge -p '{"spec":{"replicas":5}}'
sleep 3
kubectl wait --for=condition=Available deployment/demo --timeout=60s
kubectl get pods

# 删除 CR，等待级联清理
echo "[7/8] Deleting CloudRelease..."
kubectl delete cloudrelease demo
kubectl wait --for=delete deployment/demo --timeout=30s
kubectl wait --for=delete service/demo --timeout=30s

echo "[8/8] Verifying cleanup..."
kubectl get pods
kubectl get deployment demo 2>&1 || true
kubectl get service demo 2>&1 || true

echo ""
echo "=== E2E PASSED ==="
