# Installation & Operations Guide

Everything needed to develop, deploy, and operate Gocord. The main
[README](README.md) is for end users; this document is for operators and
developers.

A Go signaling server with an embedded web frontend. The server never
proxies normal media: it creates ephemeral rooms, authenticates the two
browsers, and relays SDP/ICE signaling over WebSocket. Media flows
peer-to-peer; TURN relay is a normal ICE fallback. Both IPv4 and IPv6 are
supported — the server listens on a dual-stack socket by default.

The module pins `github.com/gorilla/websocket` v1.5.3.

## Contents

- [Requirements](#requirements)
- [Quick start (local)](#quick-start-local)
- [Local development](#local-development)
- [Building](#building)
- [Production DNS](#production-dns)
- [Application configuration](#application-configuration)
- [Reverse proxy (Caddy)](#reverse-proxy-caddy)
- [coturn (TURN/STUN)](#coturn-turnstun)
- [systemd service](#systemd-service)
- [Docker](#docker)
- [Verifying the deployment](#verifying-the-deployment)
- [Behavior under disconnection](#behavior-under-disconnection)
- [Call flow](#call-flow)
- [Security & privacy notes](#security--privacy-notes)
- [Tests](#tests)

## Requirements

- Go 1.23 or newer.
- A browser with WebRTC and camera/microphone support.
- HTTPS in production (browsers only grant camera/mic access on secure
  contexts; `localhost` is exempt during development).
- For production fallback: a coturn server reachable over the same address
  family as the participants.

## Quick start (local)

```bash
go mod download
go run ./cmd/server
```

Then open `http://localhost:8080`.

The default listener is:

```text
[::]:8080
```

## Local development

### Hot reloading (live dev server)

To automatically rebuild and restart on Go code and frontend (`web/`)
changes:

```bash
# Using the Makefile (ensures Air is available)
make dev

# Or directly with Air
air
```

### Loopback-only listener

```bash
LISTEN_ADDR='[::1]:8080' \
PUBLIC_BASE_URL='http://[::1]:8080' \
go run ./cmd/server
```

For realistic iPhone/Android testing, use HTTPS with a real hostname and
certificate.

## Building

```bash
go build -trimpath -ldflags='-s -w' -o ipv6-video-call ./cmd/server
```

The frontend is embedded in the binary through `web/embed.go`; the
production binary is self-contained.

## Production DNS

Publish A and/or AAAA records as appropriate for your deployment:

```text
call.example.com.  A     203.0.113.10
call.example.com.  AAAA  2001:db8:1234::10
turn.example.com.  A     203.0.113.20
turn.example.com.  AAAA  2001:db8:1234::20
```

The A/AAAA records exist because browsers only grant camera/microphone
access on secure (HTTPS) contexts, and trusted certificates are issued for
domain names. The domain must resolve to your server for both users and
the ACME certificate validation performed by the reverse proxy. Publish
both families when your server is dual-stack so IPv4-only and IPv6-only
clients can both connect.

## Application configuration

Copy `.env.example` to `/etc/ipv6-video-call.env` and edit it.

| Variable | Example | Notes |
|---|---|---|
| `LISTEN_ADDR` | `[::]:8080` | Dual-stack listener; use `[::1]:8080` for loopback-only. |
| `PUBLIC_BASE_URL` | `https://call.example.com` | Absolute base URL as seen by browsers. |
| `STUN_URLS` | `stun:turn.example.com:3478` | STUN servers used by ICE. |
| `TURN_URLS` | `turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp` | TURN fallback servers. |
| `TURN_SHARED_SECRET` | *(random)* | Same value as `static-auth-secret` in coturn. Never sent to browsers. |
| `MAX_ROOMS` | `5000` | Global cap on concurrent rooms. |
| `ROOM_CREATE_RATE` | `0.2` | Room creations per second per client (rate limiting). |
| `ROOM_CREATE_BURST` | `5` | Burst allowance for the same limiter. |
| `TRUST_PROXY_HEADERS` | `true` | See below. |
| `ROOM_TTL` | *(optional)* | Room lifetime; rooms are in-memory only. |
| `EMPTY_ROOM_GRACE` | *(optional)* | Reconnect window after the last peer leaves. |
| `DEBUG_SIGNALING` | *(optional)* | `1` enables debug logging; leave unset in production. |

Set `TRUST_PROXY_HEADERS=true` only when a trusted reverse proxy such as
Caddy fronts the server. It makes rate limiting use the last
`X-Forwarded-For` entry, which is the address the proxy itself observed.
With the server exposed directly, leave it false: any client could
otherwise forge the header and sidestep the limit.

### How TURN credentials work

`TURN_SHARED_SECRET` is never sent to browsers. `/api/config` creates a
temporary username of the form:

```text
<expiration-unix-time>:<random-id>
```

and returns a coturn-compatible credential:

```text
base64(HMAC-SHA1(shared-secret, temporary-username))
```

Set the same secret as `static-auth-secret` in coturn. A mismatch between
the two is the most common cause of "media never connects" symptoms.

## Reverse proxy (Caddy)

Edit `deploy/Caddyfile` and replace the hostname. The supplied
configuration proxies to the Go server on loopback:

```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Caddy handles HTTPS certificates (ACME) and WebSocket proxying
automatically.

## coturn (TURN/STUN)

Install coturn, copy `deploy/coturn.conf`, and replace:

- The listening/relay addresses with the TURN server's actual addresses
  (the shipped values are documentation-only placeholders).
- `turn.example.com` with your TURN hostname.
- `static-auth-secret` with the exact value used by the Go server.

Example:

```bash
sudo cp deploy/coturn.conf /etc/turnserver.conf
sudo systemctl restart coturn
```

Open the following firewall ports to the Internet:

```text
UDP 3478
TCP 3478
UDP 49160-49200
```

The small relay range is adequate for this private two-person application.
Increase it if you later support concurrent rooms.

Note: on a server whose global addresses are directly attached to the
network interface, `listening-ip`/`relay-ip` can be omitted entirely —
coturn then binds all system addresses. `external-ip` (public/private
mapping) is only required behind NAT.

## systemd service

Create a dedicated service user and install the binary:

```bash
sudo useradd --system --home /nonexistent --shell /usr/bin/nologin ipv6call
sudo mkdir -p /opt/ipv6-video-call
sudo install -m 0755 ipv6-video-call /opt/ipv6-video-call/ipv6-video-call
sudo cp deploy/ipv6-video-call.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ipv6-video-call
```

Helper scripts in `deploy/` (`setup_server.sh`, `deploy_to_oracle.sh`)
automate parts of a fresh-server deployment.

## Docker

A `Dockerfile` and `docker-compose.yml` are provided for container-based
deployments. Build and run with:

```bash
docker compose up -d --build
```

The same environment variables apply; pass them through the compose file
or an env file. Remember that the TURN shared secret must match whatever
coturn instance the deployment uses.

## Verifying the deployment

1. `curl -I https://call.example.com` — expect a redirect or 200 over
   valid TLS.
2. Create a room in the browser and open the invitation link in a second
   browser profile or device.
3. In the call's diagnostics panel, confirm:
   - `candidate type` is `srflx` or `host` for direct media, `relay` when
     TURN is in use;
   - the negotiated codec (H.264 expected on typical hardware);
   - bitrate/RTT are updating.
4. On the TURN host, `turnserver` logs will show allocations when relay
   fallback is exercised.

## Behavior under disconnection

Signaling and media fail independently, so they recover independently.

**Signaling socket drops.** Media is peer-to-peer and keeps flowing, so
the socket is reconnected in the background with exponential backoff
(1s doubling to 15s) plus jitter, so two peers that dropped together do
not retry in lockstep. The client rejoins with the same client ID, which
the server matches to the existing role. If media is still healthy on
rejoin, nothing is renegotiated: a signaling-only blip must not disturb a
working call.

**Media drops.** A `disconnected` state is given a short grace period,
because it commonly self-heals on network handover. If it has not
recovered, or the state went straight to `failed`, the caller issues an
ICE restart (`createOffer({ iceRestart: true })`) over the existing
socket. Only the caller does this; a restart from the callee would
collide with the caller's negotiation. If the socket is also down, the
restart waits and the rejoin re-offers instead.

**The peer drops.** The server keeps the room alive for
`EMPTY_ROOM_GRACE` after its last peer disconnects, so `peer-left` is
presented as "waiting for them to return" rather than as the end of the
call. When the peer rejoins, the server broadcasts `peer-ready` and the
caller renegotiates. After `PEER_RETURN_TIMEOUT_MS` with no return, the
call is reported as over.

**Failures that are not retried.** A deliberate `hangup` from the peer,
and join errors that retrying cannot fix (`room_not_found`,
`room_expired`, `unauthorized`, `room_full`, `at_capacity`), stop the
retry loop and surface the reason. Otherwise a deleted room would be
hammered by an endless reconnect loop.

Reconnecting relies on the client ID held in `sessionStorage`, so a page
reload rejoins as the same participant. A client that loses that ID
counts as a new participant and is refused, because the room's two roles
are already assigned.

## Call flow

```text
Browser A                    Go signaling                     Browser B
    |                              |                               |
    | POST /api/rooms              |                               |
    |----------------------------->|                               |
    | ROOM + #SECRET               |                               |
    |<-----------------------------|                               |
    |                              |                               |
    | WSS join + secret            |          WSS join + secret    |
    |----------------------------->|<------------------------------|
    |                              |                               |
    |            peer-ready        |          peer-ready           |
    |<-----------------------------|------------------------------>|
    |                              |                               |
    | SDP offer                    |                               |
    |----------------------------->|------------------------------>|
    |                              |                               |
    |                         SDP answer                           |
    |<-----------------------------|<------------------------------|
    |                              |                               |
    |====== WebRTC DTLS-SRTP over selected candidate pair ========|
```

## Security & privacy notes

At the default log level the server does not log room IDs, invitation
secrets, SDP, ICE candidates, IP addresses, camera metadata, microphone
metadata, or call duration.

Setting `DEBUG_SIGNALING=1` raises the level to debug, which **does** log
room IDs, client IDs, roles, and signaling message types so a failing call
can be traced. Invitation secrets, SDP bodies, ICE candidates, and IP
addresses are never logged at any level. Leave `DEBUG_SIGNALING` unset in
production.

Rooms are in memory only. They expire after `ROOM_TTL`. Once a room has
had a participant, it is removed after both participants have been
disconnected for `EMPTY_ROOM_GRACE`, which provides a small reconnect
window.

### Privacy in Web Push notifications

- **No central contact directory**: push subscriptions (`endpoint`,
  `p256dh`, `auth`) and contact names exist solely in the user's browser
  `localStorage`. The Go server never stores an address book or links
  push endpoints to user accounts.
- **End-to-end encrypted push payloads**: notifications are encrypted on
  the server according to RFC 8291 (`aes128gcm` ECDH P-256 HKDF) using
  the recipient's public key. Push delivery relays (Apple APNs, Google
  FCM, Mozilla Autopush) see only ciphertext.
- **Stateless push relay**: the `POST /api/push/notify` endpoint forwards
  encrypted payloads to the push service with VAPID authentication
  (RFC 8292) and discards the request. Neither the recipient endpoint
  nor payload is logged.
- **Ephemeral room secrets**: call invitation URLs include the `#SECRET`
  fragment. Because URL fragments are never sent to the server over HTTP
  requests, room secrets are transported directly to the recipient's
  browser inside the encrypted push payload.

### Codec strategy

H.264 is preferred first because it is hardware encoded on effectively
every device, giving the lowest encode latency and CPU cost. The
trade-off is compression: WebRTC negotiates constrained baseline H.264, so
there are no B-frames and quality per bit is weaker than VP9 or AV1.

The preference lives in `web/app.js` as `PREFERRED_VIDEO_CODEC`, applied
through `setCodecPreferences()` before the offer is created. VP8/VP9/AV1
stay behind it in the list, so a peer that cannot do H.264 still
negotiates successfully. AV1 gives the best quality per bit but only
encodes cheaply where the machine has a hardware AV1 encoder. The codec
that was actually negotiated is shown live in the call's metrics panel.

## Tests

```bash
go test ./...
```

The suite covers room capacity and stable reconnect identity; secret
rejection and empty-room cleanup; ICE candidate validation; removal of
invalid ICE lines from SDP; coturn REST credential generation; teardown
of a peer whose send buffer stalls; the room cap including reclaiming
expired rooms; room-creation rate limiting and forged
`X-Forwarded-For` handling; security headers (CSP allows no inline
script or style); and room survival across a peer reconnect inside the
grace window.
