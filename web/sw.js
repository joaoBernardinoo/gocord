// Gocord Service Worker - Privacy-Preserving Push Notifications
"use strict";

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  let data = {};
  if (event.data) {
    try {
      data = event.data.json();
    } catch (_) {
      try {
        data = { body: event.data.text() };
      } catch (__) {
        data = {};
      }
    }
  }

  const callerName = data.callerName || "";
  const title = data.title || (callerName ? `Call from ${callerName}` : "Incoming Call // Gocord");
  const body = data.body || (callerName ? `${callerName} is calling you on Gocord` : "Incoming encrypted peer-to-peer call");
  const callUrl = data.url || "/";
  const room = data.room || "";

  const options = {
    body: body,
    icon: "/assets/favicon.ico",
    badge: "/assets/favicon.ico",
    tag: room ? `gocord-call-${room}` : "gocord-call",
    renotify: true,
    requireInteraction: true,
    data: {
      url: callUrl,
      room: room,
      timestamp: Date.now(),
    },
    actions: [
      { action: "join", title: "✦ JOIN CALL" },
      { action: "dismiss", title: "✕ DISMISS" },
    ],
    vibrate: [200, 100, 200, 100, 400],
  };

  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();

  if (event.action === "dismiss") {
    return;
  }

  const targetUrl = event.notification.data?.url || "/";

  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((windowClients) => {
      // If a window is already on the exact call URL, focus it
      for (const client of windowClients) {
        if (client.url === targetUrl && "focus" in client) {
          return client.focus();
        }
      }
      // If any window from this origin is open, navigate it to the call URL and focus
      if (windowClients.length > 0 && "navigate" in windowClients[0] && "focus" in windowClients[0]) {
        return windowClients[0].navigate(targetUrl).then((c) => (c ? c.focus() : null));
      }
      // Otherwise open a new window
      if (self.clients.openWindow) {
        return self.clients.openWindow(targetUrl);
      }
    })
  );
});
