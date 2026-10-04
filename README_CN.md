# CloudRelease Operator

[English](README.md)

一个 Kubernetes Operator，通过自定义资源管理 Web 应用部署。
它把高层的 `CloudRelease` 声明翻译成标准的 `Deployment` 和 `Service` 资源。

## 它做什么

不用写完整的 Deployment/Service YAML，用户只需要提交一个 `CloudRelease`：

```yaml
apiVersion: delivery.cloudrelease.dev/v1alpha1
kind: CloudRelease
metadata:
  name: demo
spec:
  image: nginx:1.27-alpine
  replicas: 3
  port: 80
```

Operator 自动创建并管理：

- 一个 `Deployment`（指定镜像、副本数、容器端口）
- 一个 `Service`（暴露端口）
- 状态回写（`readyReplicas`、`observedGeneration`）

还具备以下能力：

- **自愈**：手动删除 Service 或 Deployment 后自动重建
- **扩缩容**：修改 `spec.replicas` 自动调整 Deployment 副本数
- **级联删除**：删除 CR 自动清理所有子资源
- **Leader Election**：多副本部署时同一时刻只有一个实例在处理 Reconcile

## 快速开始（本地开发）

### 前置要求

- Go 1.24+
- kubectl
- 本地 Kubernetes 集群（kind、minikube 或 Docker Desktop）

### 安装 CRD

```sh
kubectl apply -f config/crd/bases/
```

### 本地启动 Operator

```sh
go run ./cmd/main.go
```

### 创建一个 CloudRelease

```sh
kubectl apply -f config/samples/delivery_v1alpha1_cloudrelease.yaml
```

验证：

```sh
kubectl get cloudrelease
kubectl get deployment
kubectl get service
kubectl get pods
```

### 清理

```sh
kubectl delete -f config/samples/delivery_v1alpha1_cloudrelease.yaml
```

## 测试

三层测试：

```sh
# 单元测试（fake client，毫秒级）
go test ./internal/controller/

# EnvTest（真实 kube-apiserver，验证 CRD schema 和默认值）
export KUBEBUILDER_ASSETS=$PWD/bin/k8s/1.36.2-linux-amd64
go test -tags=envtest ./test/envtest/

# E2E 脚本（kind 集群，全链路验证）
./hack/e2e.sh
```

## 架构

```
CloudRelease (CR)
  spec.image / replicas / port
        │
        ▼
  Reconcile 调谐循环
        │
        ├──▶ Deployment (apps/v1)
        │      replicas = spec.replicas
        │      image    = spec.image
        │      port     = spec.port
        │
        ├──▶ Service (v1)
        │      port = spec.port
        │
        └──▶ Status
               observedGeneration
               readyReplicas
```

每次 Reconcile：

1. 读取 `CloudRelease` 对象
2. 对比期望状态（spec）和实际状态（Deployment/Service）
3. 创建或 patch 子资源，使其对齐
4. 回写 status

调谐循环是 **level-triggered（电平触发）**：每次都读取当前状态并收敛，
不依赖事件队列持久化。Operator 重启后不需要人工恢复。

## 设计取舍

- **CreateOrPatch 保证幂等**：所有子资源用 `controllerutil.CreateOrPatch` 调谐，
  重复执行 Reconcile 结果一致。
- **UID 作为 selector**：Deployment 用 CloudRelease 的 UID 作为 label selector，
  防止 Operator 误接管同名的陌生资源。
- **OwnerReference 级联删除**：子资源设置 `OwnerReference` 指向父 CR，
  删除 CR 时 K8s 垃圾回收器自动清理子资源。
- **不做 Webhook**：校验由 CRD schema marker（`+kubebuilder:validation`）在
  API Server 侧完成，不引入 admission webhook，保持项目简洁。
- **Status 子资源隔离**：status 通过 `/status` 写入，与 spec 更新隔离。

## 项目结构

```
api/v1alpha1/       # CloudRelease 类型定义和 CRD marker
internal/controller/ # Reconcile 逻辑和单元测试
cmd/main.go         # Manager 入口
config/
  crd/bases/        # 生成的 CRD YAML
  rbac/             # 生成的 RBAC 权限
  samples/          # 示例 CloudRelease
test/envtest/       # EnvTest 集成测试
hack/e2e.sh         # E2E 脚本
```

## License

Apache License 2.0.
