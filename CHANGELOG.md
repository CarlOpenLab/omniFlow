# Changelog

Notable changes per release. The release workflow lifts the section matching the
pushed tag straight into the GitHub release notes, so keep the headings exactly
`## <version>`.

## 0.2.2

**Mock responses are edited beside the capture, and capture waits for you to start it.** The Inspector now lists its mock rules in a panel beside the captured exchange; clicking a rule edits its status, content type and response body in the same window, and Mock This Response opens that editor at once. A mocked response replays the headers it was captured with, so CORS and the server's other response headers survive the replaced body. Opening OmniFlow no longer starts capture: the Inspector opens paused and the proxy starts only when you press Start Capture; the old "start the proxy when OmniFlow opens" setting is gone.
— Mock 响应可以直接在抓包旁边编辑，抓包也改为由你手动启动：Inspector 右侧新增「Mock 规则」列表，点击规则即可在同一窗口编辑状态码、Content-Type 与响应体，右键「Mock 此响应」会直接打开这个编辑器。Mock 响应会回放抓取时保存的响应头，替换正文后 CORS 等服务器约定仍然保留。打开 OmniFlow 不再自动开始抓包：Inspector 以暂停状态打开，只有点击「开始抓包」才会启动代理；原「打开 OmniFlow 时启动代理」设置已移除。

## 0.2.1

**Windows HTTPS capture works, and the proxy always gives the machine back.** The Windows trust check now asks the certificate store the way it expects, so installing the Root CA makes HTTPS capture start at once and removing its trust stops it at once, on every platform. Restoring the previous proxy now tries every network service and value even when one fails, each system command has its own five-second limit, and a failed takeover is rolled back with a fresh deadline instead of the one that just expired. On Linux the certificate dialog shows the live trust state and Check Again again. This release also carries the work that reached main after 0.2.0: Map Remote and header rules, response mocking, Slow 3G/Fast 3G/4G throttling, saving a captured exchange to the collection, body search in the traffic filter, a device setup page that installs the Root CA on a phone, one-click CA trust on Windows and macOS, LAN access, and window controls drawn on Windows and Linux.
— Windows 的 HTTPS 抓包恢复可用，系统代理也总能还给机器：Windows 的证书信任查询现在按 CryptoAPI 要求的方式传参，安装根证书后 HTTPS 抓包立即开始，移除信任后立即停止，所有平台一致。还原原有代理时会逐条尝试每个网络服务与设置值，单条失败不再中断其余；每条系统命令有独立的 5 秒上限，接管失败后的回滚使用新的超时，而不是刚过期的那个。Linux 的证书对话框恢复了实时信任状态与「重新检查」。本版同时包含 0.2.0 之后进入 main 的工作：Map Remote 与请求头规则、响应 Mock、慢速 3G/快速 3G/4G 限速、把抓到的交换存入集合、流量过滤器的正文搜索、给手机安装根证书的设备接入页、Windows 与 macOS 的一键信任根证书、局域网访问，以及 Windows 与 Linux 上自绘的窗口控件。

## 0.2.0

**Capture takes the machine's system proxy by itself.** Starting the proxy now points the system's HTTP and HTTPS proxies at OmniFlow on macOS, Windows and GNOME, so browsers and apps that follow the system proxy are captured without configuring anything. The settings in force before capture are saved to a backup file and restored when capture stops, when the app quits, and after a crash on the next launch; a second launch hands over to the running instance rather than touching the proxy. HTTPS is only routed through the proxy once the Root CA is known to be trusted — otherwise capture stays on HTTP, and the app checks again when you install it. Settings → Capture has a switch to turn this off, and says that Firefox keeps its own proxy settings.
— 抓包会自动接管系统代理：启动代理时，macOS、Windows 与 GNOME 的系统 HTTP/HTTPS 代理会指向 OmniFlow，跟随系统代理的浏览器和应用无需任何配置就会被抓取。抓包前的代理设置会存入备份文件，在停止抓包、退出应用以及崩溃后下次启动时还原；再次启动应用会交给已在运行的实例，不会改动代理。只有在确认根证书已被信任后才代理 HTTPS，否则只抓 HTTP；安装证书后应用会重新检查。设置 → 抓包页新增开关可关闭自动接管，并提示 Firefox 使用自己的代理设置。

## 0.1.2

**Settings is a window of pages, and OmniFlow can update itself.** Settings is now a 760×560 dialog with Appearance, Capture, Data, Updates and About down the left and the chosen page on a card of its own at the right, each page scrolling separately instead of every section stacked in one column. The new Updates page shows the version and the last check, switches automatic checks and automatic downloads, and offers **Check for Updates…**; a development build or a package-managed install says why it cannot update itself. Releases now carry signed update archives, so an installed app can move to this version and later ones in place.
— 设置改为分页窗口：760×560 的对话框中，左侧列出外观、抓包、数据、更新与关于，右侧单独显示所选页面，各页独立滚动，不再是所有设置堆叠在一列。新增的更新页显示版本与上次检查时间，可开关自动检查与自动下载，并提供 **检查更新…**；开发版或由包管理器安装的版本会说明为何不能自更新。发布包现在附带签名的更新归档，已安装的应用可以原地升级到本版本及后续版本。

## 0.1.1

**Bodies of any size scroll, stay colored and read whole.** Response and inspector bodies are drawn by a virtualized editor: the full body is shown rather than the first 1 MiB, scrolling reaches its last line, JSON keeps its colors at any size, and ⌘F finds in the editor's own bar — every match counted, typing no longer re-lays out the body. A **Wrap** switch in the body header folds long log lines at the panel width, so one screen shows the whole line.
— 任意大小的正文都能完整滚动与着色：Composer 与 Inspector 的正文改用虚拟化编辑器，完整显示而不是只显示前 1 MiB，可滚动到最后一行，JSON 在任意大小下保持着色；⌘F 使用编辑器自带的查找栏，匹配数不再设上限，输入时不再重排整份正文。正文头部新增 **换行** 开关，长日志按面板宽度折行，一屏即可读完。

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
