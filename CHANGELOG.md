# Changelog

Notable changes per release. The release workflow lifts the section matching the
pushed tag straight into the GitHub release notes, so keep the headings exactly
`## <version>`.

## 0.1.0

**OmniFlow 0.1.0：一个窗口里的原生 HTTP 工作台 — A native HTTP workbench in one window**

- **A composer and a capture proxy share one window.** The composer sends requests with a method, URL and query parameters, headers, JSON, text or URL-encoded form bodies, and bearer or basic auth; responses show status, headers, a JSON-highlighted body, image previews, a saved binary body and a timing waterfall. The inspector is a local HTTP proxy that records every exchange.
  — 一个窗口里同时有 Composer 与抓包代理：支持方法、URL 与查询参数、请求头、JSON/文本/form 表单体、Bearer 与 Basic 认证；响应显示状态、响应头、JSON 高亮正文、图片预览、二进制保存与耗时瀑布图。Inspector 是记录每次交换的本地 HTTP 代理。
- **Environments, collections and history survive a restart.** Named sets of `{{variables}}` fill the URL, headers, body and auth at send time, and built-ins (`{{$timestamp}}`, `{{$isoTimestamp}}`, `{{$uuid}}`, `{{$randomInt}}`) are made up on each send. Saved requests keep edits as a draft until saved; history reopens the whole request as it was sent, and deletions offer Undo.
  — 环境、集合与历史在重启后仍在：命名变量组在发送时填充 URL、请求头、正文与认证；内置变量（`{{$timestamp}}`、`{{$isoTimestamp}}`、`{{$uuid}}`、`{{$randomInt}}`）每次发送时生成。已保存请求的修改先留在草稿里，保存后才写回；历史还原完整的已发送请求；删除后可在页脚撤销。
- **cURL goes in and out.** Paste a `curl` command into the URL field and it opens as the request it describes, with a note of any flag that could not be kept; ⇧⌘C copies the composer's request or a captured flow as a runnable curl command.
  — cURL 可以导入也可以导出：把 `curl` 命令粘进 URL 栏即可还原为请求，无法保留的参数会注明；⇧⌘C 把 Composer 的请求或抓到的 flow 复制成可直接运行的 curl 命令。
- **The inspector records, filters and replays.** Filter by host, text or status class, walk the capture with ↑ and ↓, right-click to replay, copy or show only a host, and export the whole capture as HAR 1.2. It keeps the latest 3,000 flows and up to 512 MB of bodies. HTTPS is decrypted through a root authority the app creates on this machine; install it from Settings only on devices you control.
  — Inspector 能记录、过滤与重放：按 host、文本或状态码过滤，↑/↓ 浏览抓包，右键重放、复制或只看某个 host，整份抓包导出为 HAR 1.2；保留最近 3,000 条与最多 512 MB 正文。HTTPS 通过本机生成的根证书解密；只在你控制的设备上从设置里安装。
- **The interface speaks English and 简体中文.** Follow the system language or choose one in Settings; the switch takes effect immediately, menus and dialogs included. Appearance, card opacity and material, request timeout, redirect and TLS verification behaviour, proxy port, autostart and the data folder are all in Settings.
  — 界面支持 English 与简体中文：可跟随系统，也可在设置里选择，切换立即生效，菜单与对话框一并切换。外观、卡片透明度与材质、请求超时、是否跟随重定向与校验 TLS、代理端口、开机启动与数据目录都在设置里。
- **Installers for macOS, Windows and Linux.** macOS gets a universal DMG (Apple silicon and Intel), Windows a Setup installer and a portable ZIP, Linux amd64/arm64 Debian packages and tarballs whose `install.sh` installs to `~/.local`. Release assets are currently self-signed: on macOS right-click the app and choose **Open** the first time; on Windows click **More info → Run anyway** if SmartScreen appears.
  — 提供 macOS、Windows 与 Linux 安装包：macOS 为 Universal DMG（Apple silicon 与 Intel），Windows 为 Setup 安装包与便携 ZIP，Linux amd64/arm64 为 Debian 包与带 `install.sh`（安装到 `~/.local`）的 tar.gz。发布资产目前为自签名：macOS 首次打开请右键选择 **Open**；Windows 如遇 SmartScreen 请点 **More info → Run anyway**。
