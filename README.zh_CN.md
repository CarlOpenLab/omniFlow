# OmniFlow

[English](README.md) · [简体中文](README.zh_CN.md)

macOS、Windows 与 Linux 上的原生 HTTP 工作台：Postman 风格的 Composer 与
Proxyman 风格的抓包代理同处一个窗口。界面由
[Mygo](https://github.com/egoist/mygo) 在 GPU 上绘制——没有 webview——macOS 上
窗口的亚克力是系统自带的材质，从视图中透出。

快捷键按 macOS 的写法（⌘）书写；Windows 与 Linux 上用 Ctrl 代替。

## 下载

每个版本的安装包都在
[Releases 页面](https://github.com/CarlOpenLab/omniFlow/releases)：

| 平台 | 文件 | 说明 |
| --- | --- | --- |
| macOS | `omniflow_<version>_macos_universal.dmg` | Universal，Apple silicon 与 Intel 通用。把 OmniFlow 拖进「应用程序」。 |
| Windows | `omniflow_<version>_windows_amd64_setup.exe` | 安装程序；`omniflow_<version>_windows_amd64_portable.zip` 可在任意文件夹直接运行。 |
| Linux | `omniflow_<version>_linux_amd64.deb`、`omniflow_<version>_linux_arm64.deb` | Debian 包；`.tar.gz` 版本带 `install.sh`，安装到 `~/.local`。 |

发布资产目前为自签名。macOS 首次打开请右键 OmniFlow 并选择 **Open**；Windows
如遇 SmartScreen 请点 **More info → Run anyway**。每个版本还附带
`checksums.txt`。

## 功能

- **Composer** — 支持请求方法、URL、查询参数、请求头、JSON 或文本正文（带实时
  JSON 检查）、以表格编辑的 URL 编码表单正文，以及 Bearer 或 Basic 认证。响应
  显示状态、响应头、带 JSON 高亮的正文、图片预览（其他二进制正文可保存），以及
  DNS、连接、TLS、等待与下载的瀑布图。⌘. 取消进行中的请求；⌘F 在当前正文中
  查找。
- **环境** — 命名的 `{{变量}}` 集合（`{{host}}`、`{{token}}`），在侧栏选择
  （⌘E 管理）。发送时在 URL、请求头、正文与认证中填充，并在书写处高亮。
  `{{$timestamp}}`、`{{$isoTimestamp}}`、`{{$uuid}}` 与 `{{$randomInt}}` 每次
  发送时生成。没有变量能填充的名字会在提示里说明，而不是把花括号发出去。
- **集合** — 双击请求名可重命名。对已保存请求的修改会作为草稿保留，侧栏标出，
  直到 ⌘S 保存；右键请求可复制、拷贝或删除。历史会重新打开当时发出的完整请求。
  抓到的请求也可以直接从 Inspector 存入集合。删除、清空历史或会话后，页脚会
  提供几秒钟的撤销。
- **cURL 导入** — 把 `curl` 命令粘进 URL 栏，它会按命令描述的请求打开，无法保留
  的参数会注明。
- **Inspector（抓包）** — 本地 HTTP 代理，记录每次交换；按 host、文本或状态码
  分类过滤（点击 STATUS 列标题），并在 Composer 中重放抓到的请求。↑ 与 ↓ 在
  抓包中移动；右键 flow 可重放、复制或只看它的 host。过滤器旁的 **正文** 按钮
  会同时搜索请求与响应正文。右键一个抓到的响应并选择 **Mock 此响应**，代理就会
  用你编辑的本地文件回应相同的 method、host 与 path；设置 → Mock 规则 可开关或
  删除规则。整份抓包可导出为 HAR 1.2 文件。最多保留最近 3,000 条 flow 与
  512 MB 正文；超过任一条，最旧的先走。HTTPS 通过应用在本机创建的根证书解密；
  从设置 → Root CA Certificate 安装（仅限你控制的设备）。
- **Copy as cURL** — ⇧⌘C 把 Composer 的请求或抓到的 flow 复制成发送同样请求的
  curl 命令。
- **设置** — 系统/浅色/深色外观（侧栏底部也可切换）、界面语言（跟随系统、
  English 或简体中文）、卡片透明度（30–100%）与材质强度（无、轻微、中等、深）、
  请求超时、是否跟随重定向与校验 TLS 证书、代理端口、开机启动、HTTPS 解密、
  历史记录与数据目录。

## 构建与运行

需要 Go 1.27。应用可在 macOS、Windows 与 Linux 上构建；签名与磁盘映像需要
macOS。

```sh
# 在任意平台从源码运行
go run ./cmd/omniflow

# 测试与原生界面检查
go test ./...
go tool mygo vet .

# 为当前机器打包（macOS：.app 与 .dmg）
go tool mygo build .

# 一次构建所有平台的安装包
go tool mygo build -platform darwin/universal,linux/amd64,linux/arm64,windows/amd64
```

在 macOS 上，`scripts/dev.sh` 与 `scripts/build.sh` 会用钥匙串里第一个
`Apple Development` 身份签名（可用 `MACOS_SIGNING_IDENTITY` 覆盖），让权限授权
在重新构建后仍然保留。发布构建为自签名。

保存的数据位于应用的用户数据目录：macOS 为
`~/Library/Application Support/OmniFlow`，Windows 为 `%AppData%\OmniFlow`，
Linux 为 `~/.config/OmniFlow`（或 `$XDG_CONFIG_HOME/OmniFlow`）。抓包证书的
私钥以 `0600` 写入，除非导出公开证书，否则不会离开该目录。

让客户端走代理，例如（证书路径以 macOS 为例；其他平台使用上面的用户数据
目录）：

```sh
curl -x http://127.0.0.1:9090 http://example.com/
curl -x http://127.0.0.1:9090 \
  --cacert "$HOME/Library/Application Support/OmniFlow/omniflow-ca.pem" \
  https://example.com/
```
