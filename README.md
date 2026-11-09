# OmniFlow

[English](README.md) · [简体中文](README.zh_CN.md)

A native HTTP workbench for macOS, Windows and Linux: a Postman-style
composer and a Proxyman-style capture proxy in one window. The interface is
drawn by [Mygo](https://github.com/egoist/mygo) on the GPU — there is no
webview — and on macOS the window's acrylic is the system's own material
showing through the view.

Shortcuts are written the way macOS writes them (⌘); on Windows and Linux,
use Ctrl in its place.

## Download

Installers for every release are on the
[Releases page](https://github.com/CarlOpenLab/omniFlow/releases):

| Platform | File | Notes |
| --- | --- | --- |
| macOS | `omniflow_<version>_macos_universal.dmg` | Universal, Apple silicon and Intel. Drag OmniFlow into Applications. |
| Windows | `omniflow_<version>_windows_amd64_setup.exe` | Installer; `omniflow_<version>_windows_amd64_portable.zip` runs from any folder. |
| Linux | `omniflow_<version>_linux_amd64.deb`, `omniflow_<version>_linux_arm64.deb` | Debian packages; the `.tar.gz` builds ship `install.sh` for `~/.local`. |

Release assets are currently self-signed. On macOS, right-click OmniFlow and
choose **Open** the first time. On Windows, click **More info → Run anyway**
if SmartScreen appears. Each release also carries a `checksums.txt`.

## What it does

- **Composer** — request method, URL, query parameters, headers, JSON or text
  bodies (with a live JSON check), URL-encoded form bodies edited as a table,
  and bearer or basic auth. Responses show status, headers, body with JSON
  highlighting, image previews (other binary bodies can be saved), and a
  waterfall of DNS, connect, TLS, waiting and download. ⌘. cancels a request
  in flight; ⌘F finds text in the body on screen.
- **Environments** — named sets of `{{variables}}` (`{{host}}`, `{{token}}`),
  chosen in the sidebar (⌘E to manage). They are filled in across the URL,
  headers, body and auth as a request is sent, and highlighted where they are
  written. `{{$timestamp}}`, `{{$isoTimestamp}}`, `{{$uuid}}` and
  `{{$randomInt}}` are made up on each send. A name nothing fills in is named
  in the notice rather than sent as braces.
- **Collection** — double-click a request's name to rename it. Edits to a
  saved request stay as a draft, marked in the sidebar, until ⌘S saves them;
  right-click a request to duplicate, copy or delete it. History reopens the
  whole request as it was sent. A captured exchange can be saved straight into
  the collection from the Inspector. Deletions and a cleared history or session
  offer Undo in the footer for a few seconds.
- **cURL import** — paste a `curl` command into the URL field and it opens as
  the request it describes, with a note of any flag that could not be kept.
- **Inspector** — a local HTTP proxy that records every exchange, filters by
  host, text or status class (click the STATUS column's heading), and replays
  a captured request in the composer. ↑ and ↓ move through the capture;
  right-click a flow to replay it, copy it or show only its host. The filter's
  **Bodies** button searches request and response bodies too. Right-click a
  captured response and choose **Mock This Response** to answer that method,
  host and path from a file you can edit; Settings → Mocks switches the rules
  off or removes them. The whole capture exports as a HAR 1.2 file. It keeps
  the latest 3,000 flows and up to 512 MB of bodies; past either, the oldest go
  first. HTTPS is decrypted through a root authority the app creates on this
  machine; install it from Settings → Root CA Certificate (only on devices you
  control). While the proxy runs, OmniFlow also takes the machine's system
  proxy, so browsers and apps that follow it are captured without any setup.
  Settings → Capture turns that off; Firefox keeps its own proxy settings and
  is not captured until they are pointed at the proxy.
- **Rules and throttling** — right-click a captured request and open **Rules**
  to map it to another origin, or to add, set or remove one request or
  response header; Settings → Rules lists, edits and switches the rules. The
  Inspector's **Throttle** menu simulates Slow 3G, Fast 3G or 4G, with latency
  and a bandwidth cap.
- **Device setup and trust** — Settings → Capture can expose the proxy to
  devices on the local network; **Device Setup** in the Inspector's sidebar
  then shows the proxy address and a small page that installs the Root CA on
  an iPhone, iPad or Android device. The certificate dialog can trust or remove
  the Root CA for the current user on Windows and macOS.
- **Copy as cURL** — ⇧⌘C copies the composer's request, or the captured
  flow, as a curl command that sends the same request.
- **Settings** — system/light/dark appearance (also in the sidebar's foot),
  the interface language (follow the system, English or 简体中文), the card's
  opacity (30–100%) and the material's strength (none, subtle, medium, deep),
  the request timeout, whether redirects are followed and whether TLS
  certificates are verified, proxy port, autostart, automatic system proxy,
  HTTPS decryption, the rules, history and the data folder.

## Build and run

Requires Go 1.27. The app builds on macOS, Windows and Linux; signing and
disk images need macOS.

```sh
# Run from source, on any platform
go run ./cmd/omniflow

# Tests and the native-UI checks
go test ./...
go tool mygo vet .

# A package for the machine you are on (macOS: .app and .dmg)
go tool mygo build .

# Installers for every platform
go tool mygo build -platform darwin/universal,linux/amd64,linux/arm64,windows/amd64
```

On macOS, `scripts/dev.sh` and `scripts/build.sh` sign with the first
`Apple Development` identity in your keychain (override with
`MACOS_SIGNING_IDENTITY`), so permission grants survive rebuilds. Release
builds are self-signed.

Saved data lives in the app's user-data directory:
`~/Library/Application Support/OmniFlow` on macOS, `%AppData%\OmniFlow` on
Windows, and `~/.config/OmniFlow` (or `$XDG_CONFIG_HOME/OmniFlow`) on Linux.
The capture certificate's private key is written `0600` and never leaves that
directory unless you export the public certificate.

While OmniFlow's proxy is running, the machine's system proxy already points
at it: open a browser and its traffic appears in the Inspector. The previous
proxy settings are put back when capture stops or OmniFlow quits, including
after a crash. If capturing a browser or app running on another device, point
it at the proxy by hand, for example (the certificate path shown for macOS;
use the user-data directory above elsewhere). Firefox uses its own proxy
settings, so browser traffic is not captured until they are set the same way.

```sh
curl -x http://127.0.0.1:9090 http://example.com/
curl -x http://127.0.0.1:9090 \
  --cacert "$HOME/Library/Application Support/OmniFlow/omniflow-ca.pem" \
  https://example.com/
```
