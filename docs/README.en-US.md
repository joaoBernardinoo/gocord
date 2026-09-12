<div align="right">
  <strong>Languages:</strong>
  <a href="README.md">English</a> |
  <a href="docs/README.pt-BR.md">Português (Brasil)</a>
</div>

<p align="center">
  <img src="docs/assets/animation.gif" alt="Gocord Animation" width="100%">
</p>

# Gocord

### A private video call for two people. One link, no apps, no accounts, no one in the middle.

Open a browser, share a link, and talk. Gocord is a self-hosted video call
you fully own — your conversation goes directly from one device to the other,
never through a third party's servers.

## How it works

1. **Create a room.** You get a private link with a secret code baked in.
2. **Share it with one person.** However you like — chat, email, QR code.
3. **Talk.** The video and audio travel straight between the two browsers.

That's it. No sign-up, no download, no "please verify your phone number."

## Why you'd choose it

- **Your call is not stored anywhere.** Rooms live in memory and disappear.
  There is no database, no call history, and nothing to subpoena.
- **The media never touches your server.** Video and audio flow directly
  between the two browsers. The server only introduces them to each other.
- **The invitation secret never leaves the browser.** The room key travels
  in the link's `#fragment`, which browsers never send to any server.
- **Your contact book belongs to you.** Contacts and notification keys live
  only in your browser's local storage — there is no account and no central
  directory to leak.
- **Even the "answer the call" notifications are end-to-end encrypted.**
  Apple, Google, and Mozilla's push relays deliver your notification as
  ciphertext they cannot read.
- **Works on modern networks.** Full IPv4 **and** IPv6 support, with an
  automatic relay fallback when the two browsers can't reach each other
  directly.
- **Survives bad connections.** If your Wi-Fi drops and comes back, the call
  picks up again instead of dying.
- **Built for real hardware.** It prefers H.264 — the codec your phone and
  laptop encode in silicon — so calls run cool and lag-free, with automatic
  quality and low-bandwidth modes.
- **See what's happening.** A live diagnostics panel shows the real codec,
  resolution, frame rate, bitrate, and latency of your call.

## Who it's for

- Couples, families, and friends who want video calls without a tech giant
  in the room.
- Freelancers and consultants who need a no-install call link for clients.
- Teams with strict privacy requirements that can't use consumer apps.
- Anyone curious about how WebRTC, TURN, and end-to-end encryption actually
  work — the whole stack is open source.

## What you need to run it

One small Linux server and a domain name. That's the whole list — the app,
including its web interface, ships as a single binary and sips resources.
It runs comfortably on entry-level or free-tier cloud instances.

**Step-by-step instructions live in [INSTALLATION.md](INSTALLATION.md).**

For developers: `git clone`, `go run ./cmd/server`, open
`http://localhost:8080`. Details in
[INSTALLATION.md](INSTALLATION.md#local-development).

## FAQ

**Is this really private?** As private as a two-person call can be on today's
web. Media is peer-to-peer and DTLS-SRTP encrypted. The server never sees
your room secret, never stores contacts, and keeps no history. Push
notifications are encrypted end-to-end before they reach any relay.

**Can the server owner snoop on my call?** No. The server only relays the
handshake between the two browsers. Video and audio go directly between
peers, and if a TURN relay is needed, TURN traffic is also DTLS-SRTP
encrypted — a relay forwards media it cannot decrypt.

**How much does it cost to host?** A domain (~US$10/year) plus the smallest
VM you can rent, often within a provider's free tier. Two-person rooms are
in-memory and featherweight.

**Why only two people?** By design. A two-person call is small enough to
stay honest about its privacy claims and simple enough to audit.

**Is it an app?** No — it's a website you host. Works on desktop and mobile
browsers, camera switch and fullscreen included.

## Project details

For architecture, signaling protocol, security design, and configuration
reference, see [INSTALLATION.md](INSTALLATION.md) and the source itself —
the project is intentionally small enough to read.

## Educational Disclaimer

This project is developed solely as an independent, educational research
demonstration of peer-to-peer WebRTC signaling, media streaming, and
zero-knowledge encryption using Go and browser standards.

**Non-Affiliation Notice:** This project is not affiliated with, sponsored
by, endorsed by, or in any way officially associated with Discord Inc., or
any of its subsidiaries or affiliates. "Discord" is a registered trademark
of Discord Inc. The name "Gocord" is an arbitrary portmanteau and implies
no endorsement or relationship. No proprietary protocols, client code, or
private APIs belonging to Discord Inc. are used.

## License

This project is licensed under the [MIT License](LICENSE). See the
[LICENSE](LICENSE) file for the full text, warranty disclaimer, and
limitation of liability.
