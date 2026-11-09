# OmniFlow 自动配置系统代理设计

## 意图与原因

用户打开 OmniFlow 后想直接抓浏览器等本机应用的包。现在必须手动把系统代理
指到 `127.0.0.1:9090`；根证书只解决 HTTPS 解密后的信任问题，不负责把流量
送进代理。Charles/Proxyman 的「零配置」来自软件自动修改系统代理。OmniFlow
应补上这一步：抓包启动时自动设置，停止或退出时恢复，默认开启。

## 目标

1. 抓包运行且开关开启时，自动把本机 HTTP/HTTPS 系统代理指向 OmniFlow 的
   监听地址 `127.0.0.1:<port>`。
2. 停止抓包、关闭自动代理开关、退出应用时，把系统代理恢复为设置前的原样，
   包括 PAC / 自动发现 / 原服务器与端口。
3. 异常退出或崩溃后，下次启动先恢复上次留下的备份，再启动代理。
4. 设置 → 抓包页新增「自动配置系统代理」开关，默认开，随 Prefs 持久化。
5. HTTPS 代理仅在根证书已信任时启用；HTTP 始终启用。CA 未信任时 HTTPS
   保持直连、不被破坏，界面提示安装根证书。
6. macOS、Windows、Linux 均实现；不引入常驻 helper、系统扩展或内核扩展。
7. 发布 0.2.0。

## 非目标

- 不抓不跟随系统代理的客户端：Firefox（默认）、部分自建网络栈的 App、
  设置了自己代理的工具。设置页给出说明。
- 不做透明代理 / pf 重定向 / pcap / Network Extension。
- 不自动安装或信任根证书；仍由用户在设置里安装一次。
- 不修改 SOCKS / FTP 代理，不改 bypass / 忽略列表。
- 不保证服务型进程（Windows WinHTTP、系统服务）被捕获。

## 反目标

- 退出后系统代理没有恢复，导致全机断网。
- 崩溃后留下指向死端口的代理而不恢复。
- 未信任 CA 时自动打开 HTTPS 代理/解密，导致所有 HTTPS 失败。
- 覆盖用户原有代理配置后无法还原，包括 PAC、端口、bypass。

## 现有基础（已核实）

- `Proxy.Start` 监听 `127.0.0.1:<port>`；`ProxyAutostart` 默认 true；
  `DecryptHTTPS` 默认 true（`internal/model/model.go:222-229`）。
- `startProxy` / `stopProxy` / `restartProxy` / `toggleProxy`
  （`cmd/omniflow/app.go:915-991`）；退出时 `main.go:41-45` 直接
  `a.prx.Stop()`。
- `trust_darwin.go` 用 `security verify-cert -q -c <cert> -p basic` 查信任；
  `trust_other.go` 返回未知。
- 设置页开关用 `settingsSwitchField`（`cmd/omniflow/settings.go:343`）；
  catalog 源文件为 `locales/en.json`、`locales/zh-Hans.json`，
  `go tool mygo generate` 生成 `locales/catalog_gen.go`。
- macOS 上 admin 用户可直接 `networksetup` 改代理，无需 sudo（本机已验证：
  set / get / 清空均 exit 0，测试服务已恢复原状）。
- 发布流程：改 `mygo.json` version + `CHANGELOG.md` 的 `## <version>`，
  提交 main，打 `vX.Y.Z` tag 推送，`.github/workflows/release.yml`
  构建三平台并发布。

## 方案

### 新包 `internal/sysproxy`

- 文件：`sysproxy.go`（公共逻辑）、`darwin.go`、`windows.go`、`linux.go`、
  `other.go`，按 build tag 区分。
- API：

  ```go
  type Manager struct { dir string; mu sync.Mutex; active bool }
  func New(dir string) *Manager
  func (m *Manager) Enable(port int, captureHTTPS bool) error
  func (m *Manager) Restore() error
  func (m *Manager) Recover() error
  func (m *Manager) Active() bool
  ```

- `Enable` 先读取当前状态并原子写入备份文件
  `<dataDir>/system-proxy-backup.json`（0600，temp + rename），再应用；
  成功则 `active = true`。
- `Enable` 在已经 active 时必须幂等：只按新的端口 / `captureHTTPS` 重新应用，
  **不得重新快照**，否则会把自己设置的代理当成“原状态”，恢复后仍指向自己。
  端口变化走 stop → start，由恢复后的原始状态重新快照。
- **同平台的既有备份永远优先**：只要备份文件存在（即使本进程 `active=false`），
  `Enable` 就用它作为“原状态”，绝不覆盖。这覆盖两种情况：上次崩溃后恢复失败、
  以及本进程恢复后又重新开启。备份平台与当前平台不同则改名为 `.stale` 后继续。
- 启动时 `Recover` 失败：保留备份，照常允许代理监听，但 `Enable` 仍以旧备份
  为原状态；界面显示一条警告。绝不因为恢复失败而重新快照。
- 备份损坏：改名为 `.corrupt`，**本次会话不自动改动系统代理**（避免把可能是
  自己留下的死端口配置再次当成原状态），显示警告让用户检查系统设置；
  用户显式关闭再打开开关时，才以当时状态重新快照。
- `Restore` 从备份恢复，所有条目都尝试后删除备份；失败保留备份并返回错误，
  下次启动由 `Recover` 重试。
- `Recover` 在启动时发现遗留备份就直接恢复；恢复成功删除备份。
- 所有外部命令带 5 秒 context 超时；单条失败不中断其余服务的恢复，
  最后汇总错误。
- **失败语义**：
  - 快照读取失败：整次 `Enable` 放弃，不写备份、不改任何设置，返回错误。
  - 应用过程中部分失败：立即用刚写的备份做一次完整恢复；恢复成功则删除备份、
    `active=false`、返回错误；恢复失败则保留备份、`active=true`、返回错误，
    停止/退出时继续重试恢复。
  - `stopProxy` 只要备份存在就调用 `Restore`，与 `active` 无关。
- 平台实现通过一个可注入的命令执行器测试，命令构造与输出解析不直接依赖
  真实系统。

### macOS

- 服务：`networksetup -listallnetworkservices`，跳过首行说明和 `*` 开头的
  停用服务，对所有启用服务生效。
- **跳过使用认证代理的服务**：`-getwebproxy` / `-getsecurewebproxy` 报告
  `Authenticated Proxy Enabled: 1` 的服务原样不动，并在结果里注明；
  凭据在钥匙串里，`networksetup` 无法安全地快照与还原。全部服务都被跳过时
  `Enable` 返回提示，不声称成功。
- 快照每个服务：`-getwebproxy`、`-getsecurewebproxy`、`-getautoproxyurl`、
  `-getproxyautodiscovery` 的 enabled / server / port / url
  （认证标志只用于判断是否跳过）。
- 应用：
  1. 关闭 `-setautoproxyurl` 与 `-setproxyautodiscovery`，避免它们优先于
     手动代理；
  2. `-setwebproxy <svc> 127.0.0.1 <port>`；
  3. `captureHTTPS` 时 `-setsecurewebproxy <svc> 127.0.0.1 <port>`，
     否则 `-setsecurewebproxystate <svc> off`（防止沿用旧 HTTPS 代理）。
- 恢复：按快照恢复 PAC 与自动发现；server 非空则
  `-setwebproxy` / `-setsecurewebproxy` 后按原 enabled 状态开关；server 为空
  则 `-setwebproxy <svc> "" 0` 再按原状态开关（本机已验证该写法可清空）。
- 不调用 sudo；失败时返回错误，UI 提示「无法修改系统代理，请检查权限」。

### Windows

- `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`：
  - 快照 `ProxyEnable`、`ProxyServer`、`ProxyOverride`、`AutoConfigURL`、
    `AutoDetect`（包括“值不存在”状态）。
  - 应用：清空 `AutoConfigURL`、`AutoDetect=0`、
    `ProxyServer=http=127.0.0.1:<port>;https=127.0.0.1:<port>`
    （`captureHTTPS=false` 时只有 http）、`ProxyEnable=1`，保留
    `ProxyOverride`。
  - 用 `InternetSetOptionW(INTERNET_OPTION_SETTINGS_CHANGED /
    INTERNET_OPTION_REFRESH)` 通知 WinINet。
  - 恢复：按快照写回或删除值，并再次通知。
- 依赖 `golang.org/x/sys/windows/registry`；per-user 修改不需要管理员。

### Linux

- GNOME / gsettings（`org.gnome.system.proxy`）：快照 mode、http host/port、
  https host/port、`use-same-proxy`；应用 `mode='manual'`、http/https host/port、
  `use-same-proxy=false`（否则默认值 true 会让 HTTP 代理同时接管 HTTPS，
  `captureHTTPS=false` 时仍会把 HTTPS 送进未信任的代理）；恢复全部写回。
- `captureHTTPS=false` 时 https host 为空、port 0。
- `gsettings` 不存在时返回「当前桌面环境不支持自动设置系统代理」，
  抓包本身不受影响。
- 不修改 `ignore-hosts`。

### HTTPS 信任门控

- `captureHTTPS = MITM 开启 && 根证书已信任（只有 trustYes）`。
  `trustUnknown` 与 `trustNo` 一样按未信任处理：不启用系统 HTTPS 代理，
  避免把所有 HTTPS 打断。
- 检查必须在每次 `Enable` 与 CA 对话框「重新检查」时**实时**执行，
  不使用进程内缓存：
  - macOS：保持 `security verify-cert -q -c <cert> -p basic`。
  - Windows：用 CryptoAPI 打开当前用户与本地机器的 Root 证书存储
    （`CertOpenStore` + `CertFindCertificateInStore`，按 SHA-1 查找），
    任一命中即已信任；Go 1.27 的 `x509.SystemCertPool()` 在 Windows 上是
    不透明验证器，不能用于判断。
  - Linux：每次读取系统 CA bundle（Debian/Ubuntu 的
    `/etc/ssl/certs/ca-certificates.crt`；RHEL 系遍历 `/etc/ssl/certs/*.pem`），
    检查本 CA 的 DER 是否在其中；无法确定系统 CA 位置时返回 unknown。
    只在 NSS（浏览器私有库）里安装的 CA 不算系统信任，这是已知限制。
- 若未信任：只设 HTTP。Inspector 与设置页提示安装根证书。
- CA 对话框的「重新检查」得到 trustYes 后，若抓包运行中则重新同步系统代理，
  把 HTTPS 补上。
- 不因为 `captureHTTPS=false` 强制关闭代理的 MITM：显式把客户端指到代理的
  用户可能自行信任了证书；证书被拒时现有的 flow 错误提示仍会说明原因。

### 应用生命周期

- `main` 最开头调用 `mygo.App.RequestSingleInstanceLock()`；拿不到锁就退出，
  用 `OnSecondInstance` 聚焦已有窗口。第二个实例因此不会执行 `Recover`，
  不会把第一个实例正在使用的系统代理恢复掉，也不会与备份文件竞争。
- app 增加 `autoProxy bool`（由 Prefs 派生）与
  `sysproxy *sysproxy.Manager`（用 store 目录初始化）。
- `applyPreferences` / 启动：先 `Recover()`，再按 `ProxyAutostart` 启动代理。
- `startProxy` 成功后调用 `syncSystemProxy()`；`stopProxy` 先 `Restore()`；
  `restartProxy` 走 stop → start，端口变化会先恢复旧端口再设新端口。
- 退出：`main.go` 的 OnClosed 与 Run 之后统一走 `a.stopProxy()`（或显式
  `Restore()` 后再 `Stop()`），确保恢复。
- 开关切换：保存 Prefs；若抓包运行中，立即 Enable / Restore。MITM 开关变化
  同样重新同步。
- 若代理未运行、自动代理开：不设置系统代理，避免指向死端口。

### 设置 UI

- 抓包页新增开关（`settingsSwitchField`）：
  - `settingsSystemProxy` = "Set the system proxy automatically" /
    "自动配置系统代理"
- 开关下方一行小字 hint：
  - `settingsSystemProxyHint` = "Browsers and apps that follow the system
    proxy are captured. Firefox uses its own proxy settings." /
    "跟随系统代理的浏览器和应用会自动被抓取；Firefox 使用自己的代理设置。"
- switch 绑定 `a.autoProxy`；保存时写
  `Prefs.DisableAutoProxy = !a.autoProxy`（零值 = 开，兼容旧文件）。
- 设置页在渲染后比较开关的前后值，变化时调用 `syncSystemProxy()`；
  `checkTrust` 从 no/unknown 变为 yes 后也调用一次。

### 备份文件格式

```json
{
  "version": 1,
  "platform": "darwin",
  "createdAt": "2026-11-09T12:00:00Z",
  "data": { }
}
```

- `data` 为平台自己的 JSON。
- 备份中的 `platform` 与当前平台不一致时不恢复（跨平台备份无法还原）。

### 测试

- `internal/sysproxy` 用注入的 fake runner：
  - darwin：多服务快照 → 应用 → 恢复命令序列；跳过停用与认证代理服务；
    HTTP-only；空 server 恢复；备份先写、恢复后删除；单条失败仍恢复其他服务；
    已有备份时 `Enable` 不重新快照。
  - linux：`use-same-proxy` 的快照 / 设为 false / 恢复。
  - 通用：备份原子写、`Recover` 恢复并删除、恢复失败保留备份、
    损坏备份改名且不阻塞启动、部分应用失败触发回滚。
  - windows / linux 中不依赖 OS 的构造逻辑（注册表值、gsettings 命令、
    证书哈希）抽成纯函数测试。
- app：开关切换保存 Prefs；startProxy 成功后调用 manager（接口注入 fake）；
  stopProxy 先 Restore；单实例锁拿不到时不创建 app。
- 全量 `go test ./...`、`go tool mygo vet .`。
- 交叉编译检查 `GOOS=windows go build ./internal/sysproxy`、
  `GOOS=linux go build ./internal/sysproxy`。

### 文档与发布

- `README.md` / `README.zh_CN.md` 的「让客户端走代理」改为：自动接管，
  并保留手动 / 其他设备示例；补充 Firefox 例外。
- `mygo.json` 从 0.1.2 改为 0.2.0。
- `CHANGELOG.md` 新增 `## 0.2.0`，中英双段，风格与现有条目一致。
- 提交 main（本仓库不用分支），打 tag `v0.2.0` 推送，等 release.yml
  三平台构建完成，验证 GitHub Release 资产与更新清单。

## 我做出的决定（请检查）

1. 默认开启自动代理：打开 OmniFlow = 启动代理 + 接管系统代理。
2. 对所有启用的网络服务生效，而不只是当前活动服务，覆盖 Wi-Fi / Ethernet
   切换。
3. 抓包期间关闭 PAC / 自动发现，结束后还原。
4. HTTPS 代理只在根证书**确定已信任**时启用；unknown 一律按未信任处理，
   避免首次使用把所有 HTTPS 打断。
5. 不添加 sudo / helper；macOS admin 用户直接调 `networksetup`（已本机验证）。
6. 版本 0.2.0。
7. Firefox 等例外只提示，不代为配置；侧栏不加额外状态行。
8. 已有备份永远优先、绝不重新快照；恢复失败也不覆盖备份。
9. 加入单实例锁，第二个进程不会碰系统代理与备份。
10. macOS 上使用认证代理的网络服务原样跳过，不动凭据。
11. 部分应用失败时立即尝试回滚；回滚失败则保留备份供下次恢复。
12. 备份损坏时不自动改系统代理，只警告，等用户显式重开开关。

## 留给构建者

- 具体命令输出解析、注册表 / syscall 细节、gsettings key 细节；
- 错误与提示文案的最终措辞；
- 测试文件组织与 fake runner 的具体形状。

## 本次使用到的工件

- 无 artboard / HTML；需求来自 2026-11-09 的会话与现有 README。
