// OmniAgent web UI shell. One capability-driven SPA for both personal and
// team deployments (TRD §1a/§6) — no build step, no external assets.
(function () {
  "use strict";

  var app = document.getElementById("app");
  var nav = document.getElementById("nav");
  var navExtra = document.getElementById("nav-extra");

  // chatTeardown releases the previous chat view's WebSocket/timers before a
  // re-render (e.g. logout) so we never leak a socket per render().
  var chatTeardown = null;

  // Team-mode view router state (RMI-311/312/313). One SPA, several surfaces:
  // "chat" (DMs + groups), "catalog" (discovery), "agents" (owner/maintainer
  // config), "curation" (superadmin featured). currentCaps/currentMe are the
  // last-fetched capability + identity so tab switches re-render without a
  // round-trip. pendingOpenChat lets the catalog hand a freshly-started chat to
  // the chat view to auto-open.
  var teamView = "chat";
  var currentCaps = null;
  var currentMe = null;
  var pendingOpenChat = null;

  function csrfHeaders(extra) {
    var h = Object.assign({ "Content-Type": "application/json" }, extra || {});
    return h;
  }

  // el is a small DOM builder used by the agents/catalog/curation views:
  // el("div", {className:"x", onclick:fn}, [child, "text"]). Text is always set
  // via textContent / text nodes (never innerHTML), so user-supplied strings
  // cannot inject markup. Kept local rather than retrofitting the older chat
  // views, which predate it.
  function el(tag, props, children) {
    var node = document.createElement(tag);
    if (props) {
      Object.keys(props).forEach(function (k) {
        if (k === "className") node.className = props[k];
        else if (k === "text") node.textContent = props[k];
        else if (k === "onclick") node.addEventListener("click", props[k]);
        else node[k] = props[k];
      });
    }
    (children || []).forEach(function (c) {
      if (c == null) return;
      node.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
    });
    return node;
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

  // attachSpeechToText wires a mic button to dictate into a text input via
  // the browser's native SpeechRecognition API — no backend involved. Single-
  // utterance (not the continuous/auto-restart-on-silence pattern some voice
  // assistants use), since this dictates one composer message, not an open
  // mic. Hides the button outright when the browser has no support, rather
  // than failing on click.
  function attachSpeechToText(button, input) {
    var Recognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!Recognition) {
      button.hidden = true;
      return;
    }
    var recognition = new Recognition();
    recognition.continuous = false;
    recognition.interimResults = true;
    var listening = false;

    function stop() {
      listening = false;
      button.classList.remove("recording");
      try { recognition.stop(); } catch (e) { /* already stopped */ }
    }

    recognition.onresult = function (event) {
      var transcript = "";
      for (var i = 0; i < event.results.length; i++) {
        transcript += event.results[i][0].transcript;
      }
      input.value = transcript;
    };
    recognition.onerror = function () { stop(); };
    recognition.onend = function () { stop(); };

    button.addEventListener("click", function () {
      if (listening) {
        stop();
        return;
      }
      listening = true;
      button.classList.add("recording");
      try {
        recognition.start();
      } catch (e) {
        stop();
      }
    });
  }

  // attachTranslate wires a translate button + language popover to POST the
  // input's current text to /api/translate and replace it with the response.
  // Only ever called when currentCaps.translate is true (a deployment-wide
  // LLM is configured — see config/capabilities.go).
  var TRANSLATE_LANGS = ["Spanish", "French", "German", "Chinese", "Japanese", "Korean", "Portuguese", "Italian"];

  function attachTranslate(button, input) {
    var menu = document.createElement("div");
    menu.className = "translate-menu";
    menu.hidden = true;
    TRANSLATE_LANGS.forEach(function (lang) {
      var langBtn = document.createElement("button");
      langBtn.type = "button";
      langBtn.textContent = lang;
      langBtn.addEventListener("click", function () {
        var text = input.value.trim();
        menu.hidden = true;
        if (!text) return;
        jsonFetch("/api/translate", {
          method: "POST",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({ text: text, targetLang: lang }),
        })
          .then(function (body) {
            if (body && body.translation) input.value = body.translation;
          })
          .catch(function () { /* leave input unchanged on failure */ });
      });
      menu.appendChild(langBtn);
    });
    button.addEventListener("click", function () {
      menu.hidden = !menu.hidden;
    });
    document.addEventListener("click", function (ev) {
      if (!menu.hidden && ev.target !== button && !menu.contains(ev.target)) menu.hidden = true;
    });
    button.insertAdjacentElement("afterend", menu);
  }

  function renderNav(caps, me) {
    nav.innerHTML = "";
    // Team-mode view tabs (RMI-311/312/313): Chats, Catalog, My Agents, and
    // Curation (superadmin only). Personal mode has a single surface and no tabs.
    if (caps.multiUser && me) {
      var tabs = [["chat", "Chats"], ["catalog", "Catalog"], ["agents", "My Agents"]];
      if (me.superadmin) tabs.push(["curation", "Curation"]);
      if (me.superadmin) tabs.push(["admin", "Admin"]);
      tabs.push(["account", "Account"]);
      tabs.forEach(function (t) {
        nav.appendChild(el("button", {
          type: "button",
          text: t[1],
          className: "nav-tab" + (teamView === t[0] ? " active" : ""),
          onclick: function () { switchTeamView(t[0]); },
        }));
      });
    }
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

  // switchTeamView changes the active team-mode surface: it re-renders the nav
  // (to move the active marker) and mounts the chosen view. Used by the tabs and
  // by the catalog after starting a chat.
  function switchTeamView(name) {
    teamView = name;
    renderNav(currentCaps, currentMe);
    mountTeamView();
  }

  // mountTeamView renders the current team surface into #app, first tearing down
  // any live chat view so its WebSocket/timers do not leak across a switch.
  function mountTeamView() {
    if (chatTeardown) {
      chatTeardown();
      chatTeardown = null;
    }
    app.innerHTML = "";
    navExtra.innerHTML = "";
    var me = currentMe;
    if (teamView === "catalog") {
      app.appendChild(renderCatalog(me));
    } else if (teamView === "agents") {
      app.appendChild(renderAgents(me));
    } else if (teamView === "curation" && me && me.superadmin) {
      app.appendChild(renderCuration(me));
    } else if (teamView === "admin" && me && me.superadmin) {
      app.appendChild(renderAdmin(me));
    } else if (teamView === "account") {
      app.appendChild(renderAccount(me));
    } else {
      teamView = "chat";
      app.appendChild(renderTeamChat(me));
    }
  }

  // renderAccount is the member's self-service surface: change your own
  // password (current required once one is set).
  function renderAccount() {
    var section = el("section", { className: "admin-area" });
    section.appendChild(el("h2", { className: "view-title", text: "Account" }));
    var status = el("p", { className: "error" });
    var current = el("input", { type: "password", placeholder: "current password (leave blank if none set)", autocomplete: "current-password" });
    var next = el("input", { type: "password", placeholder: "new password (min 8 chars)", autocomplete: "new-password" });
    var form = el("form", { className: "agent-form" }, [
      field("Current password", current), field("New password", next),
      el("div", { className: "form-actions" }, [el("button", { type: "submit", text: "Change password" })]),
      status,
    ]);
    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      status.className = "error"; status.textContent = "";
      jsonFetch("/api/users/me/password", {
        method: "POST", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        body: JSON.stringify({ current_password: current.value, new_password: next.value }),
      }).then(function () {
        current.value = ""; next.value = "";
        status.className = "ok"; status.textContent = "Password changed.";
      }).catch(function (err) {
        status.textContent = err.message === "unauthenticated" ? "" : err.message;
      });
    });
    section.appendChild(card("Change password", form));
    return section;
  }

  // ssoErrorMessage maps a ?error= query param (set by the magic-link and
  // SSO callback redirects) to a human-readable message. Unknown codes are
  // shown as-is so a future error code is never silently swallowed.
  var SSO_ERROR_MESSAGES = {
    invalid_link: "That sign-in link is invalid or has expired.",
    sso_state: "Sign-in could not be verified — please try again.",
    sso_failed: "Sign-in failed — please try again.",
    not_allowed: "That account is not permitted to sign in.",
    disabled: "That account has been disabled.",
  };

  function consumeLoginError() {
    var params = new URLSearchParams(location.search);
    var code = params.get("error");
    if (!code) return null;
    // Strip it so a reload doesn't re-show a stale error.
    params.delete("error");
    var qs = params.toString();
    history.replaceState(null, "", location.pathname + (qs ? "?" + qs : ""));
    return SSO_ERROR_MESSAGES[code] || code;
  }

  function renderLogin(caps) {
    var section = document.createElement("section");
    section.className = "narrow";
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

    // Email + password sign-in (additive to magic-link + SSO).
    var pwEmail = el("input", { type: "email", required: true, placeholder: "you@example.com", autocomplete: "username" });
    var pwPass = el("input", { type: "password", required: true, placeholder: "password", autocomplete: "current-password" });
    var pwStatus = el("p", { className: "error" });
    var pwForm = el("form", { className: "login" }, [pwEmail, pwPass, el("button", { type: "submit", text: "Sign in" })]);
    pwForm.addEventListener("submit", function (ev) {
      ev.preventDefault();
      pwStatus.className = "error"; pwStatus.textContent = "";
      fetch("/api/auth/password", {
        method: "POST", credentials: "same-origin", headers: csrfHeaders(),
        body: JSON.stringify({ email: pwEmail.value, password: pwPass.value }),
      }).then(function (res) {
        if (res.ok) { render(); return; }
        pwStatus.textContent = "Invalid email or password.";
      }).catch(function () { pwStatus.textContent = "Could not reach the server. Try again."; });
    });
    section.appendChild(el("p", { className: "muted", text: "…or sign in with a password:" }));
    section.appendChild(pwForm);
    section.appendChild(pwStatus);

    var loginError = consumeLoginError();
    if (loginError) {
      section.appendChild(el("p", { className: "error", text: loginError }));
    }

    if (caps && (caps.googleSso || caps.githubSso)) {
      var sso = el("div", { className: "sso-links" });
      // Real top-level navigations, not fetch — OAuth requires a full-page
      // redirect to the provider's consent screen.
      if (caps.googleSso) {
        sso.appendChild(el("a", { className: "sso-link", href: "/api/auth/google", text: "Sign in with Google" }));
      }
      if (caps.githubSso) {
        sso.appendChild(el("a", { className: "sso-link", href: "/api/auth/github", text: "Sign in with GitHub" }));
      }
      section.appendChild(sso);
    }

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
    section.className = "chat narrow";
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
    var micBtn = document.createElement("button");
    micBtn.type = "button";
    micBtn.className = "icon-btn";
    micBtn.title = "Dictate";
    micBtn.textContent = "🎤";
    attachSpeechToText(micBtn, input);
    var translateBtn = document.createElement("button");
    translateBtn.type = "button";
    translateBtn.className = "icon-btn";
    translateBtn.title = "Translate";
    translateBtn.textContent = "🌐";
    translateBtn.hidden = !currentCaps || !currentCaps.translate;
    if (!translateBtn.hidden) attachTranslate(translateBtn, input);
    var button = document.createElement("button");
    button.type = "submit";
    button.textContent = "Send";
    form.appendChild(input);
    form.appendChild(micBtn);
    form.appendChild(translateBtn);
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
      micBtn.disabled = on;
      translateBtn.disabled = on;
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

  // ---- Team mode: private DMs + group chats (RMI-118) --------------------

  function jsonFetch(url, opts) {
    return fetch(url, Object.assign({ credentials: "same-origin" }, opts || {})).then(function (res) {
      if (res.status === 401) {
        // Session expired mid-session; re-render drops us at the login screen.
        render();
        throw new Error("unauthenticated");
      }
      if (!res.ok) {
        return res.text().then(function (body) {
          var msg = res.status + "";
          try {
            var parsed = JSON.parse(body);
            if (parsed && parsed.error) msg = parsed.error;
          } catch (e) {
            /* non-JSON error body */
          }
          throw new Error(msg);
        });
      }
      if (res.status === 204) return null;
      return res.json();
    });
  }

  function chatLabel(chat) {
    if (chat.type === "private") return "Agent (DM)";
    return chat.name || "Untitled group";
  }

  // teamMessageNode renders one message attributed to its author: the agent,
  // yourself, or a named co-member (resolved from the member map).
  function teamMessageNode(msg, me, names) {
    var li = document.createElement("li");
    if (msg.id) li.dataset.id = msg.id;
    var label, cls;
    if (msg.authorType === "agent") {
      label = "Agent";
      cls = "msg-agent";
    } else if (me && msg.authorUserId && msg.authorUserId === me.user_id) {
      label = "You";
      cls = "msg-user";
    } else {
      label = (msg.authorUserId && names[msg.authorUserId]) || "Member";
      cls = "msg-other";
    }
    li.className = "msg " + cls;
    var author = document.createElement("span");
    author.className = "msg-author";
    author.textContent = label;
    var content = document.createElement("span");
    content.className = "msg-content";
    content.textContent = msg.content;
    li.appendChild(author);
    li.appendChild(content);
    return li;
  }

  function renderTeamChat(me) {
    var PAGE = 50;

    // ---- Chat list + create affordances: mounted into the persistent
    // left nav's #nav-extra slot (below the view tabs), not a second
    // in-content sidebar. ---------------------------------------------
    var actions = document.createElement("div");
    actions.className = "side-actions";
    var dmBtn = document.createElement("button");
    dmBtn.type = "button";
    dmBtn.textContent = "Chat with agent";
    var groupBtn = document.createElement("button");
    groupBtn.type = "button";
    groupBtn.textContent = "New group";
    actions.appendChild(dmBtn);
    actions.appendChild(groupBtn);
    var listEl = document.createElement("ul");
    listEl.className = "chat-list";
    navExtra.appendChild(actions);
    navExtra.appendChild(listEl);

    // ---- Main pane: header, messages, composer ------------------------
    var pane = document.createElement("section");
    pane.className = "chat-pane";
    var header = document.createElement("div");
    header.className = "pane-header";
    var title = document.createElement("h2");
    title.className = "pane-title";
    title.textContent = "Select or start a chat";
    var headerBtns = document.createElement("div");
    headerBtns.className = "pane-header-btns";
    var membersBtn = document.createElement("button");
    membersBtn.type = "button";
    membersBtn.textContent = "Members";
    membersBtn.hidden = true;
    headerBtns.appendChild(membersBtn);
    header.appendChild(title);
    header.appendChild(headerBtns);

    var list = document.createElement("ul");
    list.className = "messages";
    var memberPanel = document.createElement("div");
    memberPanel.className = "member-panel";
    memberPanel.hidden = true;

    var status = document.createElement("p");
    status.className = "loading";

    var form = document.createElement("form");
    form.className = "send";
    form.hidden = true;
    var input = document.createElement("input");
    input.type = "text";
    input.autocomplete = "off";
    input.placeholder = "Message…";
    var micBtn = document.createElement("button");
    micBtn.type = "button";
    micBtn.className = "icon-btn";
    micBtn.title = "Dictate";
    micBtn.textContent = "🎤";
    attachSpeechToText(micBtn, input);
    var translateBtn = document.createElement("button");
    translateBtn.type = "button";
    translateBtn.className = "icon-btn";
    translateBtn.title = "Translate";
    translateBtn.textContent = "🌐";
    translateBtn.hidden = !currentCaps || !currentCaps.translate;
    if (!translateBtn.hidden) attachTranslate(translateBtn, input);
    var sendBtn = document.createElement("button");
    sendBtn.type = "submit";
    sendBtn.textContent = "Send";
    form.appendChild(input);
    form.appendChild(micBtn);
    form.appendChild(translateBtn);
    form.appendChild(sendBtn);

    pane.appendChild(header);
    pane.appendChild(list);
    pane.appendChild(memberPanel);
    pane.appendChild(status);
    pane.appendChild(form);

    var state = {
      chats: [],
      current: null, // {id, type, name}
      names: {}, // userId -> username for the current chat
      myRole: null, // caller's role in the current chat
      unread: Object.create(null), // chatId -> true
      ids: Object.create(null), // rendered message IDs in the open chat
      oldestId: null,
      hasMore: false,
      loadingOlder: false,
      pending: false,
      replyTimer: null,
      ws: null,
      reconnectTimer: null,
      closed: false,
    };

    function seen(id) {
      if (!id) return false;
      if (state.ids[id]) return true;
      state.ids[id] = true;
      return false;
    }

    // ---- Sidebar rendering --------------------------------------------
    function renderList() {
      listEl.innerHTML = "";
      if (!state.chats.length) {
        var empty = document.createElement("li");
        empty.className = "chat-item empty";
        empty.textContent = "No chats yet";
        listEl.appendChild(empty);
        return;
      }
      state.chats.forEach(function (c) {
        var li = document.createElement("li");
        li.className = "chat-item";
        if (state.current && c.id === state.current.id) li.className += " active";
        var name = document.createElement("span");
        name.className = "chat-item-name";
        name.textContent = chatLabel(c);
        li.appendChild(name);
        if (state.unread[c.id]) {
          var dot = document.createElement("span");
          dot.className = "unread-dot";
          dot.textContent = "●";
          li.appendChild(dot);
        }
        li.addEventListener("click", function () {
          selectChat({ id: c.id, type: c.type, name: c.name });
        });
        listEl.appendChild(li);
      });
    }

    function loadChats() {
      return jsonFetch("/api/chats").then(function (body) {
        state.chats = body.chats || [];
        renderList();
      });
    }

    // ---- Chat selection + message rendering ---------------------------
    function resetMessages() {
      list.innerHTML = "";
      state.ids = Object.create(null);
      state.oldestId = null;
      state.hasMore = false;
    }

    function appendMsg(msg) {
      if (seen(msg.id)) return;
      var atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 40;
      list.appendChild(teamMessageNode(msg, me, state.names));
      if (atBottom) list.scrollTop = list.scrollHeight;
    }

    function selectChat(summary) {
      state.current = summary;
      delete state.unread[summary.id];
      memberPanel.hidden = true;
      title.textContent = chatLabel(summary);
      membersBtn.hidden = summary.type !== "group";
      form.hidden = false;
      input.placeholder = summary.type === "group" ? "Message the group…" : "Message the agent…";
      resetMessages();
      renderList();
      status.className = "loading";
      status.textContent = "Loading…";

      // Resolve member names first (groups) so authorship renders correctly.
      var prep = summary.type === "group" ? loadMembers(summary.id) : Promise.resolve();
      prep
        .then(function () {
          return jsonFetch("/api/chats/" + encodeURIComponent(summary.id));
        })
        .then(function (chat) {
          status.textContent = "";
          state.current = { id: chat.id, type: chat.type, name: chat.name };
          chat.messages.forEach(function (m) {
            seen(m.id);
            list.appendChild(teamMessageNode(m, me, state.names));
          });
          state.oldestId = chat.messages.length ? chat.messages[0].id : null;
          state.hasMore = chat.hasMore;
          list.scrollTop = list.scrollHeight;
          setPending(false);
          input.focus();
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Failed to load chat: " + err.message;
        });
    }

    function loadOlder() {
      if (!state.current || state.loadingOlder || !state.hasMore) return;
      state.loadingOlder = true;
      var prevHeight = list.scrollHeight;
      var url = "/api/chats/" + encodeURIComponent(state.current.id) + "/messages?limit=" + PAGE +
        (state.oldestId ? "&before=" + encodeURIComponent(state.oldestId) : "");
      jsonFetch(url)
        .then(function (body) {
          var frag = document.createDocumentFragment();
          body.messages.forEach(function (m) {
            if (seen(m.id)) return;
            frag.appendChild(teamMessageNode(m, me, state.names));
          });
          list.insertBefore(frag, list.firstChild);
          if (body.messages.length) state.oldestId = body.messages[0].id;
          state.hasMore = body.hasMore;
          list.scrollTop += list.scrollHeight - prevHeight;
        })
        .catch(function () {
          /* best-effort scroll-back */
        })
        .then(function () {
          state.loadingOlder = false;
        });
    }

    list.addEventListener("scroll", function () {
      if (list.scrollTop < 40) loadOlder();
    });

    // ---- Composer -----------------------------------------------------
    function setPending(on) {
      state.pending = on;
      input.disabled = on;
      sendBtn.disabled = on;
      micBtn.disabled = on;
      translateBtn.disabled = on;
      if (state.replyTimer) {
        clearTimeout(state.replyTimer);
        state.replyTimer = null;
      }
      if (on) {
        // Safety net so the composer never stays stuck if a reply is lost.
        state.replyTimer = setTimeout(function () {
          if (state.pending) setPending(false);
        }, 120000);
      } else if (!state.closed && state.current) {
        input.focus();
      }
    }

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var content = input.value.trim();
      if (!content || !state.current || state.pending) return;
      var chatID = state.current.id;
      var isPrivate = state.current.type === "private";
      input.value = "";
      // Private DMs block the composer until the agent replies; group sends
      // return immediately (no agent turn until RMI-113).
      if (isPrivate) setPending(true);
      jsonFetch("/api/chats/" + encodeURIComponent(chatID) + "/messages", {
        method: "POST",
        headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        body: JSON.stringify({ content: content }),
      })
        .then(function (body) {
          // The user message arrives over the WS fan-out; record its ID so we
          // don't render it twice, and echo it now for immediate feedback.
          if (body && body.user) appendMsg(body.user);
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          appendMsg({ authorType: "agent", content: "(error: " + err.message + ")" });
          if (isPrivate) setPending(false);
        });
    });

    // ---- Member panel (groups) ----------------------------------------
    function loadMembers(chatID) {
      return jsonFetch("/api/chats/" + encodeURIComponent(chatID) + "/members").then(function (body) {
        state.names = Object.create(null);
        state.myRole = null;
        (body.members || []).forEach(function (m) {
          state.names[m.userId] = m.username;
          if (me && m.userId === me.user_id) state.myRole = m.role;
        });
        return body.members || [];
      });
    }

    function renderMemberPanel(members) {
      memberPanel.innerHTML = "";
      var isOwner = state.myRole === "owner";

      var ul = document.createElement("ul");
      ul.className = "member-list";
      members.forEach(function (m) {
        var li = document.createElement("li");
        var nm = document.createElement("span");
        nm.textContent = m.username + (me && m.userId === me.user_id ? " (you)" : "");
        var role = document.createElement("span");
        role.className = "member-role";
        role.textContent = m.role;
        li.appendChild(nm);
        li.appendChild(role);
        // Owners can remove non-owner members other than themselves.
        if (isOwner && m.role !== "owner" && !(me && m.userId === me.user_id)) {
          var rm = document.createElement("button");
          rm.type = "button";
          rm.className = "member-remove";
          rm.textContent = "Remove";
          rm.addEventListener("click", function () {
            rm.disabled = true;
            jsonFetch("/api/chats/" + encodeURIComponent(state.current.id) + "/members/" + encodeURIComponent(m.userId), {
              method: "DELETE",
              headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
            })
              .then(refreshMembers)
              .catch(function (err) {
                if (err.message === "unauthenticated") return;
                rm.disabled = false;
                memberErr.textContent = "Remove failed: " + err.message;
              });
          });
          li.appendChild(rm);
        }
        ul.appendChild(li);
      });
      memberPanel.appendChild(ul);

      var memberErr = document.createElement("p");
      memberErr.className = "error member-err";

      if (isOwner) {
        var invite = document.createElement("form");
        invite.className = "invite";
        var iu = document.createElement("input");
        iu.type = "text";
        iu.autocomplete = "off";
        iu.placeholder = "username to invite";
        var ib = document.createElement("button");
        ib.type = "submit";
        ib.textContent = "Invite";
        invite.appendChild(iu);
        invite.appendChild(ib);
        invite.addEventListener("submit", function (ev) {
          ev.preventDefault();
          var username = iu.value.trim();
          if (!username) return;
          ib.disabled = true;
          jsonFetch("/api/chats/" + encodeURIComponent(state.current.id) + "/members", {
            method: "POST",
            headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
            body: JSON.stringify({ username: username }),
          })
            .then(function () {
              iu.value = "";
              ib.disabled = false;
              return refreshMembers();
            })
            .catch(function (err) {
              if (err.message === "unauthenticated") return;
              ib.disabled = false;
              memberErr.textContent = "Invite failed: " + err.message;
            });
        });
        memberPanel.appendChild(invite);
      }
      memberPanel.appendChild(memberErr);

      var leave = document.createElement("button");
      leave.type = "button";
      leave.className = "leave-btn";
      leave.textContent = "Leave group";
      leave.addEventListener("click", function () {
        leave.disabled = true;
        jsonFetch("/api/chats/" + encodeURIComponent(state.current.id) + "/leave", {
          method: "POST",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        })
          .then(function () {
            state.current = null;
            title.textContent = "Select or start a chat";
            membersBtn.hidden = true;
            memberPanel.hidden = true;
            form.hidden = true;
            resetMessages();
            return loadChats();
          })
          .catch(function (err) {
            if (err.message === "unauthenticated") return;
            leave.disabled = false;
            memberErr.textContent = "Leave failed: " + err.message;
          });
      });
      memberPanel.appendChild(leave);
    }

    function refreshMembers() {
      if (!state.current || state.current.type !== "group") return Promise.resolve();
      return loadMembers(state.current.id).then(function (members) {
        renderMemberPanel(members);
      });
    }

    membersBtn.addEventListener("click", function () {
      if (!state.current || state.current.type !== "group") return;
      if (memberPanel.hidden) {
        refreshMembers().then(function () {
          memberPanel.hidden = false;
        });
      } else {
        memberPanel.hidden = true;
      }
    });

    // ---- Create affordances -------------------------------------------
    dmBtn.addEventListener("click", function () {
      jsonFetch("/api/chats/dm")
        .then(function (chat) {
          return loadChats().then(function () {
            selectChat({ id: chat.id, type: chat.type, name: chat.name });
          });
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Could not open DM: " + err.message;
        });
    });

    groupBtn.addEventListener("click", function () {
      var name = window.prompt("Group name:");
      if (name === null) return;
      name = name.trim();
      if (!name) return;
      jsonFetch("/api/chats", {
        method: "POST",
        headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        body: JSON.stringify({ name: name }),
      })
        .then(function (chat) {
          return loadChats().then(function () {
            selectChat({ id: chat.id, type: chat.type, name: chat.name });
          });
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Could not create group: " + err.message;
        });
    });

    // ---- WebSocket: live messages across all the caller's chats --------
    function wsURL() {
      var proto = window.location.protocol === "https:" ? "wss:" : "ws:";
      return proto + "//" + window.location.host + "/ws";
    }

    function handleEvent(msg) {
      if (msg.type === "event" && msg.content === "chat.message" && msg.data) {
        var d = msg.data;
        if (state.current && d.chatId === state.current.id) {
          appendMsg({ id: d.id, authorType: d.authorType || "user", authorUserId: d.authorUserId, content: d.content });
          if (state.pending && d.authorType === "agent") setPending(false);
        } else if (d.chatId) {
          state.unread[d.chatId] = true;
          renderList();
        }
      } else if (msg.type === "event" && (msg.content === "chat.member.added" || msg.content === "chat.member.removed")) {
        if (state.current && msg.data && msg.data.chatId === state.current.id && !memberPanel.hidden) {
          refreshMembers();
        }
      } else if (msg.type === "error") {
        appendMsg({ authorType: "agent", content: "(error: " + (msg.error || "agent reply failed") + ")" });
        if (state.pending) setPending(false);
      }
    }

    function connectWS() {
      if (state.closed) return;
      var ws;
      try {
        ws = new WebSocket(wsURL());
      } catch (e) {
        scheduleReconnect();
        return;
      }
      state.ws = ws;
      ws.addEventListener("message", function (ev) {
        var msg;
        try {
          msg = JSON.parse(ev.data);
        } catch (e) {
          return;
        }
        handleEvent(msg);
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
        connectWS();
        // Resync the open chat in case a message landed while disconnected.
        if (state.current) {
          var cur = state.current;
          selectChat({ id: cur.id, type: cur.type, name: cur.name });
        }
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

    status.textContent = "Loading chats…";
    loadChats()
      .then(function () {
        status.textContent = state.chats.length ? "" : "Start a DM with the agent or create a group.";
        connectWS();
        // A chat just started from the catalog (RMI-312) is auto-opened here.
        if (pendingOpenChat) {
          var target = pendingOpenChat;
          pendingOpenChat = null;
          selectChat(target);
        }
      })
      .catch(function (err) {
        if (err.message === "unauthenticated") return;
        status.className = "error";
        status.textContent = "Failed to load chats: " + err.message;
      });

    return pane;
  }

  // ---- Catalog: discover agents, start chats (RMI-312) ------------------

  function renderCatalog() {
    var section = el("section", { className: "catalog" });
    var status = el("p", { className: "loading", text: "Loading catalog…" });
    section.appendChild(status);

    // startChat hands the new chat to the chat view, which auto-opens it.
    function startDM(entry) {
      jsonFetch("/api/chats/agents/" + encodeURIComponent(entry.id) + "/dm", {
        method: "POST",
        headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
      })
        .then(function (chat) {
          pendingOpenChat = { id: chat.id, type: chat.type, name: chat.name };
          switchTeamView("chat");
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Could not start chat: " + err.message;
        });
    }

    function startGroup(entry) {
      var name = window.prompt("Group name:");
      if (name === null) return;
      name = name.trim();
      if (!name) return;
      jsonFetch("/api/chats/agents/" + encodeURIComponent(entry.id) + "/group", {
        method: "POST",
        headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        body: JSON.stringify({ name: name }),
      })
        .then(function (chat) {
          pendingOpenChat = { id: chat.id, type: chat.type, name: chat.name };
          switchTeamView("chat");
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Could not create group: " + err.message;
        });
    }

    function card(entry) {
      var head = el("div", { className: "agent-card-head" }, [
        el("span", { className: "agent-card-name", text: entry.name }),
        entry.featured ? el("span", { className: "badge featured", text: "Featured" }) : null,
      ]);
      var body = [head];
      if (entry.description) {
        body.push(el("p", { className: "agent-card-desc", text: entry.description }));
      }
      var actions = el("div", { className: "agent-card-actions" });
      if (entry.canStart) {
        actions.appendChild(el("button", { type: "button", text: "Chat", onclick: function () { startDM(entry); } }));
        actions.appendChild(el("button", { type: "button", text: "New group", onclick: function () { startGroup(entry); } }));
      } else {
        actions.appendChild(el("span", { className: "muted", text: "No access" }));
      }
      body.push(actions);
      return el("li", { className: "agent-card" }, body);
    }

    function sectionFor(title, entries) {
      var wrap = el("div", { className: "catalog-section" }, [el("h3", { text: title })]);
      if (!entries.length) {
        wrap.appendChild(el("p", { className: "muted", text: "None yet." }));
        return wrap;
      }
      var ul = el("ul", { className: "agent-cards" });
      entries.forEach(function (e) { ul.appendChild(card(e)); });
      wrap.appendChild(ul);
      return wrap;
    }

    jsonFetch("/api/catalog")
      .then(function (cat) {
        status.remove();
        var featured = cat.featured || [];
        var listed = cat.listed || [];
        if (!featured.length && !listed.length) {
          section.appendChild(el("p", { className: "muted", text: "No agents are listed yet. Create one under My Agents and set it to Listed." }));
          return;
        }
        section.appendChild(sectionFor("Featured", featured));
        section.appendChild(sectionFor("All agents", listed));
      })
      .catch(function (err) {
        if (err.message === "unauthenticated") return;
        status.className = "error";
        status.textContent = "Failed to load catalog: " + err.message;
      });

    return section;
  }

  // ---- My Agents: owner/maintainer configuration (RMI-311) --------------

  function renderAgents() {
    var section = el("section", { className: "agents-area" });
    var content = el("div", { className: "agents-content" });
    section.appendChild(content);

    function setError(node, err) {
      if (err.message === "unauthenticated") return;
      node.className = "error";
      node.textContent = err.message;
    }

    // ---- List view ----
    function showList() {
      content.innerHTML = "";
      var bar = el("div", { className: "agents-bar" }, [
        el("h2", { className: "view-title", text: "My Agents" }),
        el("button", { type: "button", text: "New agent", onclick: showCreate }),
      ]);
      content.appendChild(bar);
      var status = el("p", { className: "loading", text: "Loading…" });
      content.appendChild(status);

      jsonFetch("/api/agents")
        .then(function (body) {
          status.remove();
          var agents = body.agents || [];
          if (!agents.length) {
            content.appendChild(el("p", { className: "muted", text: "You do not own or maintain any agents yet." }));
            return;
          }
          var ul = el("ul", { className: "agent-cards" });
          agents.forEach(function (a) {
            ul.appendChild(el("li", { className: "agent-card link", onclick: function () { showDetail(a.id); } }, [
              el("div", { className: "agent-card-head" }, [
                el("span", { className: "agent-card-name", text: a.name }),
                el("span", { className: "badge " + a.visibility, text: a.visibility }),
                a.featured ? el("span", { className: "badge featured", text: "Featured" }) : null,
              ]),
              el("p", { className: "agent-card-desc muted", text: "@" + a.slug }),
            ]));
          });
          content.appendChild(ul);
        })
        .catch(function (err) { setError(status, err); });
    }

    // ---- Create view ----
    function showCreate() {
      content.innerHTML = "";
      content.appendChild(el("h2", { className: "view-title", text: "New agent" }));
      var status = el("p", { className: "error" });

      var slug = el("input", { type: "text", placeholder: "slug (3-32 chars, a-z 0-9 - _)", autocomplete: "off" });
      var name = el("input", { type: "text", placeholder: "Display name", autocomplete: "off" });
      var desc = el("textarea", { placeholder: "Short description (shown in the catalog)", rows: 2 });
      var persona = el("textarea", { placeholder: "Persona / system prompt", rows: 5 });
      var model = el("input", { type: "text", placeholder: "Model (optional — deployment default)", autocomplete: "off" });
      var provider = el("input", { type: "text", placeholder: "Provider (optional — deployment default)", autocomplete: "off" });

      var form = el("form", { className: "agent-form" }, [
        field("Slug", slug), field("Name", name), field("Description", desc),
        field("Persona", persona), field("Model", model), field("Provider", provider),
        el("div", { className: "form-actions" }, [
          el("button", { type: "submit", text: "Create" }),
          el("button", { type: "button", text: "Cancel", onclick: showList }),
        ]),
        status,
      ]);
      form.addEventListener("submit", function (ev) {
        ev.preventDefault();
        status.textContent = "";
        jsonFetch("/api/agents", {
          method: "POST",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({
            slug: slug.value.trim(), name: name.value.trim(), description: desc.value,
            persona: persona.value, model: model.value.trim(), provider: provider.value.trim(),
          }),
        })
          .then(function (a) { showDetail(a.id); })
          .catch(function (err) { setError(status, err); });
      });
      content.appendChild(form);
    }

    // ---- Detail / edit view ----
    function showDetail(id) {
      content.innerHTML = "";
      var status = el("p", { className: "loading", text: "Loading…" });
      content.appendChild(el("button", { type: "button", className: "back", text: "← Back", onclick: showList }));
      content.appendChild(status);

      jsonFetch("/api/agents/" + encodeURIComponent(id))
        .then(function (d) {
          status.remove();
          content.appendChild(el("h2", { className: "view-title", text: d.name }));
          content.appendChild(el("p", { className: "muted", text: "@" + d.slug }));
          if (d.caps.configure) {
            content.appendChild(configCard(d));
            content.appendChild(skillsCard(d));
            content.appendChild(secretsCard(d));
          }
          if (d.caps.manageRegistry) {
            content.appendChild(visibilityCard(d));
          }
          content.appendChild(maintainersCard(d));
          content.appendChild(dangerCard(d));
        })
        .catch(function (err) { setError(status, err); });
    }

    function configCard(d) {
      var status = el("p", { className: "error" });
      var name = el("input", { type: "text", value: d.name, autocomplete: "off" });
      var desc = el("textarea", { rows: 2 }); desc.value = d.description || "";
      var persona = el("textarea", { rows: 5 }); persona.value = d.persona || "";
      var model = el("input", { type: "text", value: d.model || "", autocomplete: "off" });
      var provider = el("input", { type: "text", value: d.provider || "", autocomplete: "off" });
      var form = el("form", { className: "agent-form" }, [
        field("Name", name), field("Description", desc), field("Persona", persona),
        field("Model", model), field("Provider", provider),
        el("div", { className: "form-actions" }, [el("button", { type: "submit", text: "Save changes" })]),
        status,
      ]);
      form.addEventListener("submit", function (ev) {
        ev.preventDefault();
        status.className = "error"; status.textContent = "";
        jsonFetch("/api/agents/" + encodeURIComponent(d.id), {
          method: "PATCH",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({
            name: name.value.trim(), description: desc.value, persona: persona.value,
            model: model.value.trim(), provider: provider.value.trim(),
          }),
        })
          .then(function () { status.className = "ok"; status.textContent = "Saved."; })
          .catch(function (err) { setError(status, err); });
      });
      return card("Configuration", form);
    }

    function skillsCard(d) {
      var status = el("p", { className: "error" });
      var available = d.availableSkills || [];
      var enabled = {};
      (d.enabledSkills || []).forEach(function (s) { enabled[s] = true; });
      var boxes = el("div", { className: "skill-list" });
      if (!available.length) {
        boxes.appendChild(el("p", { className: "muted", text: "No skills are available in this deployment." }));
      }
      var inputs = [];
      available.forEach(function (s) {
        var cb = el("input", { type: "checkbox", value: s });
        cb.checked = !!enabled[s];
        inputs.push(cb);
        boxes.appendChild(el("label", { className: "skill" }, [cb, " " + s]));
      });
      var save = el("button", { type: "button", text: "Save skills", onclick: function () {
        status.className = "error"; status.textContent = "";
        var chosen = inputs.filter(function (cb) { return cb.checked; }).map(function (cb) { return cb.value; });
        jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/skills", {
          method: "PUT",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({ skills: chosen }),
        })
          .then(function () { status.className = "ok"; status.textContent = "Saved."; })
          .catch(function (err) { setError(status, err); });
      } });
      return card("Skills", el("div", {}, [boxes, el("div", { className: "form-actions" }, [save]), status]));
    }

    function visibilityCard(d) {
      var status = el("p", { className: "error" });
      var sel = el("select", {}, [
        el("option", { value: "private", text: "Private — only editors can start chats" }),
        el("option", { value: "listed", text: "Listed — anyone can discover and start" }),
      ]);
      sel.value = d.visibility;
      var save = el("button", { type: "button", text: "Save visibility", onclick: function () {
        status.className = "error"; status.textContent = "";
        jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/visibility", {
          method: "PUT",
          headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({ visibility: sel.value }),
        })
          .then(function () { status.className = "ok"; status.textContent = "Saved."; })
          .catch(function (err) { setError(status, err); });
      } });
      return card("Visibility", el("div", {}, [sel, el("div", { className: "form-actions" }, [save]), status]));
    }

    // secretsCard lists the secrets the agent's enabled skills declare, with a
    // set/unset indicator and a write-only paste-to-set input (GitHub-style).
    // Values are never rendered — the API returns names + set-state only.
    function secretsCard(d) {
      var status = el("p", { className: "error" });
      var list = el("ul", { className: "member-list" });
      var body = el("div", {}, [list]);

      function reload() {
        jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/secrets")
          .then(function (r) { renderSecrets(r.secrets || []); })
          .catch(function (err) { setError(status, err); });
      }
      function renderSecrets(secrets) {
        list.innerHTML = "";
        if (!secrets.length) {
          list.appendChild(el("li", {}, [el("span", { className: "muted", text: "No skill declares a secret. Enable a skill that needs one." })]));
          return;
        }
        secrets.forEach(function (s) {
          var label = s.name + (s.required ? " *" : "");
          var input = el("input", { type: "password", placeholder: s.set ? "•••••• (set) — paste to replace" : "paste value to set", autocomplete: "off" });
          var setBtn = el("button", { type: "button", text: "Save", onclick: function () {
            var v = input.value;
            if (!v) return;
            status.className = "error"; status.textContent = "";
            jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/secrets", {
              method: "PUT", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
              body: JSON.stringify({ name: s.env, value: v }),
            }).then(function () { input.value = ""; reload(); }).catch(function (err) { setError(status, err); });
          } });
          var row = [
            el("span", { className: "secret-name", text: label }),
            el("span", { className: "badge " + (s.set ? "active" : "disabled"), text: s.set ? "set" : "unset" }),
            s.description ? el("span", { className: "muted", text: s.description }) : null,
            input,
            setBtn,
          ];
          if (s.set) {
            row.push(el("button", { type: "button", className: "member-remove", text: "Delete", onclick: function () {
              jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/secrets/" + encodeURIComponent(s.env), {
                method: "DELETE", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
              }).then(reload).catch(function (err) { setError(status, err); });
            } }));
          }
          list.appendChild(el("li", { className: "secret-row" }, row));
        });
      }

      body.appendChild(status);
      reload();
      return card("Secrets", body);
    }

    function maintainersCard(d) {
      var status = el("p", { className: "error" });
      var list = el("ul", { className: "member-list" });
      var body = el("div", {}, [list]);

      function reload() {
        jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/roles")
          .then(function (r) { renderRoles(r.roles || []); })
          .catch(function (err) { setError(status, err); });
      }
      function renderRoles(roles) {
        list.innerHTML = "";
        roles.forEach(function (m) {
          var isMe = currentMe && m.userId === currentMe.user_id;
          var li = el("li", {}, [
            el("span", { text: m.username + (isMe ? " (you)" : "") }),
            el("span", { className: "member-role", text: m.role }),
          ]);
          // Owners/superadmin may remove other non-owner holders.
          if (d.caps.manageMaintainers && m.role !== "owner" && !isMe) {
            li.appendChild(el("button", { type: "button", className: "member-remove", text: "Remove", onclick: function () {
              jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/maintainers/" + encodeURIComponent(m.userId), {
                method: "DELETE", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
              }).then(reload).catch(function (err) { setError(status, err); });
            } }));
          }
          list.appendChild(li);
        });
      }

      if (d.caps.manageMaintainers) {
        var uname = el("input", { type: "text", placeholder: "username to add", autocomplete: "off" });
        var invite = el("form", { className: "invite" }, [uname, el("button", { type: "submit", text: "Add maintainer" })]);
        invite.addEventListener("submit", function (ev) {
          ev.preventDefault();
          var u = uname.value.trim();
          if (!u) return;
          status.className = "error"; status.textContent = "";
          jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/maintainers", {
            method: "POST", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }), body: JSON.stringify({ username: u }),
          }).then(function () { uname.value = ""; reload(); }).catch(function (err) { setError(status, err); });
        });
        body.appendChild(invite);
      }
      body.appendChild(status);
      reload();
      return card("Maintainers", body);
    }

    function dangerCard(d) {
      var status = el("p", { className: "error" });
      var buttons = [];
      // Self-leave (any role holder).
      buttons.push(el("button", { type: "button", className: "leave-btn", text: "Leave this agent", onclick: function () {
        if (!window.confirm("Leave this agent? You will lose your role on it.")) return;
        jsonFetch("/api/agents/" + encodeURIComponent(d.id) + "/leave", {
          method: "POST", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        }).then(showList).catch(function (err) { setError(status, err); });
      } }));
      // Delete (owner/superadmin — mirrors ManageMaintainers gate).
      if (d.caps.manageMaintainers) {
        buttons.push(el("button", { type: "button", className: "member-remove", text: "Delete agent", onclick: function () {
          if (!window.confirm("Delete this agent permanently? Its skills and roles are removed.")) return;
          jsonFetch("/api/agents/" + encodeURIComponent(d.id), {
            method: "DELETE", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          }).then(showList).catch(function (err) { setError(status, err); });
        } }));
      }
      return card("Danger zone", el("div", {}, [el("div", { className: "form-actions" }, buttons), status]));
    }

    showList();
    return section;
  }

  // field wraps a labeled control for the agent forms.
  function field(label, control) {
    return el("label", { className: "form-field" }, [el("span", { className: "form-label", text: label }), control]);
  }

  // card is a titled panel used across the agent detail view.
  function card(title, body) {
    return el("section", { className: "panel" }, [el("h3", { className: "panel-title", text: title }), body]);
  }

  // ---- Superadmin curation: featured agents (RMI-313) -------------------

  function renderCuration() {
    var section = el("section", { className: "curation" });
    section.appendChild(el("p", { className: "muted", text: "Feature listed agents to promote them to the top of everyone's catalog." }));
    var status = el("p", { className: "loading", text: "Loading…" });
    section.appendChild(status);
    var container = el("div");
    section.appendChild(container);

    function toggle(entry) {
      jsonFetch("/api/agents/" + encodeURIComponent(entry.id) + "/featured", {
        method: "PUT",
        headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
        body: JSON.stringify({ featured: !entry.featured }),
      })
        .then(load)
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Could not update: " + err.message;
        });
    }

    function group(title, entries) {
      container.appendChild(el("h3", { text: title }));
      if (!entries.length) {
        container.appendChild(el("p", { className: "muted", text: "None." }));
        return;
      }
      var ul = el("ul", { className: "agent-cards" });
      entries.forEach(function (e) {
        ul.appendChild(el("li", { className: "agent-card" }, [
          el("div", { className: "agent-card-head" }, [
            el("span", { className: "agent-card-name", text: e.name }),
            e.featured ? el("span", { className: "badge featured", text: "Featured" }) : null,
          ]),
          el("div", { className: "agent-card-actions" }, [
            el("button", { type: "button", text: e.featured ? "Unfeature" : "Feature", onclick: function () { toggle(e); } }),
          ]),
        ]));
      });
      container.appendChild(ul);
    }

    function load() {
      container.innerHTML = "";
      jsonFetch("/api/catalog")
        .then(function (cat) {
          status.remove();
          group("Featured", cat.featured || []);
          group("Listed", cat.listed || []);
        })
        .catch(function (err) {
          if (err.message === "unauthenticated") return;
          status.className = "error";
          status.textContent = "Failed to load: " + err.message;
        });
    }

    load();
    return section;
  }

  // ---- Superadmin admin: allowlist + members (RMI-119) ------------------

  function renderAdmin() {
    function setErr(node, err) {
      if (err.message === "unauthenticated") return;
      node.className = "error";
      node.textContent = err.message;
    }

    function allowlistCard() {
      var status = el("p", { className: "error" });
      var list = el("ul", { className: "member-list" });
      var email = el("input", { type: "email", placeholder: "email to allow", autocomplete: "off" });
      var note = el("input", { type: "text", placeholder: "note (optional)", autocomplete: "off" });
      var addBtn = el("button", { type: "submit", text: "Add" });
      var invite = el("form", { className: "invite" }, [email, note, addBtn]);

      function reload() {
        jsonFetch("/api/admin/allowlist")
          .then(function (r) { renderList(r.allowlist || []); })
          .catch(function (err) { setErr(status, err); });
      }
      function renderList(entries) {
        list.innerHTML = "";
        entries.forEach(function (e) {
          list.appendChild(el("li", {}, [
            el("span", { text: e.email }),
            e.note ? el("span", { className: "member-role", text: e.note }) : null,
            el("button", { type: "button", className: "member-remove", text: "Remove", onclick: function () {
              jsonFetch("/api/admin/allowlist?email=" + encodeURIComponent(e.email), {
                method: "DELETE", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
              }).then(reload).catch(function (err) { setErr(status, err); });
            } }),
          ]));
        });
      }
      invite.addEventListener("submit", function (ev) {
        ev.preventDefault();
        var e = email.value.trim();
        if (!e) return;
        status.className = "error"; status.textContent = "";
        jsonFetch("/api/admin/allowlist", {
          method: "POST", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
          body: JSON.stringify({ email: e, note: note.value.trim() }),
        }).then(function () { email.value = ""; note.value = ""; reload(); }).catch(function (err) { setErr(status, err); });
      });

      reload();
      return card("Allowlist", el("div", {}, [list, invite, status]));
    }

    function membersCard() {
      var status = el("p", { className: "error" });
      var list = el("ul", { className: "member-list" });

      function reload() {
        jsonFetch("/api/admin/users")
          .then(function (r) { renderList(r.users || []); })
          .catch(function (err) { setErr(status, err); });
      }
      function renderList(users) {
        list.innerHTML = "";
        users.forEach(function (u) {
          var isMe = currentMe && u.id === currentMe.user_id;
          var row = [
            el("span", { text: u.displayName || u.username }),
            el("span", { className: "badge", text: u.role }),
            el("span", { className: "badge " + u.status, text: u.status }),
          ];
          (u.identities || []).forEach(function (provider) {
            row.push(el("span", { className: "badge identity", text: provider }));
          });
          if (!isMe) {
            var willDisable = u.status === "active";
            row.push(el("button", { type: "button", className: "member-remove", text: willDisable ? "Disable" : "Enable", onclick: function () {
              jsonFetch("/api/admin/users/" + encodeURIComponent(u.id), {
                method: "PATCH", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
                body: JSON.stringify({ status: willDisable ? "disabled" : "active" }),
              }).then(reload).catch(function (err) { setErr(status, err); });
            } }));
          }
          // Superadmin can set/reset any member's password (write-only).
          var pw = el("input", { type: "password", placeholder: "set password", autocomplete: "off" });
          row.push(pw);
          row.push(el("button", { type: "button", text: "Set", onclick: function () {
            if (!pw.value) return;
            status.className = "error"; status.textContent = "";
            jsonFetch("/api/admin/users/" + encodeURIComponent(u.id), {
              method: "PATCH", headers: csrfHeaders({ "X-OmniAgent-CSRF": "1" }),
              body: JSON.stringify({ password: pw.value }),
            }).then(function () { pw.value = ""; status.className = "ok"; status.textContent = "Password set for " + (u.displayName || u.username) + "."; }).catch(function (err) { setErr(status, err); });
          } }));
          list.appendChild(el("li", { className: "secret-row" }, row));
        });
      }

      reload();
      return card("Members", el("div", {}, [list, status]));
    }

    // globalSecretBindingsCard is read-only: the single-operator config
    // bindings (secrets:, skills.config.<name>.secrets) are only ever
    // changed by editing the config file, never through the web UI. Shows
    // names + set-state, never values (RMI-OMNIAGENT-213).
    function globalSecretBindingsCard() {
      var status = el("p", { className: "error" });
      var list = el("ul", { className: "member-list" });

      jsonFetch("/api/admin/secret-bindings")
        .then(function (r) { renderList(r.bindings || []); })
        .catch(function (err) { setErr(status, err); });

      function renderList(bindings) {
        list.innerHTML = "";
        if (!bindings.length) {
          list.appendChild(el("li", { className: "muted", text: "No global secret bindings configured." }));
          return;
        }
        bindings.forEach(function (b) {
          list.appendChild(el("li", { className: "secret-row" }, [
            el("span", { className: "secret-name", text: b.name }),
            el("span", { className: "badge " + (b.set ? "active" : "disabled"), text: b.set ? "set" : "unset" }),
            el("span", { className: "muted", text: b.source === "global" ? "global" : "skill: " + b.source }),
          ]));
        });
      }

      return card("Global Secret Bindings", el("div", {}, [list, status]));
    }

    var section = el("section", { className: "admin-area" });
    section.appendChild(el("h2", { className: "view-title", text: "Admin" }));
    section.appendChild(allowlistCard());
    section.appendChild(membersCard());
    section.appendChild(globalSecretBindingsCard());
    return section;
  }

  function render() {
    if (chatTeardown) {
      chatTeardown();
      chatTeardown = null;
    }
    app.innerHTML = "";
    navExtra.innerHTML = "";
    fetchCapabilities()
      .then(function (caps) {
        var showLogin = caps.authRequired;
        return (showLogin ? fetchMe() : Promise.resolve(null)).then(function (me) {
          currentCaps = caps;
          currentMe = me;
          if (!me) teamView = "chat"; // reset the active surface across logout
          renderNav(caps, me);
          if (showLogin && !me) {
            app.appendChild(renderLogin(caps));
            return;
          }
          if (!caps.multiUser) {
            app.appendChild(renderChat());
            return;
          }
          // Team mode: a small view router over the chat, catalog, agents, and
          // curation surfaces (rendered only when multiUser, per the capability
          // gate). mountTeamView owns #app from here.
          mountTeamView();
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
