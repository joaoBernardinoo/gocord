<div align="right">
  <strong>Languages:</strong>
  <a href="README.md">English</a> |
  <a href="README.pt-BR.md">Português (Brasil)</a>
</div>

<p align="center">
  <img src="animation.gif" alt="Gocord Animation" width="100%">
</p>

# Browser Video Call

A two-person browser video call with a small Go signaling server and native WebRTC media.

The Go server never proxies normal media. It creates ephemeral rooms, authenticates the two browsers, and relays SDP/ICE signaling over WebSocket. Both IPv4 and IPv6 are supported; the server listens on a dual-stack socket by default.

## What is implemented

- Go `net/http` server listening on a dual-stack socket (IPv4 and IPv6).
- Embedded HTML/CSS/JavaScript frontend; the production binary is self-contained.
- Two-person in-memory rooms with cryptographically random room IDs and 256-bit secrets.
- Invitation URL format: `/join/ROOM#SECRET`.
- Only the hash of the room secret is stored server-side.
- WebSocket signaling for `join`, `offer`, `answer`, `ice-candidate`, `peer-ready`, `peer-left`, `hangup`, `ping`, and `pong`.
- ICE candidate validation in both browser and Go code.
- SDP candidate-line sanitization so malformed candidates are not forwarded.
- Direct peer-to-peer media first; TURN relay is a normal ICE fallback.
- Temporary coturn REST credentials generated with HMAC-SHA1 from a server-side shared secret.
- H.264-first video codec preference, applied through `setCodecPreferences()`
  before the offer is created. VP8/VP9/AV1 stay behind it in the list, so a peer
  that cannot do H.264 still negotiates successfully.
- Quality, Auto, and Low Bandwidth sender profiles.
- Per-client rate limiting and a global room cap on the unauthenticated
  room-creation endpoint.
- Native WebRTC congestion control remains active in every profile.
- Camera enable/disable, microphone mute, front/rear camera switching, fullscreen, and hangup.
- Recovery from dropped signaling and dropped media (see below).
- Connection diagnostics read from `getStats()`: transport, candidate
  types/addresses, negotiated codec, resolution, FPS, bitrate, RTT, and loss.
  A metric that the browser does not report is shown as `--`.
- No database and no persistent call history.
- Browser push notifications (Web Push / RFC 8291 & RFC 8292) with zero-knowledge delivery.
- 100% client-side contact book (`localStorage`) and shareable Call Card links (`#card=...`) without user accounts or server storage.

## Codec choice

H.264 is preferred first because it is hardware encoded on effectively every
device, giving the lowest encode latency and CPU cost. The trade-off is
compression: WebRTC negotiates constrained baseline H.264, so there are no
B-frames and quality per bit is weaker than VP9 or AV1.

The preference lives in `web/app.js` as `PREFERRED_VIDEO_CODEC`. AV1 gives the
best quality per bit but only encodes cheaply where the machine has a hardware
AV1 encoder; in software it is expensive and adds latency at high frame rates.
The codec that was actually negotiated is shown live in the call's metrics panel.

## Requirements

- Go 1.23 or newer.
- A browser with WebRTC and camera/microphone support.
- HTTPS in production.
- For production fallback: a coturn server reachable over the same address family as the participants.

The module pins `github.com/gorilla/websocket` v1.5.3.

## Local development

```bash
go mod download
go run ./cmd/server
```

### Hot Reloading (Live Dev Server)

To automatically rebuild and restart on Go code and frontend (`web/`) changes:

```bash
# Using Makefile (automatically ensures Air is available)
make dev

# Or directly with Air
air
```

The default listener is:

```text
[::]:8080
```

For a loopback-only development listener:

```bash
LISTEN_ADDR='[::1]:8080' \
PUBLIC_BASE_URL='http://[::1]:8080' \
go run ./cmd/server
```

Then open:

```text
http://[::1]:8080
```

For realistic iPhone/Android testing, use HTTPS with a real hostname and certificate.

## Production DNS

Publish A and/or AAAA records as appropriate for your deployment:

```text
call.example.com.  A     203.0.113.10
call.example.com.  AAAA  2001:db8:1234::10
turn.example.com.  A     203.0.113.20
turn.example.com.  AAAA  2001:db8:1234::20
```

## Build

```bash
go build -trimpath -ldflags='-s -w' -o ipv6-video-call ./cmd/server
```

The frontend is embedded in the binary through `web/embed.go`.

## Application configuration

Copy `.env.example` to `/etc/ipv6-video-call.env` and edit it.

Important values:

```text
LISTEN_ADDR=[::]:8080
PUBLIC_BASE_URL=https://call.example.com
STUN_URLS=stun:turn.example.com:3478
TURN_URLS=turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp
TURN_SHARED_SECRET=<random secret>
MAX_ROOMS=5000
ROOM_CREATE_RATE=0.2
ROOM_CREATE_BURST=5
TRUST_PROXY_HEADERS=true
```

Set `TRUST_PROXY_HEADERS=true` only when a trusted reverse proxy such as Caddy
fronts the server. It makes rate limiting use the last `X-Forwarded-For` entry,
which is the address the proxy itself observed. With the server exposed
directly, leave it false: any client could otherwise forge the header and
sidestep the limit.

`TURN_SHARED_SECRET` is never sent to browsers. `/api/config` creates a temporary username of the form:

```text
<expiration-unix-time>:<random-id>
```

and returns a coturn-compatible credential:

```text
base64(HMAC-SHA1(shared-secret, temporary-username))
```

Set the same secret as `static-auth-secret` in coturn.

## Caddy

Edit `deploy/Caddyfile` and replace the hostname. The supplied configuration proxies to the Go server on loopback:

```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

Caddy handles HTTPS and WebSocket proxying automatically.

## coturn

Install coturn, copy `deploy/coturn.conf`, and replace:

- The listening address with the TURN server's address.
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

The small relay range is adequate for this private two-person application. Increase it if you later support concurrent rooms.

## systemd

Create a dedicated service user and install the binary:

```bash
sudo useradd --system --home /nonexistent --shell /usr/bin/nologin ipv6call
sudo mkdir -p /opt/ipv6-video-call
sudo install -m 0755 ipv6-video-call /opt/ipv6-video-call/ipv6-video-call
sudo cp deploy/ipv6-video-call.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ipv6-video-call
```

## Disconnection recovery

Signaling and media fail independently, so they recover independently.

**Signaling socket drops.** Media is peer-to-peer and keeps flowing, so the
socket is reconnected in the background with exponential backoff (1s doubling to
15s) plus jitter, so two peers that dropped together do not retry in lockstep.
The client rejoins with the same client ID, which the server matches to the
existing role. If media is still healthy on rejoin, nothing is renegotiated: a
signaling-only blip must not disturb a working call.

**Media drops.** A `disconnected` state is given a short grace period, because it
commonly self-heals on network handover. If it has not recovered, or the state
went straight to `failed`, the caller issues an ICE restart
(`createOffer({ iceRestart: true })`) over the existing socket. Only the caller
does this; a restart from the callee would collide with the caller's negotiation.
If the socket is also down, the restart waits and the rejoin re-offers instead.

**The peer drops.** The server keeps the room alive for `EMPTY_ROOM_GRACE` after
its last peer disconnects, so `peer-left` is presented as "waiting for them to
return" rather than as the end of the call. When the peer rejoins, the server
broadcasts `peer-ready` and the caller renegotiates. After
`PEER_RETURN_TIMEOUT_MS` with no return, the call is reported as over.

**Failures that are not retried.** A deliberate `hangup` from the peer, and join
errors that retrying cannot fix (`room_not_found`, `room_expired`,
`unauthorized`, `room_full`, `at_capacity`), stop the retry loop and surface the
reason. Otherwise a deleted room would be hammered by an endless reconnect loop.

Reconnecting relies on the client ID held in `sessionStorage`, so a page reload
rejoins as the same participant. A client that loses that ID counts as a new
participant and is refused, because the room's two roles are already assigned.

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

## Privacy behavior

At the default log level the server does not log room IDs, invitation secrets,
SDP, ICE candidates, IP addresses, camera metadata, microphone metadata, or call
duration.

Setting `DEBUG_SIGNALING=1` raises the level to debug, which **does** log room
IDs, client IDs, roles, and signaling message types so a failing call can be
traced. Invitation secrets, SDP bodies, ICE candidates, and IP addresses are
never logged at any level. Leave `DEBUG_SIGNALING` unset in production.

Rooms are in memory only. They expire after `ROOM_TTL`. Once a room has had a participant, it is removed after both participants have been disconnected for `EMPTY_ROOM_GRACE`, which provides a small reconnect window.

### Privacy in Web Push notifications

- **No Central Contact Directory**: Push subscriptions (`endpoint`, `p256dh`, `auth`) and contact names exist solely in the user's browser `localStorage`. The Go server never stores an address book or links push endpoints to user accounts.
- **End-to-End Encrypted Push Payloads**: Notifications are encrypted on the server according to RFC 8291 (`aes128gcm` ECDH P-256 HKDF) using the recipient's public key. Push delivery relays (Apple APNs, Google FCM, Mozilla Autopush) see only ciphertext.
- **Stateless Push Relay**: The `POST /api/push/notify` endpoint forwards encrypted payloads to the push service with VAPID authentication (RFC 8292) and discards the request. Neither the recipient endpoint nor payload is logged.
- **Ephemeral Room Secrets**: Call invitation URLs include the `#SECRET` fragment. Because URL fragments are never sent to the server over HTTP requests, room secrets are transported directly to the recipient's browser inside the encrypted push payload.

## Tests

```bash
go test ./...
```

The test suite covers:

- room capacity and stable reconnect identity;
- secret rejection and empty-room cleanup;
- ICE candidate validation;
- removal of invalid ICE lines from SDP;
- coturn REST credential generation;
- teardown of a peer whose send buffer stalls, so signaling is never dropped
  silently;
- the room cap, including reclaiming expired rooms before rejecting;
- room-creation rate limiting and forged `X-Forwarded-For` handling;
- security headers, including that the CSP allows no inline script or style;
- room survival across a peer reconnect inside the grace window, and expiry
  once the window closes with no reconnect.

## Educational Disclaimer

This project is developed solely as an independent, educational research demonstration of peer-to-peer WebRTC signaling, media streaming, and zero-knowledge encryption using Go and browser standards.

**Non-Affiliation Notice:** This project is not affiliated with, sponsored by, endorsed by, or in any way officially associated with Discord Inc., or any of its subsidiaries or affiliates. "Discord" is a registered trademark of Discord Inc. The name "Gocord" is an arbitrary portmanteau and implies no endorsement or relationship. No proprietary protocols, client code, or private APIs belonging to Discord Inc. are used.

## License

This project is licensed under the [MIT License](LICENSE). See the [LICENSE](LICENSE) file for the full text, warranty disclaimer, and limitation of liability.
