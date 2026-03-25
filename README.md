# axe.v2

`axe` 是一个轻量 SSH 批量操作 CLI。

## 功能

- `axe <tag>`: 连接单台主机。
- `axe <tag1> <tag2> -c "cmd"`: 在多台主机执行同一命令。
- `axe <tag1> <tag2> -s ./local /remote`: 向多台主机传文件。
- `axe -e` / `axe -l` / `axe -lp`: 编辑配置文件。

## 配置文件

默认文件：`.axe.v2.config.json`

示例：

```json
{
  "passwords": [
    {
      "default": true,
      "port": "22",
      "username": "root",
      "password": ""
    }
  ],
  "tags": [
    {
      "tag": "prod-a",
      "address": "10.0.0.11",
      "port": "22",
      "username": "root",
      "password": ""
    }
  ]
}
```

## 环境变量

- `axe_debug`: 打开调试输出。
- `axe_port`: 覆盖默认端口。
- `axe_username`: 覆盖默认用户名。
- `axe_password`: 覆盖默认密码。

## 本地构建

```bash
go build ./...
```

## 自动构建

仓库已包含 GitHub Actions：`.github/workflows/build.yml`

触发条件：
- `push` 到任意分支
- `pull_request`
