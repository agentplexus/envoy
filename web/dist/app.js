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
    var wrap = document.createElement("section");
    wrap.className = "team";

    // ---- Sidebar: chat list + create affordances ----------------------
    var side = document.createElement("aside");
    side.className = "sidebar";
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
    side.appendChild(actions);
    side.appendChild(listEl);

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
    var sendBtn = document.createElement("button");
    sendBtn.type = "submit";
    sendBtn.textContent = "Send";
    form.appendChild(input);
    form.appendChild(sendBtn);

    pane.appendChild(header);
    pane.appendChild(list);
    pane.appendChild(memberPanel);
    pane.appendChild(status);
    pane.appendChild(form);

    wrap.appendChild(side);
    wrap.appendChild(pane);

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
      })
      .catch(function (err) {
        if (err.message === "unauthenticated") return;
        status.className = "error";
        status.textContent = "Failed to load chats: " + err.message;
      });

    return wrap;
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
          // Team mode: private DMs + group chats (rendered only when
          // multiUser, per the capability gate).
          app.appendChild(renderTeamChat(me));
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
