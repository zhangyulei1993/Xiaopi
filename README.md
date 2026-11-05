# 小皮租房

## 当前开发环境

```bash
docker compose -f docker/docker-compose.yml up -d

# 线上仅重建 API：在服务器使用 /opt/xiaopi/scripts/deploy-server-api.sh。
# 它会显式读取 /opt/xiaopi/.env，避免误用 docker/.env（该文件不保存密钥）。
```

- API：`http://127.0.0.1:18080`
- MySQL：`127.0.0.1:13306`
- Redis：`127.0.0.1:16379`

### 房源媒体存储（生产）

配置 `COS_BUCKET`、`COS_REGION`、`COS_SECRET_ID`、`COS_SECRET_KEY` 与
`COS_PUBLIC_BASE` 后，新上传的房源图片和视频会直接存入腾讯云 COS；未配置时仍
使用本地 `uploads` 目录，方便开发环境继续运行。生产密钥仅保存在
`/opt/xiaopi/.env`，不要写入仓库或小程序端。

## 小程序

```bash
# 在 develop 工作区的仓库根目录执行；会拒绝从其他分支/工作区构建。
./scripts/build-miniapp-develop.sh
```

微信开发者工具只导入脚本最后输出的绝对路径。不要导入其他 worktree
（例如 `xiaopi/miniapp/dist/mp-weixin`）中的构建目录。

## 管理后台

```bash
cd admin
./node_modules/.bin/vite --host 0.0.0.0 --port 15174
```

当前阶段已包含微信登录接口占位、房源领域模型、MySQL 持久化、房源列表/详情/发布小程序页面，以及后台登录、房源审核、客户管理基础页面。
