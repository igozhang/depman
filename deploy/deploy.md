# depman 部署

| 项 | 值 |
|----|----|
| 镜像 | `krccr.ccs.tencentyun.com/igokorea/depman` |
| 构建机 | `VM-4-11-ubuntu`（nerdctl） |
| 集群 | `zabbix68` · ns / deploy = `depman` |
| 域名 | `depman.igozhang.cn` |

## 1. 构建推送（仓库根目录）

```bash
systemctl is-active buildkit || systemctl start buildkit

TAG=$(date +%Y%m%d-%H%M%S) && echo "TAG=${TAG}" && \
BUILDKIT_MAX_PARALLELISM=1 nerdctl build --progress=plain \
  -t "krccr.ccs.tencentyun.com/igokorea/depman:${TAG}" \
  -t "krccr.ccs.tencentyun.com/igokorea/depman:latest" . && \
nerdctl push "krccr.ccs.tencentyun.com/igokorea/depman:${TAG}" && \
nerdctl push "krccr.ccs.tencentyun.com/igokorea/depman:latest"
```

## 2. 部署

私有仓：在 `depman` ns 放入拉取密钥，并挂到 **default ServiceAccount**（清单不写 `imagePullSecrets`）。可用已有 CCR 凭证改名拷贝：

```bash
kubectl -n k8man get secret harbor-qq-secret -o yaml \
  | sed 's/namespace: k8man/namespace: depman/; s/name: harbor-qq-secret/name: tcr-igokorea/' \
  | grep -vE '^\s*(resourceVersion|uid|creationTimestamp):' \
  | kubectl apply -f - \
&& kubectl -n depman patch serviceaccount default --type=strategic \
  -p '{"imagePullSecrets":[{"name":"tcr-igokorea"}]}'
```

```bash
kubectl apply -f deploy/k8s/depman.yaml && \
kubectl -n depman rollout restart deploy/depman && \
kubectl -n depman rollout status deploy/depman && \
kubectl -n depman get pod,svc,ingress,pvc
```

## 3. 验证

```bash
curl -sS http://depman.igozhang.cn/api/health
```
