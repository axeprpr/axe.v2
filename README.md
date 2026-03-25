# axe.v2

`axe` is a lightweight CLI for SSH operations across one or many hosts.  
`axe` 是一个轻量 SSH CLI，可用于单机或多机操作。

## Features | 功能

- `axe <tag>`: SSH into one host by tag (or use tag as raw host).
  - 按标签连接单台主机（若无匹配标签则把 tag 当作主机地址）。
- `axe <tag1> <tag2> -c "cmd"`: Run one command on multiple hosts.
  - 在多台主机上执行同一命令。
- `axe <tag1> <tag2> -s ./local /remote`: Copy file/dir to multiple hosts.
  - 向多台主机复制文件或目录。
- `axe tag list|add|edit|del`: Manage host tags in CLI.
  - 通过命令行管理主机标签。
- `axe default show|set`: Manage default SSH settings.
  - 管理默认 SSH 连接参数。
- `axe version`: Show version.
  - 查看版本。

## Configuration | 配置

Default config file: `.axe.v2.config.json`  
默认配置文件：`.axe.v2.config.json`

Example | 示例:

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

## Tag Commands | 标签管理

```bash
# List tags | 列出标签
axe tag list

# Add tag | 新增标签
axe tag add prod-a 10.0.0.11 --port 22 --user root

# Edit tag | 修改标签
axe tag edit prod-a --address 10.0.0.12 --port 2222

# Delete tag | 删除标签
axe tag del prod-a
```

## Default Commands | 默认连接参数

```bash
# Show defaults | 查看默认值
axe default show

# Update defaults | 更新默认值
axe default set --port 22 --user root
```

## Batch Execution Flags | 批量执行参数

- `--parallel N`: Max concurrency (default `5`).  
  最大并发数（默认 `5`）。
- `--timeout 20s`: Per-host timeout (default `15s`).  
  单机超时（默认 `15s`）。
- `--retries N`: Retry count on failure (default `0`).  
  失败重试次数（默认 `0`）。
- `--dry-run`: Print planned commands only.  
  仅打印将执行命令，不实际执行。
- `--verbose`: Print retry/debug details.  
  打印重试与调试信息。
- `--json`: Output per-host result in JSON lines.  
  按主机输出 JSON 行，便于集成。

Example | 示例:

```bash
axe --dry-run --json --parallel 10 web-1 web-2 -c "hostname"
```

## Environment Variables | 环境变量

- `axe_debug`: enable debug output | 开启调试输出
- `axe_port`: override default port | 覆盖默认端口
- `axe_username`: override default username | 覆盖默认用户名
- `axe_password`: override default password | 覆盖默认密码

## Build | 构建

```bash
go build ./...
```

## CI | 自动构建

GitHub Actions workflow: `.github/workflows/build.yml`  
GitHub Actions 工作流：`.github/workflows/build.yml`

Triggers | 触发条件:
- `push` to any branch | 推送到任意分支
- `pull_request`
