(() => {
  "use strict";

  const els = {
    // Screen 1: Lobby
    homeView: document.getElementById("homeView"),
    createButton: document.getElementById("createButton"),
    quickStartBtn: document.getElementById("quickStartBtn"),
    joinInput: document.getElementById("joinInput"),
    joinButton: document.getElementById("joinButton"),
    homeError: document.getElementById("homeError"),
    openFriendsBtn: document.getElementById("openFriendsBtn"),
    navFriendsBtn: document.getElementById("navFriendsBtn"),
    mobileNavFriendsBtn: document.getElementById("mobileNavFriendsBtn"),

    // Screen: Friends / Contact Book
    friendsView: document.getElementById("friendsView"),
    backToHomeFromFriendsBtn: document.getElementById("backToHomeFromFriendsBtn"),
    pushStatusBadge: document.getElementById("pushStatusBadge"),
    enablePushBtn: document.getElementById("enablePushBtn"),
    enablePushBtnText: document.getElementById("enablePushBtnText"),
    pushSetupBlock: document.getElementById("pushSetupBlock"),
    myCardBlock: document.getElementById("myCardBlock"),
    myDisplayName: document.getElementById("myDisplayName"),
    myCallCardLink: document.getElementById("myCallCardLink"),
    copyCallCardBtn: document.getElementById("copyCallCardBtn"),
    iosPushBanner: document.getElementById("iosPushBanner"),
    pushError: document.getElementById("pushError"),
    contactNameInput: document.getElementById("contactNameInput"),
    contactCardInput: document.getElementById("contactCardInput"),
    saveContactBtn: document.getElementById("saveContactBtn"),
    contactError: document.getElementById("contactError"),
    contactSuccess: document.getElementById("contactSuccess"),
    contactsCount: document.getElementById("contactsCount"),
    contactsList: document.getElementById("contactsList"),

    // Screen 2: Pre-Call / Green Room
    preCallView: document.getElementById("preCallView"),
    preCallRoomBadge: document.getElementById("preCallRoomBadge"),
    precallCreateRoomNavBtn: document.getElementById("precallCreateRoomNavBtn"),
    inviteBlock: document.getElementById("inviteBlock"),
    inviteURL: document.getElementById("inviteURL"),
    copyButton: document.getElementById("copyButton"),
    previewVideo: document.getElementById("previewVideo"),
    previewFallback: document.getElementById("previewFallback"),
    precallVuMeter: document.getElementById("precallVuMeter"),
    precallMicToggle: document.getElementById("precallMicToggle"),
    precallCamToggle: document.getElementById("precallCamToggle"),
    precallNoiseToggle: document.getElementById("precallNoiseToggle"),
    precallMicLabel: document.getElementById("precallMicLabel"),
    precallCamLabel: document.getElementById("precallCamLabel"),
    audioSource: document.getElementById("audioSource"),
    audioOutput: document.getElementById("audioOutput"),
    videoSource: document.getElementById("videoSource"),
    preCallStatus: document.getElementById("preCallStatus"),
    preCallError: document.getElementById("preCallError"),
    startButton: document.getElementById("startButton"),
    precallCreateNewBtn: document.getElementById("precallCreateNewBtn"),

    // Screen 3: Active Call
    callView: document.getElementById("callView"),
    callDurationText: document.getElementById("callDurationText"),
    callRoomBadge: document.getElementById("callRoomBadge"),
    callRoomName: document.getElementById("callRoomName"),
    callStatsBadge: document.getElementById("callStatsBadge"),
    remoteVideo: document.getElementById("remoteVideo"),
    remotePlaceholder: document.getElementById("remotePlaceholder"),
    remotePeerName: document.getElementById("remotePeerName"),
    connectionStatus: document.getElementById("connectionStatus"),
    playRemoteButton: document.getElementById("playRemoteButton"),
    selfPipCard: document.getElementById("selfPipCard"),
    localVideo: document.getElementById("localVideo"),
    switchCameraButton: document.getElementById("switchCameraButton"),
    pipVuMeter: document.getElementById("pipVuMeter"),

    // Bottom Controls Dock
    muteButton: document.getElementById("muteButton"),
    cameraButton: document.getElementById("cameraButton"),
    shareScreenButton: document.getElementById("shareScreenButton"),
    voiceFxButton: document.getElementById("voiceFxButton"),
    toggleDrawerButton: document.getElementById("toggleDrawerButton"),
    hangupButton: document.getElementById("hangupButton"),

    // Collapsible Drawer & Telemetry
    telemetryDrawer: document.getElementById("telemetryDrawer"),
    closeDrawerButton: document.getElementById("closeDrawerButton"),
    drawerAudioSource: document.getElementById("drawerAudioSource"),
    drawerAudioOutput: document.getElementById("drawerAudioOutput"),
    drawerVideoSource: document.getElementById("drawerVideoSource"),
    qualitySelect: document.getElementById("qualitySelect"),
    metricBitrate: document.getElementById("metricBitrate"),
    metricRtt: document.getElementById("metricRtt"),
    metricLoss: document.getElementById("metricLoss"),
    metricCandidate: document.getElementById("metricCandidate"),
    metricCodec: document.getElementById("metricCodec"),
    metricResolution: document.getElementById("metricResolution"),
    diagnosticsGrid: document.getElementById("diagnosticsGrid"),
  };

  const state = {
    room: "",
    secret: "",
    inviteURL: "",
    clientID: loadClientID(),
    role: "",
    ws: null,
    pc: null,
    localStream: null,
    previewStream: null,
    screenStream: null,
    remoteStream: null,
    config: null,
    started: false,
    intentionalClose: false,
    makingOffer: false,
    pendingCandidates: [],
    reconnectTimer: null,
    disconnectTimer: null,
    peerGoneTimer: null,
    reconnectAttempts: 0,
    iceRestartInFlight: false,
    terminal: false,
    statsTimer: null,
    callDurationTimer: null,
    callStartTime: 0,
    facingMode: "user",
    lastBytesSent: 0,
    lastStatsAt: 0,
    selectedPairVerified: false,
    micEnabled: true,
    camEnabled: true,
    noiseSuppression: true,
    isSharingScreen: false,
    audioContext: null,
    analyser: null,
    animFrameId: null,
    pushSubscription: null,
    myDisplayName: localStorage.getItem("gocord.displayName") || "",
    contacts: [],
    swRegistration: null,
  };
  let audioCtx = null;
  function playSound(type) {
    if (!audioCtx) {
      audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    }
    if (audioCtx.state === "suspended") audioCtx.resume();

    const osc = audioCtx.createOscillator();
    const gainNode = audioCtx.createGain();
    osc.connect(gainNode);
    gainNode.connect(audioCtx.destination);
    const now = audioCtx.currentTime;

    if (type === "unmute") {
      osc.type = "sine";
      osc.frequency.setValueAtTime(400, now);
      osc.frequency.exponentialRampToValueAtTime(800, now + 0.1);
      gainNode.gain.setValueAtTime(0, now);
      gainNode.gain.linearRampToValueAtTime(0.5, now + 0.02);
      gainNode.gain.exponentialRampToValueAtTime(0.01, now + 0.1);
      osc.start(now);
      osc.stop(now + 0.1);
    } else if (type === "mute") {
      osc.type = "sine";
      osc.frequency.setValueAtTime(400, now);
      osc.frequency.exponentialRampToValueAtTime(200, now + 0.15);
      gainNode.gain.setValueAtTime(0, now);
      gainNode.gain.linearRampToValueAtTime(0.5, now + 0.02);
      gainNode.gain.exponentialRampToValueAtTime(0.01, now + 0.15);
      osc.start(now);
      osc.stop(now + 0.15);
    } else if (type === "join") {
      const osc1 = audioCtx.createOscillator();
      const gain1 = audioCtx.createGain();
      osc1.connect(gain1);
      gain1.connect(audioCtx.destination);
      osc1.type = "sine";
      osc1.frequency.value = 523.25;
      gain1.gain.setValueAtTime(0, now);
      gain1.gain.linearRampToValueAtTime(0.3, now + 0.05);
      gain1.gain.exponentialRampToValueAtTime(0.01, now + 0.3);
      osc1.start(now);
      osc1.stop(now + 0.3);

      const osc2 = audioCtx.createOscillator();
      const gain2 = audioCtx.createGain();
      osc2.connect(gain2);
      gain2.connect(audioCtx.destination);
      osc2.type = "sine";
      osc2.frequency.value = 659.25;
      gain2.gain.setValueAtTime(0, now + 0.1);
      gain2.gain.linearRampToValueAtTime(0.3, now + 0.15);
      gain2.gain.exponentialRampToValueAtTime(0.01, now + 0.4);
      osc2.start(now + 0.1);
      osc2.stop(now + 0.4);
    } else if (type === "leave") {
      osc.type = "sine";
      osc.frequency.setValueAtTime(523.25, now);
      osc.frequency.exponentialRampToValueAtTime(261.63, now + 0.2);
      gainNode.gain.setValueAtTime(0, now);
      gainNode.gain.linearRampToValueAtTime(0.3, now + 0.05);
      gainNode.gain.exponentialRampToValueAtTime(0.01, now + 0.3);
      osc.start(now);
      osc.stop(now + 0.3);
    }
  }

  // Recovery tuning. The server holds a room for EMPTY_ROOM_GRACE (45s by
  // default) after the last peer drops, so the client's patience is matched to
  // roughly that window.
  const SIGNALING_RETRY_BASE_MS = 1000;
  const SIGNALING_RETRY_MAX_MS = 15000;
  const ICE_DISCONNECT_GRACE_MS = 2500; // "disconnected" frequently self-heals
  const PEER_RETURN_TIMEOUT_MS = 60000;

  // Join errors that no amount of retrying will fix.
  const TERMINAL_SIGNALING_CODES = new Set([
    "room_not_found", "room_expired", "unauthorized", "room_full", "at_capacity", "bad_join",
  ]);

  const videoConstraints = {
    width: { ideal: 1920 },
    height: { ideal: 1080 },
    frameRate: { ideal: 30, max: 60 },
  };

  function loadClientID() {
    const key = "gocord.clientID";
    let id = sessionStorage.getItem(key);
    if (!id) {
      id = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
      sessionStorage.setItem(key, id);
    }
    return id;
  }

  // ==========================================
  // VIDEO CODEC PREFERENCE
  // ==========================================

  // H.264 is pinned first: it is hardware encoded on effectively every device,
  // which gives the lowest encode latency and CPU cost of the available codecs.
  // The remaining codecs stay in the list behind it so a peer that cannot do
  // H.264 still negotiates something rather than failing outright.
  const PREFERRED_VIDEO_CODEC = "video/H264";

  // Reorders the video transceiver's codec list. Must run before createOffer,
  // since the answering side only honours what the offer already lists.
  function preferVideoCodec(pc) {
    if (!pc || !window.RTCRtpSender || !RTCRtpSender.getCapabilities) return;
    if (!window.RTCRtpTransceiver || !("setCodecPreferences" in RTCRtpTransceiver.prototype)) return;

    const transceiver = pc.getTransceivers()
      .find(t => t.sender?.track?.kind === "video" || t.receiver?.track?.kind === "video");
    if (!transceiver) return;

    const all = RTCRtpSender.getCapabilities("video")?.codecs || [];
    const want = PREFERRED_VIDEO_CODEC.toLowerCase();
    const preferred = all.filter(c => (c.mimeType || "").toLowerCase() === want);
    if (!preferred.length) return; // Not supported here; leave the default order.

    try {
      transceiver.setCodecPreferences([...preferred, ...all.filter(c => !preferred.includes(c))]);
    } catch (e) {
      console.warn("Could not apply codec preference:", e);
    }
  }

  // ==========================================
  // OPUS AUDIO TUNING FOR WEAK NETWORKS
  // ==========================================

  // In-band Forward Error Correction (FEC), Discontinuous Transmission (DTX),
  // mono voice, and 28 kbps bitrate constraint for packet-loss resiliency.
  function enhanceOpusSDP(sdp) {
    if (!sdp || typeof sdp !== "string") return sdp;
    const separator = sdp.includes("\r\n") ? "\r\n" : "\n";
    const lines = sdp.split(separator);
    let opusPt = null;

    for (const line of lines) {
      const match = line.match(/^a=rtpmap:(\d+)\s+opus\/48000/i);
      if (match) {
        opusPt = match[1];
        break;
      }
    }

    if (!opusPt) return sdp;

    const opusParams = "useinbandfec=1;usedtx=1;stereo=0;sprop-stereo=0;maxaveragebitrate=28000;cbr=0";
    let foundFmtp = false;
    const newLines = [];

    for (const line of lines) {
      if (line.startsWith(`a=fmtp:${opusPt} `) || line.startsWith(`a=fmtp:${opusPt}=`)) {
        foundFmtp = true;
        const prefix = `a=fmtp:${opusPt} `;
        const existingVal = line.substring(prefix.length);
        const merged = mergeFmtpParams(existingVal, opusParams);
        newLines.push(`${prefix}${merged}`);
      } else {
        newLines.push(line);
      }
    }

    if (!foundFmtp) {
      const finalLines = [];
      for (const line of newLines) {
        finalLines.push(line);
        if (line.match(new RegExp(`^a=rtpmap:${opusPt}\\s+opus`, "i"))) {
          finalLines.push(`a=fmtp:${opusPt} ${opusParams}`);
        }
      }
      return finalLines.join(separator);
    }

    return newLines.join(separator);
  }

  function mergeFmtpParams(existing, desired) {
    const map = new Map();
    if (existing) {
      existing.split(";").forEach(pair => {
        const idx = pair.indexOf("=");
        if (idx > 0) {
          map.set(pair.substring(0, idx).trim().toLowerCase(), pair.substring(idx + 1).trim());
        } else if (pair.trim()) {
          map.set(pair.trim().toLowerCase(), "");
        }
      });
    }
    desired.split(";").forEach(pair => {
      const idx = pair.indexOf("=");
      if (idx > 0) {
        map.set(pair.substring(0, idx).trim().toLowerCase(), pair.substring(idx + 1).trim());
      } else if (pair.trim()) {
        map.set(pair.trim().toLowerCase(), "");
      }
    });
    const result = [];
    for (const [k, v] of map.entries()) {
      result.push(v !== "" ? `${k}=${v}` : k);
    }
    return result.join("; ");
  }

  // ==========================================
  // AUDIO-FIRST NETWORK PRIORITY ALLOCATION
  // ==========================================

  // Prioritizes audio packets over video in the WebRTC congestion controller (BBR/GCC)
  // and configures video degradation to drop resolution instead of stuttering framerate.
  function configureRtpPriorities(pc) {
    if (!pc || !pc.getSenders) return;
    try {
      for (const sender of pc.getSenders()) {
        if (!sender.track || !sender.getParameters || !sender.setParameters) continue;
        const params = sender.getParameters();
        if (!params.encodings || params.encodings.length === 0) {
          params.encodings = [{}];
        }

        if (sender.track.kind === "audio") {
          params.encodings[0].priority = "high";
          params.encodings[0].networkPriority = "high";
        } else if (sender.track.kind === "video") {
          params.encodings[0].priority = "low";
          params.encodings[0].networkPriority = "low";
          if ("degradationPreference" in params) {
            params.degradationPreference = "maintain-framerate";
          }
        }
        sender.setParameters(params).catch(() => {});
      }
    } catch (_) {}
  }

  function parseRoomFromLocation() {
    const match = location.pathname.match(/^\/join\/([A-Za-z0-9_-]+)$/);
    if (!match) return null;
    const room = match[1];
    let secret = location.hash ? location.hash.slice(1) : "";

    if (secret) {
      try {
        sessionStorage.setItem(`gocord.secret.${room}`, secret);
      } catch (_) {}
      // Occult the secret from the browser address bar immediately
      history.replaceState(null, "", `/join/${encodeURIComponent(room)}`);
    } else {
      try {
        secret = sessionStorage.getItem(`gocord.secret.${room}`) || "";
      } catch (_) {}
    }

    return { room, secret };
  }

  function show(view) {
    els.homeView.hidden = view !== "home";
    if (els.friendsView) els.friendsView.hidden = view !== "friends";
    els.preCallView.hidden = view !== "precall";
    els.callView.hidden = view !== "call";
  }

  function setStatus(text) {
    if (els.connectionStatus) els.connectionStatus.textContent = text.toUpperCase();
    if (els.preCallStatus) els.preCallStatus.textContent = text;
  }

  function showError(element, message) {
    if (!element) return;
    element.textContent = message;
    element.hidden = !message;
  }

  async function api(path, options = {}) {
    const response = await fetch(path, {
      ...options,
      headers: { "Content-Type": "application/json", ...(options.headers || {}) },
      cache: "no-store",
    });
    let body = null;
    try { body = await response.json(); } catch (_) {}
    if (!response.ok) {
      throw new Error(body?.error || `Request failed (${response.status})`);
    }
    return body;
  }

  // ==========================================
  // PUSH NOTIFICATIONS & LOCAL CONTACTS SYSTEM
  // ==========================================

  function urlBase64ToUint8Array(base64String) {
    const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
    const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
      outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
  }

  function encodeBase64URL(str) {
    return btoa(unescape(encodeURIComponent(str)))
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/, "");
  }

  function decodeBase64URL(str) {
    let base64 = str.replace(/-/g, "+").replace(/_/g, "/");
    while (base64.length % 4) base64 += "=";
    return decodeURIComponent(escape(atob(base64)));
  }

  function loadContacts() {
    try {
      const raw = localStorage.getItem("gocord.contacts");
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed : [];
    } catch (_) {
      return [];
    }
  }

  function saveContacts(contacts) {
    state.contacts = contacts;
    try {
      localStorage.setItem("gocord.contacts", JSON.stringify(contacts));
    } catch (_) {}
    renderContacts();
  }

  function renderContacts() {
    if (!els.contactsList) return;
    if (els.contactsCount) els.contactsCount.textContent = String(state.contacts.length);

    els.contactsList.innerHTML = "";
    if (state.contacts.length === 0) {
      const empty = document.createElement("div");
      empty.className = "contacts-empty-state";
      empty.textContent = "No contacts saved yet. Enable notifications above, generate your Call Card, and exchange cards with friends!";
      els.contactsList.appendChild(empty);
      return;
    }

    state.contacts.forEach((contact) => {
      const item = document.createElement("div");
      item.className = "contact-item";

      const info = document.createElement("div");
      info.className = "contact-info";

      const avatar = document.createElement("div");
      avatar.className = "contact-avatar";
      avatar.textContent = (contact.name || "C").charAt(0).toUpperCase();

      const details = document.createElement("div");
      details.className = "contact-details";

      const name = document.createElement("span");
      name.className = "contact-name";
      name.textContent = contact.name || "Unnamed Contact";

      const meta = document.createElement("span");
      meta.className = "contact-meta";
      meta.textContent = "● PUSH READY";

      details.appendChild(name);
      details.appendChild(meta);
      info.appendChild(avatar);
      info.appendChild(details);

      const actions = document.createElement("div");
      actions.className = "contact-actions";

      const callBtn = document.createElement("button");
      callBtn.className = "btn-brutalist btn-call-contact";
      callBtn.innerHTML = `
        <svg class="icon-inline" viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.5"><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="2" ry="2"/></svg>
        <span>CALL</span>
      `;
      callBtn.addEventListener("click", () => callContact(contact, callBtn));

      const delBtn = document.createElement("button");
      delBtn.className = "btn-brutalist btn-del-contact";
      delBtn.textContent = "✕";
      delBtn.title = "Delete contact";
      delBtn.addEventListener("click", () => deleteContact(contact.id));

      actions.appendChild(callBtn);
      actions.appendChild(delBtn);

      item.appendChild(info);
      item.appendChild(actions);
      els.contactsList.appendChild(item);
    });
  }

  function deleteContact(id) {
    const updated = state.contacts.filter((c) => c.id !== id);
    saveContacts(updated);
  }

  async function callContact(contact, btn) {
    if (!contact?.subscription?.endpoint) {
      showError(els.contactError, "Invalid contact subscription.");
      return;
    }

    const origHTML = btn ? btn.innerHTML : "";
    if (btn) {
      btn.disabled = true;
      btn.textContent = "CALLING...";
    }
    showError(els.contactError, "");

    try {
      // 1. Create encrypted room
      const roomData = await api("/api/rooms", { method: "POST" });
      const joinUrl = roomData.url || `${location.origin}/join/${roomData.room}#${roomData.secret}`;
      try {
        sessionStorage.setItem(`gocord.secret.${roomData.room}`, roomData.secret);
      } catch (_) {}

      // 2. Send push notification to contact
      const caller = state.myDisplayName || "A contact";
      await api("/api/push/notify", {
        method: "POST",
        body: JSON.stringify({
          subscription: contact.subscription,
          payload: {
            title: `Call from ${caller}`,
            body: `${caller} is calling you on Gocord`,
            url: joinUrl,
            room: roomData.room,
            callerName: caller,
          },
          ttl: 120,
        }),
      });

      // 3. Move caller to green room with clean URL
      state.room = roomData.room;
      state.secret = roomData.secret;
      state.inviteURL = joinUrl;
      history.pushState(null, "", `/join/${encodeURIComponent(state.room)}`);

      if (btn) {
        btn.disabled = false;
        btn.innerHTML = origHTML;
      }
      els.inviteURL.value = state.inviteURL;
      els.inviteBlock.hidden = false;
      updateRoomTags(state.room);
      setStatus(`Calling ${contact.name || "contact"}… waiting for peer to join.`);
      show("precall");
      startPreviewMedia();
    } catch (err) {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = origHTML;
      }
      if (err.message && (err.message.includes("410") || err.message.includes("VapidPkHashMismatch") || err.message.includes("older server key") || err.message.includes("expired"))) {
        showError(els.contactError, `${contact.name}'s Call Card was created with an older server key. Ask them to click "REFRESH" on /friends and send you their updated Call Card.`);
      } else {
        showError(els.contactError, `Could not call contact: ${err.message}`);
      }
    }
  }

  function updatePushUI() {
    const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
    const isStandalone = window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;

    if (els.iosPushBanner) {
      els.iosPushBanner.hidden = !(isIOS && !isStandalone);
    }

    if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
      if (els.pushStatusBadge) {
        els.pushStatusBadge.textContent = "✕ UNSUPPORTED";
        els.pushStatusBadge.className = "push-status-pill pill-error";
      }
      if (els.enablePushBtn) els.enablePushBtn.disabled = true;
      if (els.enablePushBtnText) {
        els.enablePushBtnText.textContent = isIOS ? "INSTALL VIA SHARE ➔ ADD TO HOME SCREEN" : "PUSH NOT SUPPORTED";
      }
      return;
    }

    if (state.pushSubscription) {
      if (els.pushStatusBadge) {
        els.pushStatusBadge.textContent = "● PUSH ACTIVE";
        els.pushStatusBadge.className = "push-status-pill pill-active";
      }
      if (els.enablePushBtnText) els.enablePushBtnText.textContent = "PUSH ACTIVE (CLICK TO REFRESH)";
      if (els.myCardBlock) els.myCardBlock.hidden = false;
      updateCallCardLink();
    } else {
      if (els.pushStatusBadge) {
        els.pushStatusBadge.textContent = "● PUSH INACTIVE";
        els.pushStatusBadge.className = "push-status-pill pill-inactive";
      }
      if (els.enablePushBtnText) els.enablePushBtnText.textContent = "ENABLE CALL NOTIFICATIONS";
      if (els.myCardBlock) els.myCardBlock.hidden = true;
    }
  }

  function updateCallCardLink() {
    if (!state.pushSubscription) return;
    try {
      const subJSON = state.pushSubscription.toJSON();
      const cardObj = {
        name: state.myDisplayName || "",
        endpoint: subJSON.endpoint,
        keys: {
          p256dh: subJSON.keys?.p256dh,
          auth: subJSON.keys?.auth,
        },
      };
      const token = encodeBase64URL(JSON.stringify(cardObj));
      const fullLink = `${location.origin}/friends#card=${token}`;
      if (els.myCallCardLink) els.myCallCardLink.value = fullLink;
    } catch (_) {}
  }

  async function initServiceWorkerAndPush() {
    if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
      updatePushUI();
      return;
    }

    try {
      state.swRegistration = await navigator.serviceWorker.register("/sw.js", { scope: "/" });
      state.pushSubscription = await state.swRegistration.pushManager.getSubscription();
      updatePushUI();
    } catch (err) {
      console.warn("Service worker / push init error:", err);
      updatePushUI();
    }
  }

  async function subscribePush() {
    showError(els.pushError, "");
    if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
      const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
      if (isIOS) {
        showError(els.pushError, "On iOS, tap Share (⎋) ➔ 'Add to Home Screen' first, then open Gocord from your Home Screen.");
      } else {
        showError(els.pushError, "Push notifications are not supported in this browser.");
      }
      return;
    }

    try {
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        showError(els.pushError, "Notification permission was denied. Please allow notifications in browser site settings.");
        return;
      }

      if (!state.config) {
        state.config = await api("/api/config");
      }

      const vapidKey = state.config?.vapidPublicKey;
      if (!vapidKey) {
        showError(els.pushError, "Could not obtain VAPID public key from server.");
        return;
      }

      const reg = state.swRegistration || (await navigator.serviceWorker.ready);
      
      // Unsubscribe any prior subscription to prevent "different application server key already exists" error
      try {
        const existingSub = await reg.pushManager.getSubscription();
        if (existingSub) {
          await existingSub.unsubscribe();
        }
      } catch (_) {}

      const convertedVapidKey = urlBase64ToUint8Array(vapidKey);
      state.pushSubscription = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: convertedVapidKey,
      });

      updatePushUI();
      playSound("join");
      if (els.contactSuccess) {
        els.contactSuccess.textContent = "✦ Call Card updated with current server key! Share your new link.";
        els.contactSuccess.hidden = false;
        setTimeout(() => {
          if (els.contactSuccess) els.contactSuccess.hidden = true;
        }, 4000);
      }
    } catch (err) {
      showError(els.pushError, `Failed to subscribe: ${err.message}`);
    }
  }

  function parseCallCard(raw) {
    let input = raw.trim();
    if (input.includes("#card=")) {
      input = input.split("#card=")[1];
    } else if (input.startsWith("#card=")) {
      input = input.slice(6);
    }

    let parsed = null;
    try {
      const decoded = decodeBase64URL(input);
      parsed = JSON.parse(decoded);
    } catch (_) {
      try {
        parsed = JSON.parse(input);
      } catch (__) {}
    }

    if (!parsed) {
      throw new Error("Could not decode Call Card format.");
    }

    let endpoint = parsed.endpoint || parsed.subscription?.endpoint;
    let p256dh = parsed.keys?.p256dh || parsed.subscription?.keys?.p256dh;
    let auth = parsed.keys?.auth || parsed.subscription?.keys?.auth;
    let name = parsed.name || "";

    if (!endpoint || !p256dh || !auth) {
      throw new Error("Call Card is missing endpoint or cryptographic keys.");
    }

    return {
      name,
      subscription: {
        endpoint,
        keys: { p256dh, auth },
      },
    };
  }

  function handleSaveContact() {
    showError(els.contactError, "");
    if (els.contactSuccess) els.contactSuccess.hidden = true;

    const name = els.contactNameInput ? els.contactNameInput.value.trim() : "";
    const rawCard = els.contactCardInput ? els.contactCardInput.value.trim() : "";

    if (!name) {
      showError(els.contactError, "Please enter a contact name.");
      return;
    }
    if (!rawCard) {
      showError(els.contactError, "Please paste a Call Card link or code.");
      return;
    }

    try {
      const card = parseCallCard(rawCard);
      const contactObj = {
        id: `c_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
        name: name,
        subscription: card.subscription,
        addedAt: Date.now(),
      };

      const existingIdx = state.contacts.findIndex(
        (c) => c.subscription.endpoint === card.subscription.endpoint || c.name.toLowerCase() === name.toLowerCase()
      );

      let updated = [...state.contacts];
      if (existingIdx >= 0) {
        updated[existingIdx] = contactObj;
      } else {
        updated.push(contactObj);
      }

      saveContacts(updated);
      if (els.contactNameInput) els.contactNameInput.value = "";
      if (els.contactCardInput) els.contactCardInput.value = "";
      if (els.contactSuccess) {
        els.contactSuccess.textContent = `✦ Contact "${contactObj.name}" saved successfully!`;
        els.contactSuccess.hidden = false;
        setTimeout(() => {
          if (els.contactSuccess) els.contactSuccess.hidden = true;
        }, 4000);
      }
      playSound("join");
    } catch (err) {
      showError(els.contactError, err.message);
    }
  }

  function handleCardHashImport() {
    if (location.hash && location.hash.startsWith("#card=")) {
      try {
        const card = parseCallCard(location.hash);
        if (els.contactNameInput && card.name) els.contactNameInput.value = card.name;
        if (els.contactCardInput) els.contactCardInput.value = location.href;
        if (els.contactSuccess) {
          els.contactSuccess.textContent = `✦ Call Card detected${card.name ? ` for "${card.name}"` : ""}! Verify name and click "SAVE CONTACT".`;
          els.contactSuccess.hidden = false;
        }
        show("friends");
        history.replaceState(null, "", "/friends");
      } catch (_) {}
    }
  }

  async function copyCallCardLink() {
    if (!els.myCallCardLink || !els.myCallCardLink.value) return;
    try {
      await navigator.clipboard.writeText(els.myCallCardLink.value);
      if (els.copyCallCardBtn) {
        const orig = els.copyCallCardBtn.innerHTML;
        els.copyCallCardBtn.textContent = "COPIED!";
        setTimeout(() => { if (els.copyCallCardBtn) els.copyCallCardBtn.innerHTML = orig; }, 2000);
      }
    } catch (_) {
      els.myCallCardLink.select();
    }
  }

  function navigate(path, replace = false) {
    if (replace) {
      history.replaceState(null, "", path);
    } else {
      history.pushState(null, "", path);
    }
    handleRoute();
  }

  async function handleRoute() {
    handleCardHashImport();

    const path = location.pathname.toLowerCase();
    if (path === "/friends" || path === "/contacts") {
      show("friends");
      return;
    }

    const locationRoom = parseRoomFromLocation();
    if (!locationRoom) {
      if (!location.hash.startsWith("#card=")) {
        show("home");
      }
      return;
    }

    state.room = locationRoom.room;
    state.secret = locationRoom.secret;
    state.inviteURL = `${location.origin}/join/${state.room}#${state.secret}`;
    show("precall");
    updateRoomTags(state.room);

    if (!state.secret) {
      showError(els.preCallError, "This invitation is missing its #secret fragment. Ask for the complete link.");
      els.startButton.disabled = true;
      return;
    }

    try {
      setStatus("Checking room availability…");
      await api(`/api/rooms/${encodeURIComponent(state.room)}`);
      setStatus("Room available. Dual-stack P2P ready.");
      els.inviteURL.value = state.inviteURL;
      els.inviteBlock.hidden = false;
      startPreviewMedia();
    } catch (err) {
      showError(els.preCallError, err.message === "Request failed (404)" ? "This room does not exist or has expired." : err.message);
      els.startButton.disabled = true;
    }
  }

  async function initialize() {
    state.contacts = loadContacts();
    bindUI();
    await enumerateHardwareDevices();

    if (els.myDisplayName) {
      els.myDisplayName.value = state.myDisplayName;
    }
    renderContacts();
    initServiceWorkerAndPush();
    await handleRoute();
  }

  function updateRoomTags(room) {
    const display = `#${room.slice(0, 8)}`;
    if (els.preCallRoomBadge) els.preCallRoomBadge.textContent = `✦ GOCORD READY: ${display}`;
    if (els.callRoomName) els.callRoomName.textContent = `ROOM ${display}`;
  }

  function bindUI() {
    // Screen 1: Lobby
    if (els.createButton) els.createButton.addEventListener("click", createCall);
    if (els.quickStartBtn) els.quickStartBtn.addEventListener("click", createCall);
    if (els.joinButton) els.joinButton.addEventListener("click", handleJoinInput);
    if (els.joinInput) {
      els.joinInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") handleJoinInput();
      });
    }
    if (els.openFriendsBtn) els.openFriendsBtn.addEventListener("click", () => navigate("/friends"));
    if (els.navFriendsBtn) els.navFriendsBtn.addEventListener("click", () => navigate("/friends"));
    if (els.mobileNavFriendsBtn) els.mobileNavFriendsBtn.addEventListener("click", () => navigate("/friends"));
    if (els.backToHomeFromFriendsBtn) els.backToHomeFromFriendsBtn.addEventListener("click", () => navigate("/"));

    // Push & Contacts UI
    if (els.enablePushBtn) els.enablePushBtn.addEventListener("click", subscribePush);
    if (els.copyCallCardBtn) els.copyCallCardBtn.addEventListener("click", copyCallCardLink);
    if (els.saveContactBtn) els.saveContactBtn.addEventListener("click", handleSaveContact);
    if (els.contactCardInput) {
      els.contactCardInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") handleSaveContact();
      });
    }
    if (els.myDisplayName) {
      els.myDisplayName.addEventListener("input", (e) => {
        state.myDisplayName = e.target.value.trim();
        try {
          localStorage.setItem("gocord.displayName", state.myDisplayName);
        } catch (_) {}
        updateCallCardLink();
      });
    }

    // Screen 2: Pre-call
    if (els.copyButton) els.copyButton.addEventListener("click", copyInvite);
    if (els.callRoomBadge) els.callRoomBadge.addEventListener("click", copyInvite);
    if (els.startButton) els.startButton.addEventListener("click", startSession);
    if (els.precallCreateRoomNavBtn) els.precallCreateRoomNavBtn.addEventListener("click", createCall);
    if (els.precallCreateNewBtn) els.precallCreateNewBtn.addEventListener("click", createCall);
    if (els.precallMicToggle) els.precallMicToggle.addEventListener("click", togglePrecallMic);
    if (els.precallCamToggle) els.precallCamToggle.addEventListener("click", togglePrecallCam);
    if (els.precallNoiseToggle) els.precallNoiseToggle.addEventListener("click", togglePrecallNoise);

    if (els.audioSource) els.audioSource.addEventListener("change", switchAudioInput);
    if (els.videoSource) els.videoSource.addEventListener("change", switchVideoInput);
    if (els.audioOutput) els.audioOutput.addEventListener("change", switchAudioOutput);

    // Screen 3: In-call Controls
    if (els.muteButton) els.muteButton.addEventListener("click", toggleMute);
    if (els.cameraButton) els.cameraButton.addEventListener("click", toggleCamera);
    if (els.shareScreenButton) {
      if (!navigator.mediaDevices || !navigator.mediaDevices.getDisplayMedia) {
        els.shareScreenButton.style.display = "none";
      } else {
        els.shareScreenButton.addEventListener("click", toggleScreenShare);
      }
    }
    if (els.voiceFxButton) els.voiceFxButton.addEventListener("click", toggleVoiceFx);
    if (els.switchCameraButton) els.switchCameraButton.addEventListener("click", switchCamera);
    if (els.toggleDrawerButton) els.toggleDrawerButton.addEventListener("click", toggleDrawer);
    if (els.closeDrawerButton) els.closeDrawerButton.addEventListener("click", toggleDrawer);
    if (els.hangupButton) els.hangupButton.addEventListener("click", () => hangup(true));

    if (els.drawerAudioSource) els.drawerAudioSource.addEventListener("change", switchAudioInput);
    if (els.drawerAudioOutput) els.drawerAudioOutput.addEventListener("change", switchAudioOutput);
    if (els.drawerVideoSource) els.drawerVideoSource.addEventListener("change", switchVideoInput);

    // Quality Profile Buttons
    document.querySelectorAll(".btn-quality-pill:not(.btn-fit-pill)").forEach(btn => {
      btn.addEventListener("click", (e) => {
        document.querySelectorAll(".btn-quality-pill:not(.btn-fit-pill)").forEach(b => b.classList.remove("active"));
        e.currentTarget.classList.add("active");
        const q = e.currentTarget.dataset.quality;
        if (els.qualitySelect) els.qualitySelect.value = q;
        applyQualityProfile(q);
      });
    });

    // Video Fit Buttons
    document.querySelectorAll(".btn-fit-pill").forEach(btn => {
      btn.addEventListener("click", (e) => {
        document.querySelectorAll(".btn-fit-pill").forEach(b => b.classList.remove("active"));
        e.currentTarget.classList.add("active");
        const fit = e.currentTarget.dataset.fit;
        if (els.remoteVideo) els.remoteVideo.style.objectFit = fit;
        if (els.localVideo) els.localVideo.style.objectFit = fit;
      });
    });

    if (els.playRemoteButton) {
      els.playRemoteButton.addEventListener("click", async () => {
        try {
          await els.remoteVideo.play();
          els.playRemoteButton.hidden = true;
        } catch (_) {}
      });
    }

    if (els.remoteVideo) els.remoteVideo.addEventListener("loadedmetadata", playRemote);
    window.addEventListener("beforeunload", () => closeLocalResources(false));
    navigator.mediaDevices?.addEventListener?.("devicechange", enumerateHardwareDevices);
    initDraggablePip();
  }

  // ==========================================
  // HARDWARE DEVICES & PREVIEW AUDIO VISUALIZER
  // ==========================================

  async function enumerateHardwareDevices() {
    if (!navigator.mediaDevices?.enumerateDevices) return;
    try {
      const devices = await navigator.mediaDevices.enumerateDevices();
      populateSelect(els.audioSource, devices, "audioinput", "Default Microphone");
      populateSelect(els.drawerAudioSource, devices, "audioinput", "Default Microphone");
      populateSelect(els.audioOutput, devices, "audiooutput", "Default Speaker");
      populateSelect(els.drawerAudioOutput, devices, "audiooutput", "Default Speaker");
      populateSelect(els.videoSource, devices, "videoinput", "Default Camera");
      populateSelect(els.drawerVideoSource, devices, "videoinput", "Default Camera");
    } catch (_) {}
  }

  function populateSelect(selectEl, devices, kind, defaultLabel) {
    if (!selectEl) return;
    const current = selectEl.value;
    selectEl.innerHTML = "";
    const filtered = devices.filter(d => d.kind === kind);
    if (filtered.length === 0) {
      const opt = document.createElement("option");
      opt.value = "";
      opt.textContent = defaultLabel;
      selectEl.appendChild(opt);
      return;
    }
    filtered.forEach((d, idx) => {
      const opt = document.createElement("option");
      opt.value = d.deviceId;
      opt.textContent = d.label || `${defaultLabel} ${idx + 1}`;
      selectEl.appendChild(opt);
    });
    if (current && [...selectEl.options].some(o => o.value === current)) {
      selectEl.value = current;
    }
  }

  async function startPreviewMedia() {
    stopPreviewMedia();
    try {
      const constraints = {
        video: state.camEnabled ? {
          ...videoConstraints,
          deviceId: els.videoSource?.value ? { exact: els.videoSource.value } : undefined,
          facingMode: { ideal: state.facingMode }
        } : false,
        audio: state.micEnabled ? {
          deviceId: els.audioSource?.value ? { exact: els.audioSource.value } : undefined,
          echoCancellation: { ideal: true },
          noiseSuppression: { ideal: state.noiseSuppression },
        } : false,
      };

      if (!constraints.video && !constraints.audio) return;
      state.previewStream = await navigator.mediaDevices.getUserMedia(constraints);
      
      if (els.previewVideo && constraints.video) {
        els.previewVideo.srcObject = state.previewStream;
        if (els.previewFallback) els.previewFallback.hidden = true;
      }

      if (constraints.audio) {
        attachAudioAnalyser(state.previewStream, els.precallVuMeter);
      }
      await enumerateHardwareDevices();
    } catch (err) {
      if (els.previewFallback) els.previewFallback.hidden = false;
    }
  }

  function stopPreviewMedia() {
    if (state.previewStream) {
      state.previewStream.getTracks().forEach(t => t.stop());
      state.previewStream = null;
    }
    stopAudioAnalyser();
  }

  function attachAudioAnalyser(stream, meterEl) {
    if (!stream || stream.getAudioTracks().length === 0 || !meterEl) return;
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (!AudioCtx) return;
      if (!state.audioContext) state.audioContext = new AudioCtx();
      if (state.audioContext.state === "suspended") state.audioContext.resume();

      const source = state.audioContext.createMediaStreamSource(stream);
      state.analyser = state.audioContext.createAnalyser();
      state.analyser.fftSize = 32;
      source.connect(state.analyser);

      const bufferLength = state.analyser.frequencyBinCount;
      const dataArray = new Uint8Array(bufferLength);
      const bars = meterEl.querySelectorAll(".bar");

      function updateVU() {
        if (!state.analyser) return;
        state.analyser.getByteFrequencyData(dataArray);
        let sum = 0;
        for (let i = 0; i < bufferLength; i++) sum += dataArray[i];
        const average = sum / bufferLength;
        const activeCount = Math.min(bars.length, Math.round((average / 128) * bars.length));

        bars.forEach((bar, idx) => {
          if (idx < activeCount) {
            bar.classList.add("active");
            bar.style.height = `${Math.max(6, Math.min(14, (dataArray[idx] || average) / 18))}px`;
          } else {
            bar.classList.remove("active");
            bar.style.height = "4px";
          }
        });

        state.animFrameId = requestAnimationFrame(updateVU);
      }

      updateVU();
    } catch (_) {}
  }

  function stopAudioAnalyser() {
    if (state.animFrameId) {
      cancelAnimationFrame(state.animFrameId);
      state.animFrameId = null;
    }
    if (state.analyser) state.analyser = null;
  }

  // ==========================================
  // CALL CREATION & ROUTING
  // ==========================================

  async function createCall() {
    if (els.createButton) els.createButton.disabled = true;
    if (els.quickStartBtn) els.quickStartBtn.disabled = true;
    showError(els.homeError, "");
    try {
      const created = await api("/api/rooms", { method: "POST", body: "{}" });
      state.room = created.room;
      state.secret = created.secret;
      state.inviteURL = created.url || `${location.origin}/join/${state.room}#${state.secret}`;
      try {
        sessionStorage.setItem(`gocord.secret.${state.room}`, state.secret);
      } catch (_) {}

      // Occult the secret from the address bar
      history.pushState(null, "", `/join/${encodeURIComponent(state.room)}`);

      els.inviteURL.value = state.inviteURL;
      els.inviteBlock.hidden = false;
      updateRoomTags(state.room);
      setStatus("Room ready. Share invite link and check media.");
      show("precall");
      startPreviewMedia();
    } catch (err) {
      showError(els.homeError, err.message);
    } finally {
      if (els.createButton) els.createButton.disabled = false;
      if (els.quickStartBtn) els.quickStartBtn.disabled = false;
    }
  }

  function handleJoinInput() {
    const val = els.joinInput?.value?.trim();
    if (!val) return;
    try {
      if (val.startsWith("http")) {
        const url = new URL(val);
        const match = url.pathname.match(/^\/join\/([A-Za-z0-9_-]+)$/);
        if (match) {
          const room = match[1];
          const secret = url.hash.slice(1);
          if (secret) {
            try {
              sessionStorage.setItem(`gocord.secret.${room}`, secret);
            } catch (_) {}
          }
          state.room = room;
          state.secret = secret;
          state.inviteURL = url.href;
          history.pushState(null, "", `/join/${encodeURIComponent(room)}`);
          show("precall");
          updateRoomTags(state.room);
          els.inviteURL.value = state.inviteURL;
          els.inviteBlock.hidden = false;
          startPreviewMedia();
          return;
        }
      }

      let room = val;
      let secret = "";
      if (val.includes("#")) {
        const parts = val.split("#");
        room = parts[0];
        secret = parts[1];
      }
      if (secret) {
        try {
          sessionStorage.setItem(`gocord.secret.${room}`, secret);
        } catch (_) {}
      }
      state.room = room;
      state.secret = secret;
      state.inviteURL = `${location.origin}/join/${room}#${secret}`;
      history.pushState(null, "", `/join/${encodeURIComponent(room)}`);
      show("precall");
      updateRoomTags(state.room);
      els.inviteURL.value = state.inviteURL;
      els.inviteBlock.hidden = false;
      startPreviewMedia();
    } catch (_) {
      location.href = `/join/${encodeURIComponent(val)}`;
    }
  }

  async function copyInvite() {
    const url = state.inviteURL || els.inviteURL?.value || `${location.origin}/join/${state.room}#${state.secret}`;
    try {
      await navigator.clipboard.writeText(url);
      const btn = els.copyButton;
      if (btn) {
        const old = btn.innerHTML;
        btn.textContent = "COPIED!";
        setTimeout(() => { btn.innerHTML = old; }, 1400);
      }
      if (els.callRoomName) {
        const oldName = els.callRoomName.textContent;
        els.callRoomName.textContent = "COPIED INVITE!";
        setTimeout(() => { els.callRoomName.textContent = oldName; }, 1400);
      }
    } catch (_) {
      if (els.inviteURL) {
        els.inviteURL.value = url;
        els.inviteURL.select();
        document.execCommand("copy");
      }
    }
  }

  // ==========================================
  // PRE-CALL TOGGLES & HARDWARE SWITCHERS
  // ==========================================

  function togglePrecallMic() {
    state.micEnabled = !state.micEnabled;
    els.precallMicToggle.classList.toggle("active", state.micEnabled);
    if (els.precallMicLabel) els.precallMicLabel.textContent = state.micEnabled ? "MIC ACTIVE" : "MIC MUTED";
    if (state.previewStream) {
      state.previewStream.getAudioTracks().forEach(t => { t.enabled = state.micEnabled; });
    }
  }

  function togglePrecallCam() {
    state.camEnabled = !state.camEnabled;
    els.precallCamToggle.classList.toggle("active", state.camEnabled);
    if (els.precallCamLabel) els.precallCamLabel.textContent = state.camEnabled ? "CAM ACTIVE" : "CAM OFF";
    if (state.previewStream) {
      state.previewStream.getVideoTracks().forEach(t => { t.enabled = state.camEnabled; });
      if (els.previewFallback) els.previewFallback.hidden = state.camEnabled;
    }
  }

  async function togglePrecallNoise() {
    state.noiseSuppression = !state.noiseSuppression;
    els.precallNoiseToggle.classList.toggle("active", state.noiseSuppression);
    if (state.previewStream) {
      const oldAudioTrack = state.previewStream.getAudioTracks()[0];
      try {
        const newStream = await navigator.mediaDevices.getUserMedia({
          audio: {
            deviceId: els.audioSource?.value ? { exact: els.audioSource.value } : undefined,
            echoCancellation: { ideal: true },
            noiseSuppression: { ideal: state.noiseSuppression },
          }
        });
        const newTrack = newStream.getAudioTracks()[0];
        newTrack.enabled = state.micEnabled;
        if (oldAudioTrack) {
          oldAudioTrack.stop();
          state.previewStream.removeTrack(oldAudioTrack);
        }
        state.previewStream.addTrack(newTrack);
        attachAudioAnalyser(state.previewStream, els.precallVuMeter);
      } catch (_) {}
    }
  }

  async function switchAudioInput(e) {
    const deviceId = e.target.value;
    if (els.audioSource) els.audioSource.value = deviceId;
    if (els.drawerAudioSource) els.drawerAudioSource.value = deviceId;
    if (state.started && state.localStream) {
      const audioTrack = state.localStream.getAudioTracks()[0];
      if (audioTrack) audioTrack.stop();
      const newStream = await navigator.mediaDevices.getUserMedia({ audio: { deviceId: { exact: deviceId } } });
      const newTrack = newStream.getAudioTracks()[0];
      state.localStream.removeTrack(audioTrack);
      state.localStream.addTrack(newTrack);
      const sender = state.pc?.getSenders?.().find(s => s.track?.kind === "audio");
      if (sender) sender.replaceTrack(newTrack);
    } else if (state.previewStream) {
      const oldAudioTrack = state.previewStream.getAudioTracks()[0];
      try {
        const newStream = await navigator.mediaDevices.getUserMedia({ audio: { deviceId: { exact: deviceId } } });
        const newTrack = newStream.getAudioTracks()[0];
        newTrack.enabled = state.micEnabled;
        if (oldAudioTrack) {
          oldAudioTrack.stop();
          state.previewStream.removeTrack(oldAudioTrack);
        }
        state.previewStream.addTrack(newTrack);
        attachAudioAnalyser(state.previewStream, els.precallVuMeter);
      } catch (_) {}
    }
  }

  async function switchVideoInput(e) {
    const deviceId = e.target.value;
    if (els.videoSource) els.videoSource.value = deviceId;
    if (els.drawerVideoSource) els.drawerVideoSource.value = deviceId;
    if (state.started && state.localStream) {
      const videoTrack = state.localStream.getVideoTracks()[0];
      if (videoTrack) videoTrack.stop();
      const newStream = await navigator.mediaDevices.getUserMedia({
        video: { ...videoConstraints, deviceId: { exact: deviceId } }
      });
      const newTrack = newStream.getVideoTracks()[0];
      state.localStream.removeTrack(videoTrack);
      state.localStream.addTrack(newTrack);
      const sender = state.pc?.getSenders?.().find(s => s.track?.kind === "video");
      if (sender) sender.replaceTrack(newTrack);
      els.localVideo.srcObject = state.localStream;
    } else {
      startPreviewMedia();
    }
  }

  async function switchAudioOutput(e) {
    const deviceId = e.target.value;
    if (els.audioOutput) els.audioOutput.value = deviceId;
    if (els.drawerAudioOutput) els.drawerAudioOutput.value = deviceId;
    if (els.remoteVideo?.setSinkId) {
      try { await els.remoteVideo.setSinkId(deviceId); } catch (_) {}
    }
  }

  // ==========================================
  // ACTIVE CALL SESSION
  // ==========================================

  async function startSession() {
    if (state.started) return;
    state.started = true;
    state.intentionalClose = false;
    state.terminal = false;
    state.reconnectAttempts = 0;
    state.iceRestartInFlight = false;
    els.startButton.disabled = true;
    showError(els.preCallError, "");

    stopPreviewMedia();

    try {
      setStatus("Initializing signaling…");
      state.config = await api("/api/config");

      setStatus("Acquiring media streams…");
      state.localStream = await navigator.mediaDevices.getUserMedia({
        video: state.camEnabled ? {
          ...videoConstraints,
          deviceId: els.videoSource?.value ? { exact: els.videoSource.value } : undefined,
          facingMode: { ideal: state.facingMode }
        } : false,
        audio: state.micEnabled ? {
          deviceId: els.audioSource?.value ? { exact: els.audioSource.value } : undefined,
          echoCancellation: { ideal: true },
          noiseSuppression: { ideal: state.noiseSuppression },
          autoGainControl: { ideal: true },
        } : false,
      });

      const videoTrack = state.localStream.getVideoTracks()[0];
      if (videoTrack && "contentHint" in videoTrack) videoTrack.contentHint = "motion";
      if (els.localVideo) {
        els.localVideo.srcObject = state.localStream;
        els.localVideo.addEventListener("loadedmetadata", () => {
          if (els.localVideo.videoWidth && els.localVideo.videoHeight && els.selfPipCard) {
            els.selfPipCard.style.aspectRatio = `${els.localVideo.videoWidth} / ${els.localVideo.videoHeight}`;
          }
        });
      }

      if (state.micEnabled) {
        attachAudioAnalyser(state.localStream, els.pipVuMeter);
      }

      if (window.innerWidth < 768 && els.telemetryDrawer) {
        els.telemetryDrawer.classList.add("collapsed");
        if (els.toggleDrawerButton) els.toggleDrawerButton.setAttribute("aria-expanded", "false");
      }

      show("call");
      setStatus("Connecting peer mesh…");
      startCallTimer();
      createPeerConnection();
      connectWebSocket();
      startStats();
    } catch (err) {
      state.started = false;
      els.startButton.disabled = false;
      show("precall");
      showError(els.preCallError, friendlyMediaError(err));
      closeLocalResources(false);
    }
  }

  function friendlyMediaError(err) {
    if (err?.name === "NotAllowedError") return "Camera or microphone permission was denied.";
    if (err?.name === "NotFoundError") return "No usable camera or microphone was found.";
    if (err?.name === "NotReadableError") return "Hardware in use by another application.";
    return err?.message || "Could not start call.";
  }

  function startCallTimer() {
    state.callStartTime = Date.now();
    if (state.callDurationTimer) clearInterval(state.callDurationTimer);
    state.callDurationTimer = setInterval(() => {
      const elapsed = Math.floor((Date.now() - state.callStartTime) / 1000);
      const mins = String(Math.floor(elapsed / 60)).padStart(2, "0");
      const secs = String(elapsed % 60).padStart(2, "0");
      if (els.callDurationText) els.callDurationText.textContent = `LIVE [${mins}:${secs}]`;
    }, 1000);
  }

  function toggleDrawer() {
    if (!els.telemetryDrawer) return;
    const isCollapsed = els.telemetryDrawer.classList.toggle("collapsed");
    if (els.toggleDrawerButton) {
      els.toggleDrawerButton.setAttribute("aria-expanded", String(!isCollapsed));
    }
  }

  function toggleMute() {
    if (!state.localStream) return;
    const track = state.localStream.getAudioTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    const muted = !track.enabled;
    els.muteButton.setAttribute("aria-pressed", String(muted));
    els.muteButton.querySelector(".dock-label").textContent = muted ? "MIC OFF" : "MIC ON";
    playSound(muted ? "mute" : "unmute");
  }

  function toggleCamera() {
    if (!state.localStream) return;
    const track = state.localStream.getVideoTracks()[0];
    if (!track) return;
    track.enabled = !track.enabled;
    const disabled = !track.enabled;
    els.cameraButton.setAttribute("aria-pressed", String(disabled));
    els.cameraButton.querySelector(".dock-label").textContent = disabled ? "CAM OFF" : "CAM ON";
  }

  async function toggleScreenShare() {
    if (state.isSharingScreen) {
      // Revert to camera
      state.isSharingScreen = false;
      els.shareScreenButton.setAttribute("aria-pressed", "false");
      els.localVideo.classList.remove("no-mirror");
      if (state.screenStream) {
        state.screenStream.getTracks().forEach(t => t.stop());
        state.screenStream = null;
      }
      const camStream = await navigator.mediaDevices.getUserMedia({ video: videoConstraints });
      const camTrack = camStream.getVideoTracks()[0];
      const sender = state.pc?.getSenders?.().find(s => s.track?.kind === "video");
      if (sender) sender.replaceTrack(camTrack);
      els.localVideo.srcObject = camStream;
      state.localStream = camStream;
    } else {
      // Start screen share
      try {
        state.screenStream = await navigator.mediaDevices.getDisplayMedia({ video: { cursor: "always" }, audio: false });
        const screenTrack = state.screenStream.getVideoTracks()[0];
        screenTrack.onended = () => toggleScreenShare();
        const sender = state.pc?.getSenders?.().find(s => s.track?.kind === "video");
        if (sender) sender.replaceTrack(screenTrack);
        els.localVideo.srcObject = state.screenStream;
        els.localVideo.classList.add("no-mirror");
        state.isSharingScreen = true;
        els.shareScreenButton.setAttribute("aria-pressed", "true");
      } catch (_) {}
    }
  }

  function toggleVoiceFx() {
    const isPressed = els.voiceFxButton.getAttribute("aria-pressed") === "true";
    els.voiceFxButton.setAttribute("aria-pressed", String(!isPressed));
  }

  async function switchCamera() {
    state.facingMode = state.facingMode === "user" ? "environment" : "user";
    if (!state.localStream) return;
    try {
      const oldTrack = state.localStream.getVideoTracks()[0];
      if (oldTrack) oldTrack.stop();
      const nextStream = await navigator.mediaDevices.getUserMedia({
        video: { ...videoConstraints, facingMode: { exact: state.facingMode } },
      });
      const nextTrack = nextStream.getVideoTracks()[0];
      state.localStream.removeTrack(oldTrack);
      state.localStream.addTrack(nextTrack);
      els.localVideo.srcObject = state.localStream;
      const sender = state.pc?.getSenders?.().find(s => s.track?.kind === "video");
      if (sender) sender.replaceTrack(nextTrack);
    } catch (_) {}
  }

  // ==========================================
  // WEBRTC PEER CONNECTION & SIGNALING
  // ==========================================

  function createPeerConnection() {
    if (state.pc) state.pc.close();
    state.pendingCandidates = [];
    state.selectedPairVerified = false;
    state.pc = new RTCPeerConnection({
      iceServers: state.config?.iceServers || [],
      bundlePolicy: "max-bundle",
      rtcpMuxPolicy: "require",
      iceTransportPolicy: "all",
      // Pre-gather a small pool during the green room so the first offer already
      // carries candidates instead of trickling them after negotiation starts.
      iceCandidatePoolSize: 4,
    });

    for (const track of state.localStream.getTracks()) {
      state.pc.addTrack(track, state.localStream);
    }

    // Ordering must happen before the first createOffer, so do it as soon as
    // the video transceiver exists.
    preferVideoCodec(state.pc);
    configureRtpPriorities(state.pc);

    state.pc.onicecandidate = ({ candidate }) => {
      if (!candidate || !state.ws || state.ws.readyState !== WebSocket.OPEN) return;
      sendMessage({
        type: "ice-candidate",
        room: state.room,
        payload: {
          candidate: candidate.candidate,
          sdpMid: candidate.sdpMid,
          sdpMLineIndex: candidate.sdpMLineIndex,
          usernameFragment: candidate.usernameFragment,
        }
      });
    };

    state.pc.ontrack = ({ streams }) => {
      state.remoteStream = streams[0];
      els.remoteVideo.srcObject = state.remoteStream;
      if (els.remotePlaceholder) els.remotePlaceholder.hidden = true;
      setStatus("Connected (P2P)");
      if (els.remotePeerName) els.remotePeerName.textContent = "PEER CONNECTED";
    };

    state.pc.onconnectionstatechange = () => {
      if (!state.pc) return;
      onMediaStateChange(state.pc.connectionState);
    };

    // Older Safari does not fire connectionstatechange reliably; iceConnectionState
    // covers the same transitions there.
    state.pc.oniceconnectionstatechange = () => {
      if (!state.pc) return;
      const ice = state.pc.iceConnectionState;
      if (ice === "failed" || ice === "disconnected") onMediaStateChange(ice);
    };
  }

  // ==========================================
  // CONNECTION RECOVERY
  // ==========================================

  function onMediaStateChange(mediaState) {
    if (mediaState === "connected" || mediaState === "completed") {
      clearTimer("disconnectTimer");
      state.iceRestartInFlight = false;
      setStatus("Connected");
      if (els.remotePlaceholder) els.remotePlaceholder.hidden = true;
      if (els.remotePeerName) els.remotePeerName.textContent = "PEER CONNECTED";
      playSound("join");
      return;
    }
    if (mediaState !== "disconnected" && mediaState !== "failed") return;

    if (els.remotePlaceholder) els.remotePlaceholder.hidden = false;

    // A brief "disconnected" is common on network handover and usually recovers
    // without intervention, so give it a grace period before forcing a restart.
    // "failed" never recovers on its own.
    const failed = mediaState === "failed";
    const grace = failed ? 0 : ICE_DISCONNECT_GRACE_MS;
    setStatus(failed ? "Media failed, retrying…" : "Connection unstable…");

    // "failed" is definitive, so it supersedes a grace timer still waiting on a
    // "disconnected" that turned out not to heal.
    if (state.disconnectTimer) {
      if (!failed) return;
      clearTimer("disconnectTimer");
    }
    state.disconnectTimer = setTimeout(() => {
      state.disconnectTimer = null;
      if (!state.pc) return;
      const current = state.pc.connectionState;
      if (current === "connected" || current === "completed") return; // healed itself
      attemptIceRestart();
    }, grace);
  }

  // Rebuilds the candidate pair without tearing down the call. Only the offering
  // side may do this: this signaling flow has a single offerer, so a restart
  // from the callee would collide with the caller's negotiation.
  async function attemptIceRestart() {
    if (!state.pc || state.iceRestartInFlight || state.terminal) return;
    if (state.role !== "caller") {
      setStatus("Connection lost, waiting for peer to re-establish…");
      return;
    }
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
      // Nothing to carry the offer. The signaling reconnect re-offers on rejoin.
      setStatus("Waiting for signaling before retrying media…");
      return;
    }
    state.iceRestartInFlight = true;
    setStatus("Re-establishing media…");
    try {
      await makeOffer({ iceRestart: true });
    } finally {
      state.iceRestartInFlight = false;
    }
  }

  function clearTimer(name) {
    if (state[name]) {
      clearTimeout(state[name]);
      state[name] = null;
    }
  }

  // The peer's socket dropped, but the server keeps the room alive for its grace
  // window, so treat this as recoverable until the window closes.
  function startPeerReturnTimer() {
    clearTimer("peerGoneTimer");
    state.peerGoneTimer = setTimeout(() => {
      state.peerGoneTimer = null;
      setStatus("Peer did not return");
      if (els.remotePeerName) els.remotePeerName.textContent = "PEER LEFT THE CALL";
    }, PEER_RETURN_TIMEOUT_MS);
  }

  function onPeerPresent() {
    clearTimer("peerGoneTimer");
    state.reconnectAttempts = 0;
  }

  function scheduleSignalingReconnect() {
    if (state.intentionalClose || state.terminal || !state.started) return;
    if (state.reconnectTimer) return;

    // Exponential backoff with jitter, so two peers that dropped together do not
    // retry in lockstep.
    const attempt = state.reconnectAttempts++;
    const backoff = Math.min(SIGNALING_RETRY_MAX_MS, SIGNALING_RETRY_BASE_MS * 2 ** attempt);
    const delay = Math.round(backoff * (0.5 + Math.random() * 0.5));

    setStatus(`Signaling lost, retrying in ${Math.max(1, Math.round(delay / 1000))}s…`);
    state.reconnectTimer = setTimeout(() => {
      state.reconnectTimer = null;
      connectWebSocket();
    }, delay);
  }

  async function makeOffer(options = {}) {
    if (state.makingOffer || !state.pc) return;
    // An offer that cannot be sent would leave us in have-local-offer with no
    // answer coming, blocking every later negotiation.
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) return;
    // A negotiation is already in flight; its answer will settle this.
    if (state.pc.signalingState !== "stable") return;
    try {
      state.makingOffer = true;
      const offer = await state.pc.createOffer(options);
      const enhancedSDP = enhanceOpusSDP(offer.sdp);
      await state.pc.setLocalDescription({ type: "offer", sdp: enhancedSDP });
      configureRtpPriorities(state.pc);
      sendMessage({
        type: "offer",
        room: state.room,
        payload: { type: "offer", sdp: enhancedSDP }
      });
    } catch (e) {
      console.error("Failed to create WebRTC offer:", e);
    } finally {
      state.makingOffer = false;
    }
  }

  function connectWebSocket() {
    if (state.ws) {
      // Detach first: the old socket's onclose would otherwise queue a second
      // reconnect on top of this one.
      state.ws.onopen = state.ws.onmessage = state.ws.onclose = state.ws.onerror = null;
      state.ws.close();
    }
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${location.host}/ws/${encodeURIComponent(state.room)}?client_id=${encodeURIComponent(state.clientID)}`;
    state.ws = new WebSocket(url);

    state.ws.onopen = () => {
      // First message must be exact Join structure matching Go backend protocol
      sendMessage({
        type: "join",
        room: state.room,
        payload: {
          secret: state.secret,
          clientId: state.clientID
        }
      });
    };

    state.ws.onmessage = async ({ data }) => {
      try {
        const msg = JSON.parse(data);
        await handleSignalingMessage(msg);
      } catch (e) {
        console.error("Failed to parse signaling message:", e);
      }
    };

    state.ws.onerror = () => {
      // onclose always follows, which is where the retry is scheduled.
      console.warn("Signaling socket error");
    };

    state.ws.onclose = () => {
      if (state.intentionalClose || !state.started) return;
      // Media may still be flowing peer-to-peer; only signaling is gone.
      scheduleSignalingReconnect();
    };
  }

  function sendMessage(msg) {
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify(msg));
    }
  }

  function initDraggablePip() {
    const pip = els.selfPipCard;
    if (!pip) return;

    let isDragging = false;
    let startX = 0, startY = 0;
    let startLeft = 0, startTop = 0;
    let parentRect = null;

    pip.addEventListener("pointerdown", (e) => {
      if (e.target.closest("button")) return;
      isDragging = true;
      pip.classList.add("dragging");
      pip.setPointerCapture(e.pointerId);

      const rect = pip.getBoundingClientRect();
      const parent = pip.parentElement;
      parentRect = parent.getBoundingClientRect();

      startX = e.clientX;
      startY = e.clientY;
      startLeft = rect.left - parentRect.left;
      startTop = rect.top - parentRect.top;

      pip.style.left = `${startLeft}px`;
      pip.style.top = `${startTop}px`;
      pip.style.right = "auto";
      pip.style.bottom = "auto";
    });

    pip.addEventListener("pointermove", (e) => {
      if (!isDragging) return;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;

      const pipWidth = pip.offsetWidth;
      const pipHeight = pip.offsetHeight;
      const maxLeft = parentRect.width - pipWidth - 8;
      const maxTop = parentRect.height - pipHeight - 8;

      const newLeft = Math.max(8, Math.min(maxLeft, startLeft + dx));
      const newTop = Math.max(8, Math.min(maxTop, startTop + dy));

      pip.style.left = `${newLeft}px`;
      pip.style.top = `${newTop}px`;
    });

    const endDrag = (e) => {
      if (!isDragging) return;
      isDragging = false;
      pip.classList.remove("dragging");
      try { pip.releasePointerCapture(e.pointerId); } catch (_) {}

      if (!parentRect) return;
      const pipWidth = pip.offsetWidth;
      const pipHeight = pip.offsetHeight;
      const currentLeft = parseFloat(pip.style.left) || 0;
      const currentTop = parseFloat(pip.style.top) || 0;
      const inset = 16;

      const snapToRight = currentLeft >= (parentRect.width - pipWidth) / 2;
      const snapToBottom = currentTop >= (parentRect.height - pipHeight) / 2;

      if (snapToRight) {
        pip.style.left = "auto";
        pip.style.right = `${inset}px`;
      } else {
        pip.style.left = `${inset}px`;
        pip.style.right = "auto";
      }

      if (snapToBottom) {
        pip.style.top = "auto";
        pip.style.bottom = `${inset}px`;
      } else {
        pip.style.top = `${inset}px`;
        pip.style.bottom = "auto";
      }
    };

    pip.addEventListener("pointerup", endDrag);
    pip.addEventListener("pointercancel", endDrag);
  }

  async function handleSignalingMessage(msg) {
    let payload = msg.payload;
    if (typeof payload === "string") {
      try { payload = JSON.parse(payload); } catch (_) {}
    }

    switch (msg.type) {
      case "joined": {
        state.role = payload?.role || "caller";
        state.reconnectAttempts = 0; // a completed join proves the path works
        if (payload?.participants === 2) {
          onPeerPresent();
          const media = state.pc?.connectionState;
          const mediaHealthy = media === "connected" || media === "completed";
          if (mediaHealthy) {
            // This was a signaling-only reconnect. Renegotiating here would
            // disturb a call that never actually broke.
            setStatus("Connected");
          } else {
            setStatus("Peer connected, establishing media…");
            if (els.remotePeerName) els.remotePeerName.textContent = "PEER JOINED (CONNECTING)…";
            if (state.role === "caller") makeOffer();
          }
        } else {
          setStatus("Waiting for peer to join…");
          if (els.remotePeerName) els.remotePeerName.textContent = "WAITING FOR PEER TO JOIN…";
        }
        break;
      }

      case "peer-ready":
        // The peer just (re)entered the room, so its peer connection is fresh
        // even if ours still believes it is connected. Always renegotiate.
        onPeerPresent();
        setStatus("Peer connected, establishing media…");
        if (els.remotePeerName) els.remotePeerName.textContent = "PEER JOINED (CONNECTING)…";
        if (state.role === "caller") makeOffer();
        break;

      case "offer": {
        if (!payload?.sdp || !state.pc) return;
        if (els.remotePeerName) els.remotePeerName.textContent = "PEER CONNECTING…";
        try {
          await state.pc.setRemoteDescription({ type: "offer", sdp: payload.sdp });
          await drainCandidates();
          const answer = await state.pc.createAnswer();
          const enhancedSDP = enhanceOpusSDP(answer.sdp);
          await state.pc.setLocalDescription({ type: "answer", sdp: enhancedSDP });
          configureRtpPriorities(state.pc);
          sendMessage({
            type: "answer",
            room: state.room,
            payload: { type: "answer", sdp: enhancedSDP }
          });
        } catch (e) {
          console.error("Failed to answer offer:", e);
          setStatus("Could not negotiate media, retrying…");
        }
        break;
      }

      case "answer":
        if (!payload?.sdp || !state.pc) return;
        // A late answer to a superseded offer arrives while we are back in
        // stable; applying it would throw and poison the connection.
        if (state.pc.signalingState !== "have-local-offer") {
          console.warn("Ignoring answer in state", state.pc.signalingState);
          return;
        }
        try {
          await state.pc.setRemoteDescription({ type: "answer", sdp: payload.sdp });
          if (els.remotePeerName) els.remotePeerName.textContent = "PEER CONNECTED";
          await drainCandidates();
        } catch (e) {
          console.error("Failed to apply answer:", e);
        }
        break;

      case "ice-candidate":
        if (payload?.candidate) {
          const candidateInit = {
            candidate: payload.candidate,
            sdpMid: payload.sdpMid,
            sdpMLineIndex: payload.sdpMLineIndex,
          };
          if (state.pc && state.pc.remoteDescription && state.pc.remoteDescription.type) {
            try { await state.pc.addIceCandidate(candidateInit); } catch (_) {}
          } else {
            state.pendingCandidates.push(candidateInit);
          }
        }
        break;

      case "hangup":
        // Deliberate end of call by the peer, not a network drop: the server has
        // already deleted the room, so no recovery is possible or wanted.
        state.terminal = true;
        clearTimer("reconnectTimer");
        clearTimer("disconnectTimer");
        clearTimer("peerGoneTimer");
        setStatus("Peer ended the call");
        if (els.remotePeerName) els.remotePeerName.textContent = "PEER ENDED THE CALL";
        if (els.remotePlaceholder) els.remotePlaceholder.hidden = false;
        playSound("leave");
        break;

      case "peer-left":
        // Distinct from "hangup": the peer's socket dropped, and the server
        // keeps the room for its grace window, so they may still come back.
        setStatus("Peer disconnected, waiting for them to return…");
        if (els.remotePeerName) els.remotePeerName.textContent = "PEER RECONNECTING…";
        if (els.remotePlaceholder) els.remotePlaceholder.hidden = false;
        playSound("leave");
        startPeerReturnTimer();
        break;

      case "error":
        console.warn("Signaling error received:", payload);
        if (TERMINAL_SIGNALING_CODES.has(payload?.code)) {
          // Retrying cannot fix these, and a reconnect loop would hammer the
          // server and hide the real reason from the user.
          state.terminal = true;
          clearTimer("reconnectTimer");
          clearTimer("disconnectTimer");
          clearTimer("peerGoneTimer");
          setStatus(payload?.message || "Call is no longer available");
          if (els.remotePeerName) els.remotePeerName.textContent = "CALL UNAVAILABLE";
        } else if (payload?.message) {
          setStatus(`Signaling error: ${payload.message}`);
        }
        break;
    }
  }

  async function drainCandidates() {
    for (const c of state.pendingCandidates) {
      try { await state.pc.addIceCandidate(c); } catch (_) {}
    }
    state.pendingCandidates = [];
  }

  // ==========================================
  // LIVE WEBRTC TELEMETRY MATRIX
  // ==========================================

  function startStats() {
    if (state.statsTimer) clearInterval(state.statsTimer);
    state.statsTimer = setInterval(async () => {
      if (!state.pc || state.pc.connectionState !== "connected") return;
      try {
        const stats = await state.pc.getStats();
        // Everything below is read from the live report set. Nothing is
        // substituted when a value is missing: an unknown metric renders as
        // "--" rather than a plausible-looking guess.
        let bytesSent = 0, packetsSent = 0, packetsLost = 0;
        let rtt = null, width = 0, height = 0, fps = null;
        let codec = "";
        let candidateType = "Direct P2P";

        stats.forEach(report => {
          if (report.type === "outbound-rtp" && report.kind === "video") {
            bytesSent = report.bytesSent || 0;
            packetsSent = report.packetsSent || 0;
            width = report.frameWidth || 0;
            height = report.frameHeight || 0;
            if (typeof report.framesPerSecond === "number") fps = Math.round(report.framesPerSecond);
            const codecReport = report.codecId ? stats.get(report.codecId) : null;
            if (codecReport?.mimeType) codec = codecReport.mimeType.replace(/^video\//i, "");
          }
          if (report.type === "candidate-pair" && report.state === "succeeded") {
            if (typeof report.currentRoundTripTime === "number") {
              rtt = Math.round(report.currentRoundTripTime * 1000);
            }
            const local = stats.get(report.localCandidateId);
            if (local) candidateType = `${local.candidateType?.toUpperCase() || "HOST"} (${local.ip?.includes(":") ? "IPv6" : "IPv4"})`;
          }
          if (report.type === "remote-inbound-rtp") {
            packetsLost = report.packetsLost || 0;
          }
        });

        const now = Date.now();
        if (state.lastStatsAt > 0) {
          const bitrateMbps = (((bytesSent - state.lastBytesSent) * 8) / ((now - state.lastStatsAt) / 1000) / 1000000).toFixed(2);
          if (els.metricBitrate) els.metricBitrate.textContent = `${Math.max(0, bitrateMbps)} Mbps`;
        }
        state.lastBytesSent = bytesSent;
        state.lastStatsAt = now;

        const rttText = rtt === null ? "--" : String(rtt);
        const resolutionText = width && height
          ? `${width}x${height}${fps !== null ? ` @ ${fps}` : ""}`
          : "--";

        if (els.metricRtt) els.metricRtt.textContent = `${rttText} ms`;
        if (els.metricLoss) {
          const lossRate = packetsSent > 0 ? ((packetsLost / packetsSent) * 100).toFixed(1) : "0.0";
          els.metricLoss.textContent = `${lossRate}%`;
        }
        if (els.metricCandidate) els.metricCandidate.textContent = candidateType;
        if (els.metricResolution) els.metricResolution.textContent = resolutionText;
        if (els.metricCodec) els.metricCodec.textContent = codec || "--";

        if (els.callStatsBadge) {
          const parts = [];
          if (width && height) parts.push(`${height}P${fps !== null ? ` ${fps}FPS` : ""}`);
          if (codec) parts.push(codec.toUpperCase());
          parts.push(`${rttText}ms RTT`);
          parts.push("DTLS-SRTP");
          els.callStatsBadge.textContent = parts.join(" // ");
        }
      } catch (_) {}
    }, 1500);
  }

  function applyQualityProfile(profile) {
    if (!state.pc) return;
    const senders = state.pc.getSenders();
    const videoSender = senders.find(s => s.track && s.track.kind === "video");
    if (!videoSender || !videoSender.getParameters) return;

    const params = videoSender.getParameters();
    if (!params.encodings || params.encodings.length === 0) params.encodings = [{}];

    if (profile === "low") {
      params.encodings[0].maxBitrate = 350000;
      params.encodings[0].maxFramerate = 15;
    } else if (profile === "quality") {
      params.encodings[0].maxBitrate = 4000000;
      params.encodings[0].maxFramerate = 60;
    } else {
      delete params.encodings[0].maxBitrate;
      delete params.encodings[0].maxFramerate;
    }
    videoSender.setParameters(params).catch(() => {});
  }

  function playRemote() {
    if (!els.remoteVideo) return;
    els.remoteVideo.play().then(() => {
      if (els.playRemoteButton) els.playRemoteButton.hidden = true;
    }).catch(() => {
      if (els.playRemoteButton) els.playRemoteButton.hidden = false;
    });
  }

  function hangup(manual = false) {
    state.intentionalClose = manual;
    closeLocalResources(true);
    location.href = "/";
  }

  function closeLocalResources(leaveRoom = true) {
    if (state.callDurationTimer) clearInterval(state.callDurationTimer);
    if (state.statsTimer) clearInterval(state.statsTimer);
    clearTimer("reconnectTimer");
    clearTimer("disconnectTimer");
    clearTimer("peerGoneTimer");
    stopAudioAnalyser();
    stopPreviewMedia();

    if (state.localStream) {
      state.localStream.getTracks().forEach(t => t.stop());
      state.localStream = null;
    }
    if (state.screenStream) {
      state.screenStream.getTracks().forEach(t => t.stop());
      state.screenStream = null;
    }
    if (state.pc) {
      state.pc.close();
      state.pc = null;
    }
    if (state.ws) {
      if (leaveRoom && state.ws.readyState === WebSocket.OPEN) {
        sendMessage({ type: "leave" });
      }
      state.ws.onopen = state.ws.onmessage = state.ws.onclose = state.ws.onerror = null;
      state.ws.close();
      state.ws = null;
    }
  }

  window.addEventListener("popstate", () => {
    handleRoute();
  });

  initialize();
})();
