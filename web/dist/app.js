// OmniAgent web UI shell. One capability-driven SPA for both personal and
// team deployments (TRD §1a/§6) — no build step, no external assets.
(function () {
  "use strict";

  var app = document.getElementById("app");
  var nav = document.getElementById("nav");

  // chatTeardown releases the previous chat view's WebSocket/timers before a
  // re-render (e.g. logout) so we never leak a socket per render().
  var chatTeardown = null;

  function csrfHeaders(extra) {
    var h = Object.assign({ "Content-Type": "application/json" }, extra || {});
    return h;
  }

  function fetchCapabilities() {
    return fetch("/api/capabilities", { credentials: "same-origin" }).then(function (res) {
      if (!res.ok) throw new Error("capabilities request failed: " + res.status);
      return res.json();
    });
  }

  function fetchMe() {
    return fetch("/api/auth/me", { credentials: "same-origin" }).then(function (res) {
      if (res.status === 401) return null;
      if (!res.ok) throw new Error("me request failed: " + res.status);
      return res.json();
    });
  }

  function renderNav(caps, me) {
    nav.innerHTML = "";
    if (caps.authRequired && me) {
      var logout = document.createElement("button");
      logout.textContent = "Log out (" + me.username + ")";
      logout.addEventListener("click", function () {
        fetch("/api/auth/logout", { method: "POST", credentials: "same-origin", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }) })
          .then(render);
      });
      nav.appendChild(logout);
    }
  }

  function renderCapabilityList(caps) {
    var ul = document.createElement("ul");
    ul.className = "caps";
    [
      ["Multi-user", caps.multiUser],
      ["Auth required", caps.authRequired],
      ["Group chats", caps.groupChats],
      ["Admin", caps.admin],
      ["Catalog", caps.catalog],
    ].forEach(function (pair) {
      var li = document.createElement("li");
      var label = document.createElement("span");
      label.textContent = pair[0];
      var value = document.createElement("span");
      value.className = pair[1] ? "on" : "off";
      value.textContent = pair[1] ? "on" : "off";
      li.appendChild(label);
      li.appendChild(value);
      ul.appendChild(li);
    });
    return ul;
  }

  function renderLogin() {
    var section = document.createElement("section");
    var heading = document.createElement("p");
    heading.textContent = "Sign in with a magic link:";
    var form = document.createElement("form");
    form.className = "login";
    var input = document.createElement("input");
    input.type = "email";
    input.required = true;
    input.placeholder = "you@example.com";
    var button = document.createElement("button");
    button.type = "submit";
    button.textContent = "Send link";
    var status = document.createElement("p");
    status.className = "loading";
    form.appendChild(input);
    form.appendChild(button);
    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      status.textContent = "Sending…";
      fetch("/api/auth/magic-link", {
        method: "POST",
        credentials: "same-origin",
        headers: csrfHeaders(),
        body: JSON.stringify({ email: input.value }),
      })
        .then(function () {
          status.textContent = "If that address is allowed, a sign-in link is on its way.";
        })
        .catch(function () {
          status.textContent = "Could not reach the server. Try again.";
        });
    });
    section.appendChild(heading);
    section.appendChild(form);
    section.appendChild(status);
    return section;
  }

  function fetchChat() {
    return fetch("/api/chat", { credentials: "same-origin" }).then(function (res) {
      if (!res.ok) throw new Error("chat request failed: " + res.status);
      return res.json();
    });
  }

  function fetchOlder(before, limit) {
    var url = "/api/chat/history?limit=" + limit + (before ? "&before=" + encodeURIComponent(before) : "");
    return fetch(url, { credentials: "same-origin" }).then(function (res) {
      if (!res.ok) throw new Error("history request failed: " + res.status);
      return res.json();
    });
  }

  function messageNode(msg) {
    var li = document.createElement("li");
    li.className = "msg msg-" + msg.authorType;
    if (msg.id) li.dataset.id = msg.id;
    var author = document.createElement("span");
    author.className = "msg-author";
    author.textContent = msg.authorType === "agent" ? "Agent" : "You";
    var content = document.createElement("span");
    content.className = "msg-content";
    content.textContent = msg.content;
    li.appendChild(author);
    li.appendChild(content);
    return li;
  }

  // renderChat builds the personal-mode chat panel: keyset scroll-back
  // history (GET /api/chat + /api/chat/history) and live agent replies over
  // the WebSocket (POST /api/chat/messages returns immediately; the reply
  // arrives as a chat.message event). Personal mode only — team mode's chat
  // surface is a separate, not-yet-built endpoint set behind auth.
  function renderChat() {
    var PAGE = 50;
    var section = document.createElement("section");
    section.className = "chat";
    var list = document.createElement("ul");
    list.className = "messages";
    var status = document.createElement("p");
    status.className = "loading";
    status.textContent = "Loading chat…";

    var form = document.createElement("form");
    form.className = "send";
    var input = document.createElement("input");
    input.type = "text";
    input.placeholder = "Message the agent…";
    input.autocomplete = "off";
    var button = document.createElement("button");
    button.type = "submit";
    button.textContent = "Send";
    form.appendChild(input);
    form.appendChild(button);

    var state = {
      oldestId: null, // ID of the topmost loaded message (scroll-back cursor)
      hasMore: false,
      loadingOlder: false,
      ids: Object.create(null), // rendered message IDs, for dedupe on reload
      ws: null,
      reconnectTimer: null,
      replyTimer: null,
      pending: false,
      closed: false,
    };

    function seen(id) {
      if (!id) return false;
      if (state.ids[id]) return true;
      state.ids[id] = true;
      return false;
    }

    function typingIndicator(on) {
      var existing = list.querySelector(".msg-typing");
      if (on && !existing) {
        var li = document.createElement("li");
        li.className = "msg msg-agent msg-typing";
        li.textContent = "Agent is typing…";
        list.appendChild(li);
        list.scrollTop = list.scrollHeight;
      } else if (!on && existing) {
        existing.remove();
      }
    }

    function setPending(on) {
      state.pending = on;
      input.disabled = on;
      button.disabled = on;
      typingIndicator(on);
      if (state.replyTimer) {
        clearTimeout(state.replyTimer);
        state.replyTimer = null;
      }
      if (on) {
        // Safety net: never leave the composer stuck if the WS reply is lost.
        state.replyTimer = setTimeout(function () {
          if (state.pending) {
            appendAgent({ authorType: "agent", content: "(no response — reload to check history)" });
            setPending(false);
          }
        }, 120000);
      } else if (!state.closed) {
        input.focus();
      }
    }

    function appendAgent(msg) {
      if (seen(msg.id)) return;
      var atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
      list.appendChild(messageNode(msg));
      if (atBottom) list.scrollTop = list.scrollHeight;
    }

    function loadOlder() {
      if (state.loadingOlder || !state.hasMore) return;
      state.loadingOlder = true;
      var prevHeight = list.scrollHeight;
      fetchOlder(state.oldestId, PAGE)
        .then(function (body) {
          var frag = document.createDocumentFragment();
          body.messages.forEach(function (m) {
            if (seen(m.id)) return;
            frag.appendChild(messageNode(m));
          });
          list.insertBefore(frag, list.firstChild);
          if (body.messages.length) state.oldestId = body.messages[0].id;
          state.hasMore = body.hasMore;
          // Keep the viewport anchored on the message the user was reading.
          list.scrollTop += list.scrollHeight - prevHeight;
        })
        .catch(function (err) {
          status.className = "error";
          status.textContent = "Failed to load older messages: " + err.message;
        })
        .then(function () {
          state.loadingOlder = false;
        });
    }

    list.addEventListener("scroll", function () {
      if (list.scrollTop < 40) loadOlder();
    });

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var content = input.value.trim();
      if (!content || state.pending) return;
      input.value = "";
      // Optimistic local echo of the user's message.
      list.appendChild(messageNode({ authorType: "user", content: content }));
      list.scrollTop = list.scrollHeight;
      setPending(true);
      fetch("/api/chat/messages", {
        method: "POST",
        credentials: "same-origin",
        headers: csrfHeaders(),
        body: JSON.stringify({ content: content }),
      })
        .then(function (res) {
          if (!res.ok) throw new Error("send failed: " + res.status);
          return res.json();
        })
        .then(function (body) {
          // Record the persisted user-message ID so a reconnect reload does
          // not duplicate it. The agent reply arrives over the WS.
          if (body.user && body.user.id) seen(body.user.id);
        })
        .catch(function (err) {
          appendAgent({ authorType: "agent", content: "(error: " + err.message + ")" });
          setPending(false);
        });
    });

    // ---- WebSocket: live agent replies -----------------------------------

    function wsURL() {
      var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
      return proto + "//" + window.location.host + "/ws";
    }

    function reloadTail() {
      // After a reconnect, resync from the server in case a reply landed
      // while we were disconnected.
      fetchChat()
        .then(function (chat) {
          list.innerHTML = "";
          state.ids = Object.create(null);
          chat.messages.forEach(function (m) {
            seen(m.id);
            list.appendChild(messageNode(m));
          });
          state.oldestId = chat.messages.length ? chat.messages[0].id : null;
          state.hasMore = chat.hasMore;
          list.scrollTop = list.scrollHeight;
          if (state.pending) setPending(false);
        })
        .catch(function () {
          /* best-effort resync */
        });
    }

    function connectWS(isReconnect) {
      if (state.closed) return;
      var ws;
      try {
        ws = new WebSocket(wsURL());
      } catch (e) {
        scheduleReconnect();
        return;
      }
      state.ws = ws;
      ws.addEventListener("open", function () {
        if (isReconnect) reloadTail();
      });
      ws.addEventListener("message", function (ev) {
        var msg;
        try {
          msg = JSON.parse(ev.data);
        } catch (e) {
          return;
        }
        if (msg.type === "event" && msg.content === "chat.message" && msg.data) {
          appendAgent({ id: msg.data.id, authorType: msg.data.authorType || "agent", content: msg.data.content });
          if (state.pending) setPending(false);
        } else if (msg.type === "error") {
          appendAgent({ authorType: "agent", content: "(error: " + (msg.error || "agent reply failed") + ")" });
          if (state.pending) setPending(false);
        }
      });
      ws.addEventListener("close", function () {
        state.ws = null;
        scheduleReconnect();
      });
      ws.addEventListener("error", function () {
        ws.close();
      });
    }

    function scheduleReconnect() {
      if (state.closed || state.reconnectTimer) return;
      state.reconnectTimer = setTimeout(function () {
        state.reconnectTimer = null;
        connectWS(true);
      }, 2000);
    }

    chatTeardown = function () {
      state.closed = true;
      if (state.reconnectTimer) clearTimeout(state.reconnectTimer);
      if (state.replyTimer) clearTimeout(state.replyTimer);
      if (state.ws) {
        try {
          state.ws.close();
        } catch (e) {
          /* ignore */
        }
      }
    };

    section.appendChild(list);
    section.appendChild(status);
    section.appendChild(form);

    fetchChat()
      .then(function (chat) {
        status.remove();
        chat.messages.forEach(function (m) {
          seen(m.id);
          list.appendChild(messageNode(m));
        });
        state.oldestId = chat.messages.length ? chat.messages[0].id : null;
        state.hasMore = chat.hasMore;
        list.scrollTop = list.scrollHeight;
        input.focus();
        connectWS(false);
      })
      .catch(function (err) {
        status.className = "error";
        status.textContent = "Failed to load chat: " + err.message;
      });

    return section;
  }

  function render() {
    if (chatTeardown) {
      chatTeardown();
      chatTeardown = null;
    }
    app.innerHTML = "";
    fetchCapabilities()
      .then(function (caps) {
        var showLogin = caps.authRequired;
        return (showLogin ? fetchMe() : Promise.resolve(null)).then(function (me) {
          renderNav(caps, me);
          if (showLogin && !me) {
            app.appendChild(renderLogin());
            return;
          }
          if (!caps.multiUser) {
            app.appendChild(renderChat());
            return;
          }
          // Team mode's chat UI isn't built yet — show the active
          // capability set so the mode is still visible.
          app.appendChild(renderCapabilityList(caps));
        });
      })
      .catch(function (err) {
        var p = document.createElement("p");
        p.className = "error";
        p.textContent = "Failed to load: " + err.message;
        app.appendChild(p);
      });
  }

  render();
})();
