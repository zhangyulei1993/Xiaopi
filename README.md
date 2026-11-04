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

## 小程序

```bash
cd miniapp
./node_modules/.bin/uni build -p mp-weixin --mode development --emptyOutDir
```

微信开发者工具导入 `miniapp/dist/mp-weixin`。

## 管理后台

```bash
cd admin
./node_modules/.bin/vite --host 0.0.0.0 --port 15174
```

当前阶段已包含微信登录接口占位、房源领域模型、MySQL 持久化、房源列表/详情/发布小程序页面，以及后台登录、房源审核、客户管理基础页面。
