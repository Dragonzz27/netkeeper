# Netkeeper

面向 Linux 服务器的华东理工大学校园网自动登录工具，使用 Go 编写，基于 [Cobra](https://github.com/spf13/cobra) 提供命令行，通过 [chromedp](https://github.com/chromedp/chromedp/tree/v0.13.7) 控制 Chrome/Chromium。支持普通校园网、服务器机房新版 Srunsoft 登录页面、掉线重连和登录间隔可视化。

## 安装

仅支持 Linux。需要 Go 1.24+（仅编译时需要）及 Chrome 或 Chromium；默认使用无头浏览器，可在没有桌面环境的服务器上运行。无需 Python、Selenium、ChromeDriver 或前端构建工具。

```bash
go mod download
go build -o netkeeper .
./netkeeper --help
```

程序会在 `PATH` 中自动查找 `google-chrome`、`google-chrome-stable`、`chromium` 或 `chromium-browser`。浏览器不在 `PATH` 中时，通过 `--chrome-path /path/to/chrome` 指定。

## 账号配置

兼容原项目的 `.env`，无需迁移：

```dotenv
CAMPUS_USERNAME='你的账号'
CAMPUS_PASSWORD='你的密码'
```

可以复制 `.env.example` 后填写，也可通过环境变量设置相同的键。优先级为：`--username` > 环境变量 > `.env` > 交互输入。密码通过环境变量、配置文件或终端输入提供。

首次启动 `run` / `login` 时，缺失的账号密码会提示输入，并写回所选 `.env` 文件；终端输入密码时不回显，新写入的配置文件权限为 `0600`。无人值守运行前请配置好账号密码。`.env` 默认相对于当前工作目录读取，也可用 `--env-file /absolute/path/to/.env` 指定。`status`、`logs`、`serve` 不需要账号配置。

`.env` 以明文保存凭据，请仅存放在可信机器上。`.gitignore` 已忽略它；如果旧版本已跟踪该文件，忽略规则不会自动移除 Git 历史中的凭据。

## 使用

```bash
# 默认每 60 秒检测一次，掉线后自动登录；直接运行 ./netkeeper 效果相同
./netkeeper run

# 原 main_server.py 对应模式：使用 #login 按钮，并设置页面选中的上网类型
./netkeeper run --mode server

# 只登录一次，联网验证成功后退出并记录日志
./netkeeper login

# 仅检测网络；已联网返回 0，离线或检测失败返回 1
./netkeeper status

# 自定义间隔和单次操作超时
./netkeeper run --interval 30s --timeout 45s

# 指定认证页面、上网类型和浏览器
./netkeeper run --mode server --portal-url http://gateway.example/login --domain @free --chrome-path /usr/bin/chromium

# 仅在容器等环境确有需要时使用
./netkeeper run --no-sandbox

# 查看日志表格
./netkeeper logs

# 替代原 Streamlit app.py，在浏览器查看折线图和数据表
./netkeeper serve
```

`serve` 默认监听 `http://127.0.0.1:8501`，页面每 60 秒刷新一次，无外部 CDN 依赖；可用 `--addr` 修改监听地址。持续登录与日志页面可在两个终端分别运行。按 Ctrl+C 停止命令并释放浏览器进程。

常用全局参数：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--mode` | `campus` | `campus` / `server` 登录页面 |
| `--env-file` | `.env` | 配置文件路径 |
| `--log-file` | `login_log.jsonl` | 日志文件路径 |
| `--check-url` | `http://www.baidu.com` | 联网检测地址 |
| `--portal-url` | `http://172.20.3.90:804/srun_portal_pc.php` | 没有发现认证重定向时使用的地址 |
| `--interval` | `1m` | 每轮完成后等待的时间 |
| `--timeout` | `30s` | 每次检测或完整登录操作的超时 |
| `--domain` | 页面默认值 | 手动覆盖上网类型，可传 `--domain ''` |
| `--chrome-path` | 自动查找 | 浏览器可执行文件 |
| `--headless` | `true` | 无头模式 |
| `--no-sandbox` | `false` | 禁用 Chrome 沙箱 |

检测会等待浏览器完成跳转，要求最终地址与检测地址的主机名一致，且没有出现登录路径或账号密码表单。请将 `--check-url` 设为稳定、无需登录且不会跳往其他主机名的页面。已有重定向认证页面时优先使用该页面；未发现时使用 `--portal-url`。页面结构仍需包含原项目使用的 `#username`、`#password` 和对应登录按钮。

登录失败不会记录成功日志，持续运行时将在下轮重试。JSONL 保留 `timestamp`、`last_login`、`elapsed_min` 字段，兼容旧日志；缺失的间隔会按时间补算。程序启动时读取上一条成功记录，因此重启后仍能计算登录间隔。时间采用运行机器的本地时区；服务器上可设置 `TZ=Asia/Shanghai`。

## 开发

```bash
go test ./...
go test -race ./...
go vet ./...
```

可选浏览器集成测试使用本地模拟网关，不会访问真实校园网或使用 `.env` 中的凭据：

```bash
ECUST_TEST_CHROME=/path/to/chrome go test ./internal/portal -run TestBrowser -v
# 仅当测试环境需要关闭沙箱时添加 ECUST_TEST_NO_SANDBOX=1
```

```text
main.go                命令行入口
internal/cli/          Cobra 命令和参数
internal/config/       .env 与交互式凭据
internal/portal/       浏览器检测与两种登录页面
internal/runner/       定时检测与自动重连
internal/history/      兼容原格式的 JSONL 日志
internal/dashboard/    内嵌日志图表页面
```

## License

MIT License
