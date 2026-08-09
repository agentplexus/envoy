// OmniAgent web UI shell. One capability-driven SPA for both personal and
// team deployments (TRD §1a/§6) — no build step, no external assets.
(function () {
  "use strict";

  var app = document.getElementById("app");
  var nav = document.getElementById("nav");

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

  function appendMessage(list, msg) {
    var li = document.createElement("li");
    li.className = "msg msg-" + msg.authorType;
    var author = document.createElement("span");
    author.className = "msg-author";
    author.textContent = msg.authorType === "agent" ? "Agent" : "You";
    var content = document.createElement("span");
    content.className = "msg-content";
    content.textContent = msg.content;
    li.appendChild(author);
    li.appendChild(content);
    list.appendChild(li);
    list.scrollTop = list.scrollHeight;
  }

  // renderChat builds the personal-mode chat panel: history + send form,
  // wired to GET/POST /api/chat (personal mode only — team mode's chat
  // surface is a separate, not-yet-built endpoint set behind auth).
  function renderChat() {
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

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var content = input.value.trim();
      if (!content) return;
      input.value = "";
      input.disabled = true;
      button.disabled = true;
      appendMessage(list, { authorType: "user", content: content });
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
          if (body.agent) appendMessage(list, body.agent);
        })
        .catch(function (err) {
          appendMessage(list, { authorType: "agent", content: "(error: " + err.message + ")" });
        })
        .then(function () {
          input.disabled = false;
          button.disabled = false;
          input.focus();
        });
    });

    section.appendChild(list);
    section.appendChild(status);
    section.appendChild(form);

    fetchChat()
      .then(function (chat) {
        status.remove();
        chat.messages.forEach(function (m) {
          appendMessage(list, m);
        });
        input.focus();
      })
      .catch(function (err) {
        status.className = "error";
        status.textContent = "Failed to load chat: " + err.message;
      });

    return section;
  }

  function render() {
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
