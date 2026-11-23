# 部署

| 项 | 值 |
|----|----|
| 镜像仓库 | `krccr.ccs.tencentyun.com/igokorea/depman` |
| 构建机 | `VM-4-11-ubuntu`（nerdctl + buildkit，约 2C2G） |
| 运行集群 | `zabbix68`，Namespace `depman` |
| Deployment | `depman` |
| 域名 | `depman.igozhang.cn` |

## 构建并推送（VM-4-11，仓库根目录）

```bash
systemctl is-active buildkit || systemctl start buildkit
```

```bash
TAG=$(date +%Y%m%d-%H%M%S) && echo "TAG=${TAG}" && BUILDKIT_MAX_PARALLELISM=1 nerdctl build --progress=plain -t "krccr.ccs.tencentyun.com/igokorea/depman:${TAG}" -t "krccr.ccs.tencentyun.com/igokorea/depman:latest" . && nerdctl push "krccr.ccs.tencentyun.com/igokorea/depman:${TAG}" && nerdctl push "krccr.ccs.tencentyun.com/igokorea/depman:latest"
```

前端在镜像构建阶段嵌入，不写回当前 git 目录。

## 部署（zabbix68）

```bash
kubectl apply -f deploy/k8s/depman.yaml && kubectl -n depman rollout restart deploy/depman && kubectl -n depman rollout status deploy/depman && kubectl -n depman get pod,svc,ingress,pvc
```

## 验证

```bash
curl -sS http://depman.igozhang.cn/api/health
```

## 说明

- 滚动更新：`RollingUpdate`（`maxUnavailable: 0`），新 Pod Ready 后再下旧 Pod，避免短暂不可访问
- PVC：`local-path` RWO → `/data`；滚动时依赖同节点挂载（已配置 podAffinity 优先同节点）。若新 Pod 一直 Pending，检查是否被调度到其他节点
- 配置目录：`DEPMAN_ETC_DIR=/data/etc`（与 DB 同 PVC）；镜像内默认 conf 在 `/etc/depman`，首次启动若 PVC 下无 conf 会按程序逻辑生成
- 环境变量：`DEPMAN_ADDR` / `DEPMAN_DATA_DIR` / `DEPMAN_DB_PATH` / `DEPMAN_ETC_DIR` / `TZ`
- 无登录即可只读浏览；写操作须「登录」页验证 key（`GET /api/v1/auth/igokey` 取超管钥）
- 保留 PVC 升级
