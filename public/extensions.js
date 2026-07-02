const forumState = {
  category: "daily",
  sort: "latest_reply",
  search: "",
  page: 1,
  pageSize: 10,
  token: localStorage.getItem("forum_access_token") || "",
  refreshToken: localStorage.getItem("forum_refresh_token") || "",
  currentUser: null,
  selectedThreadID: "",
};

const $ = (selector) => document.querySelector(selector);

document.addEventListener("DOMContentLoaded", () => {
  bindForumEvents();
  refreshAuthState();
  loadCategories();
  loadThreads();
});

function bindForumEvents() {
  $("#search-form").addEventListener("submit", (event) => {
    event.preventDefault();
    forumState.search = $("#search-input").value.trim();
    forumState.sort = $("#sort-select").value;
    forumState.page = 1;
    loadThreads();
  });

  $("#prev-page").addEventListener("click", () => {
    if (forumState.page <= 1) return;
    forumState.page -= 1;
    loadThreads();
  });

  $("#next-page").addEventListener("click", () => {
    forumState.page += 1;
    loadThreads();
  });

  $("#thread-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const payload = Object.fromEntries(new FormData(form).entries());
    payload.category_slug = forumState.category;
    const thread = await apiFetch("/api/forum/threads", { method: "POST", body: payload });
    form.reset();
    showToast("Thread posted");
    forumState.page = 1;
    await loadThreads();
    renderThreadDetail(thread);
  });

  $("#login-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget).entries());
    const auth = await apiFetch("/api/auth/login", { method: "POST", body: payload, auth: false });
    storeAuth(auth);
    event.currentTarget.reset();
    showToast("Logged in");
    refreshAuthState();
  });

  $("#register-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget).entries());
    const auth = await apiFetch("/api/auth/register", { method: "POST", body: payload, auth: false });
    storeAuth(auth);
    event.currentTarget.reset();
    showToast("Account created");
    refreshAuthState();
  });

  $("#logout-button").addEventListener("click", () => {
    forumState.token = "";
    forumState.refreshToken = "";
    forumState.currentUser = null;
    localStorage.removeItem("forum_access_token");
    localStorage.removeItem("forum_refresh_token");
    renderAuthState();
    showToast("Logged out");
  });

  document.querySelector('[data-action="refresh"]').addEventListener("click", () => {
    loadThreads();
    if (forumState.selectedThreadID) {
      loadThread(forumState.selectedThreadID);
    }
  });
}

async function loadCategories() {
  try {
    const data = await apiFetch("/api/forum/categories", { auth: false });
    const list = $("#category-list");
    list.innerHTML = data.items.map((category) => `
      <button class="category-button" type="button" data-category="${escapeAttr(category.slug)}" aria-current="${category.slug === forumState.category}">
        <span>${escapeHTML(category.name)}</span>
        <small>${escapeHTML(category.slug)}</small>
      </button>
    `).join("");
    list.querySelectorAll("[data-category]").forEach((button) => {
      button.addEventListener("click", () => {
        forumState.category = button.dataset.category;
        forumState.page = 1;
        $("#board-title").textContent = button.textContent.trim().split(/\s+/)[0] || "Forum";
        loadCategories();
        loadThreads();
      });
    });
  } catch (error) {
    showToast(error.message, true);
  }
}

async function loadThreads() {
  const params = new URLSearchParams({
    page: forumState.page,
    page_size: forumState.pageSize,
    sort: forumState.sort,
  });
  if (forumState.search) params.set("q", forumState.search);

  try {
    const data = await apiFetch(`/api/forum/categories/${encodeURIComponent(forumState.category)}/threads?${params}`, { auth: false });
    renderThreads(data);
  } catch (error) {
    showToast(error.message, true);
  }
}

function renderThreads(data) {
  const list = $("#thread-list");
  if (!data.items.length) {
    list.innerHTML = `<div class="empty-state"><h2>No threads yet</h2><p>Start the first Daily discussion.</p></div>`;
  } else {
    list.innerHTML = data.items.map((thread) => `
      <button class="thread-row" type="button" data-thread-id="${escapeAttr(thread.id)}" aria-current="${thread.id === forumState.selectedThreadID}">
        <h2>${escapeHTML(thread.title)}</h2>
        <div class="thread-meta">
          <span>${escapeHTML(thread.author.name || "Unknown")}</span>
          <span>${formatTime(thread.created_at)}</span>
          <span>${thread.reply_count} replies</span>
          <span>${thread.view_count} views</span>
        </div>
        <p class="thread-excerpt">${escapeHTML(thread.body_excerpt)}</p>
      </button>
    `).join("");
    list.querySelectorAll("[data-thread-id]").forEach((button) => {
      button.addEventListener("click", () => loadThread(button.dataset.threadId));
    });
  }

  const page = data.pagination || {};
  $("#page-summary").textContent = `Page ${page.page || forumState.page}`;
  $("#prev-page").disabled = !page.has_previous;
  $("#next-page").disabled = !page.has_next;
}

async function loadThread(threadID) {
  try {
    const thread = await apiFetch(`/api/forum/threads/${encodeURIComponent(threadID)}`, { auth: false });
    renderThreadDetail(thread);
    loadThreads();
  } catch (error) {
    showToast(error.message, true);
  }
}

function renderThreadDetail(thread) {
  forumState.selectedThreadID = thread.id;
  const canReply = Boolean(forumState.currentUser);
  $("#thread-detail").innerHTML = `
    <section class="thread-detail-card">
      <p class="eyebrow">${escapeHTML(thread.category.name || thread.category.slug)}</p>
      <h2>${escapeHTML(thread.title)}</h2>
      <div class="detail-meta">
        ${escapeHTML(thread.author.name || "Unknown")} · ${formatTime(thread.created_at)} · ${thread.reply_count} replies · ${thread.view_count} views
      </div>
      <p class="thread-body">${escapeHTML(thread.body)}</p>
      <section class="reply-list">
        <h3>Replies</h3>
        ${thread.posts.length ? thread.posts.map(renderReply).join("") : `<div class="notice-row">No replies yet.</div>`}
      </section>
      ${canReply ? renderReplyForm(thread.id) : `<div class="notice-row">Log in to reply.</div>`}
    </section>
  `;
  const form = $("#reply-form");
  if (form) {
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      const payload = Object.fromEntries(new FormData(form).entries());
      const updated = await apiFetch(`/api/forum/threads/${encodeURIComponent(thread.id)}/posts`, { method: "POST", body: payload });
      form.reset();
      showToast("Reply posted");
      renderThreadDetail(updated);
      loadThreads();
    });
  }
}

function renderReply(reply) {
  return `
    <article class="reply-row">
      <header>
        <strong>${escapeHTML(reply.author.name || "Unknown")}</strong>
        <span class="reply-meta">${formatTime(reply.created_at)}</span>
      </header>
      <p class="reply-body">${escapeHTML(reply.body)}</p>
    </article>
  `;
}

function renderReplyForm(threadID) {
  return `
    <form id="reply-form" class="reply-form" data-thread-id="${escapeAttr(threadID)}">
      <textarea name="body" rows="3" placeholder="Add a useful reply" required></textarea>
      <button type="submit">Reply</button>
    </form>
  `;
}

async function refreshAuthState() {
  if (!forumState.token) {
    renderAuthState();
    return;
  }

  try {
    const status = await apiFetch("/api/auth/status");
    forumState.currentUser = status.logged_in ? status.user : null;
  } catch {
    forumState.currentUser = null;
  }
  renderAuthState();
}

function renderAuthState() {
  const loggedIn = Boolean(forumState.currentUser);
  $("#logged-out-panel").hidden = loggedIn;
  $("#logged-in-panel").hidden = !loggedIn;
  $("#composer-panel").hidden = !loggedIn;
  $("#auth-summary").textContent = loggedIn ? forumState.currentUser.name : "Guest";
  $("#current-user-name").textContent = loggedIn ? `${forumState.currentUser.name} is signed in` : "";
}

async function apiFetch(url, options = {}) {
  const headers = { Accept: "application/json" };
  const fetchOptions = { method: options.method || "GET", headers };
  if (options.auth !== false && forumState.token) {
    headers.Authorization = `Bearer ${forumState.token}`;
  }
  if (options.body) {
    headers["Content-Type"] = "application/json";
    fetchOptions.body = JSON.stringify(options.body);
  }

  const response = await fetch(url, fetchOptions);
  const envelope = await response.json().catch(() => null);
  if (!response.ok || !envelope || envelope.success === false) {
    const message = envelope?.error?.message || `Request failed with ${response.status}`;
    throw new Error(message);
  }
  return envelope.data;
}

function storeAuth(auth) {
  forumState.token = auth.access_token;
  forumState.refreshToken = auth.refresh_token;
  forumState.currentUser = auth.user;
  localStorage.setItem("forum_access_token", forumState.token);
  localStorage.setItem("forum_refresh_token", forumState.refreshToken);
}

function showToast(message, error = false) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.toggle("is-error", error);
  toast.hidden = false;
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => {
    toast.hidden = true;
  }, 3200);
}

function formatTime(value) {
  if (!value) return "";
  const normalized = value.includes("T") ? value : value.replace(" ", "T") + "Z";
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#039;",
  }[char]));
}

function escapeAttr(value) {
  return escapeHTML(value);
}
