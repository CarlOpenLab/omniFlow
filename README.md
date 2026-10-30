# OmniFlow

A native HTTP workbench for macOS: a Postman-style composer and a
Proxyman-style capture proxy in one window. The interface is drawn by
[Mygo](https://github.com/egoist/mygo) on the GPU — there is no webview — and
the window's acrylic is the system's own material showing through the view.

## What it does

- **Composer** — request method, URL, query parameters, headers, JSON or text
  bodies (with a live JSON check), URL-encoded form bodies edited as a table,
  and bearer or basic auth. Responses show status, headers, body with JSON
  highlighting, image previews (other binary bodies can be saved), and a
  waterfall of DNS, connect, TLS, waiting and download. ⌘. cancels a request
  in flight.
- **Environments** — named sets of `{{variables}}` (`{{host}}`, `{{token}}`),
  chosen in the sidebar (⌘E to manage). They are filled in across the URL,
  headers, body and auth as a request is sent, and highlighted where they are
  written. `{{$timestamp}}`, `{{$isoTimestamp}}`, `{{$uuid}}` and
  `{{$randomInt}}` are made up on each send. A name nothing fills in is named
  in the notice rather than sent as braces.
- **Collection** — double-click a request's name to rename it. Edits to a
  saved request stay as a draft, marked in the sidebar, until ⌘S saves them;
  right-click a request to duplicate, copy or delete it. History reopens the
  whole request as it was sent. Deletions and a cleared history or session
  offer Undo in the footer for a few seconds.
- **cURL import** — paste a `curl` command into the URL field and it opens as
  the request it describes, with a note of any flag that could not be kept.
- **Inspector** — a local HTTP proxy that records every exchange, filters by
  host, text or status class (click the STATUS column's heading), and replays
  a captured request in the composer. ↑ and ↓ move through the capture;
  right-click a flow to replay it, copy it or show only its host. The whole
  capture exports as a HAR 1.2 file. It keeps the latest 3,000 flows and up
  to 512 MB of bodies; past either, the oldest go first. HTTPS is decrypted through a root authority the app creates on
  this machine; install it from Settings → Root CA Certificate (only on
  devices you control).
- **Copy as cURL** — ⇧⌘C copies the composer's request, or the captured
  flow, as a curl command that sends the same request.
- **Settings** — system/light/dark appearance (also in the sidebar's foot),
  the card's opacity (30–100%) and the material's strength (none, subtle,
  medium, deep), the request timeout, whether redirects are followed and
  whether TLS certificates are verified, proxy port, autostart, HTTPS
  decryption, history and the data folder.

## Build and run

Requires Go 1.27 and macOS.

```sh
# Development window
go run ./cmd/omniflow

# Tests and the native-UI checks
go test ./...
go tool mygo vet .

# Packaged app bundle, ad-hoc signed, without a disk image
go tool mygo build -skip-dmg -o out .
open out/darwin-arm64/OmniFlow.app
```

The capture certificate and saved data live under
`~/Library/Application Support/OmniFlow/`. The private key is written `0600`
and never leaves that directory unless you export the public certificate.

Point a client at the proxy with, for example:

```sh
curl -x http://127.0.0.1:9090 http://example.com/
curl -x http://127.0.0.1:9090 \
  --cacert "$HOME/Library/Application Support/OmniFlow/omniflow-ca.pem" \
  https://example.com/
```
