const boardDefinitions = [
  { slug: "daily", label: "日常", icon: "U" },
  { slug: "tech", label: "技术", icon: "Σ" },
  { slug: "info", label: "情报", icon: "Q" },
  { slug: "review", label: "测评", icon: "☃" },
  { slug: "trade", label: "交易", icon: "$" },
  { slug: "carpool", label: "拼车", icon: "▣" },
  { slug: "promotion", label: "推广", icon: "≋" },
  { slug: "life", label: "生活", icon: "♡" },
  { slug: "dev", label: "Dev", icon: ">" },
  { slug: "image", label: "贴图", icon: "◎" },
  { slug: "exposure", label: "曝光", icon: "¤" },
  { slug: "sandbox", label: "沙盒", icon: "△" },
];

const initialPostRoute = postRouteFromPath();

const forumState = {
  category: categoryFromPath(),
  categories: [],
  sort: "latest_reply",
  search: "",
  page: 1,
  pageSize: 20,
  token: localStorage.getItem("forum_access_token") || "",
  refreshToken: localStorage.getItem("forum_refresh_token") || "",
  currentUser: null,
  selectedThreadID: initialPostRoute?.threadID || "",
  selectedPostPage: initialPostRoute?.page || 1,
  viewMode: initialPostRoute ? "detail" : "list",
};

const $ = (selector) => document.querySelector(selector);

document.addEventListener("DOMContentLoaded", () => {
  bindForumEvents();
  renderStaticBoardLinks();
  setFeedMode(initialPostRoute ? "detail" : "list");
  refreshAuthState();
  loadCategories();
});

function bindForumEvents() {
  $("#search-form").addEventListener("submit", (event) => {
    event.preventDefault();
    forumState.search = $("#search-input").value.trim();
    forumState.page = 1;
    forumState.selectedThreadID = "";
    forumState.selectedPostPage = 1;
    setFeedMode("list");
    resetThreadDetail();
    window.history.pushState({}, "", forumState.category ? `/categories/${encodeURIComponent(forumState.category)}` : "/");
    loadThreads();
  });

  $("#sort-select").addEventListener("change", (event) => {
    forumState.sort = event.currentTarget.value;
    forumState.page = 1;
    syncSortTabs();
    loadThreads();
  });

  document.querySelectorAll("[data-sort-tab]").forEach((button) => {
    button.addEventListener("click", () => {
      forumState.sort = button.dataset.sortTab;
      $("#sort-select").value = forumState.sort;
      forumState.page = 1;
      syncSortTabs();
      loadThreads();
    });
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
    payload.category_slug = forumState.category || "daily";
    const thread = await apiFetch("/api/forum/threads", { method: "POST", body: payload });
    form.reset();
    showToast("主题已发布");
    forumState.page = 1;
    window.history.pushState({}, "", postPath(thread.id, 1));
    renderThreadDetail(thread);
  });

  $("#login-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget).entries());
    const auth = await apiFetch("/api/auth/login", { method: "POST", body: payload, auth: false });
    storeAuth(auth);
    event.currentTarget.reset();
    showToast("已登录");
    refreshAuthState();
  });

  $("#register-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = Object.fromEntries(new FormData(event.currentTarget).entries());
    const auth = await apiFetch("/api/auth/register", { method: "POST", body: payload, auth: false });
    storeAuth(auth);
    event.currentTarget.reset();
    showToast("账号已创建");
    refreshAuthState();
  });

  $("#logout-button").addEventListener("click", () => {
    forumState.token = "";
    forumState.refreshToken = "";
    forumState.currentUser = null;
    localStorage.removeItem("forum_access_token");
    localStorage.removeItem("forum_refresh_token");
    renderAuthState();
    showToast("已退出登录");
  });

  window.addEventListener("popstate", () => {
    const postRoute = postRouteFromPath();
    forumState.category = categoryFromPath();
    forumState.page = 1;
    forumState.selectedThreadID = postRoute?.threadID || "";
    forumState.selectedPostPage = postRoute?.page || 1;
    renderCategories();
    updateBoardHeading();
    if (postRoute) {
      loadThread(postRoute.threadID, {
        postPage: postRoute.page,
        updatePath: false,
        refreshList: false,
      });
      return;
    }
    setFeedMode("list");
    resetThreadDetail();
    loadThreads();
  });
}

function renderStaticBoardLinks() {
  $("#top-board-links").innerHTML = boardDefinitions.slice(0, 8).map((board) => `
    <button type="button" data-top-board="${escapeAttr(board.slug)}">${escapeHTML(board.label)}</button>
  `).join("");
  document.querySelectorAll("[data-top-board]").forEach((button) => {
    button.addEventListener("click", () => selectCategory(button.dataset.topBoard));
  });
}

async function loadCategories() {
  try {
    const data = await apiFetch("/api/forum/categories", { auth: false });
    forumState.categories = mergeBoardDefinitions(data.items || []);
    if (forumState.category && !forumState.categories.some((category) => category.slug === forumState.category)) {
      forumState.category = "";
      window.history.replaceState({}, "", "/");
    }
    renderCategories();
    updateBoardHeading();
    const postRoute = postRouteFromPath();
    if (postRoute) {
      loadThread(postRoute.threadID, {
        postPage: postRoute.page,
        updatePath: false,
      });
      return;
    }
    loadThreads();
  } catch (error) {
    forumState.categories = mergeBoardDefinitions([]);
    renderCategories();
    updateBoardHeading();
    renderEmptyThreads("板块加载失败", error.message);
    showToast(error.message, true);
  }
}

function mergeBoardDefinitions(apiCategories) {
  const bySlug = new Map(apiCategories.map((category) => [category.slug, category]));
  return boardDefinitions.map((board) => {
    const apiCategory = bySlug.get(board.slug);
    return {
      ...board,
      ...apiCategory,
      label: apiCategory?.name || board.label,
      enabled: apiCategory ? apiCategory.enabled !== false : false,
    };
  });
}

function renderCategories() {
  $("#category-list").innerHTML = forumState.categories.map((category) => `
    <button class="category-button" type="button" data-category="${escapeAttr(category.slug)}" aria-current="${category.slug === forumState.category}">
      <span class="category-icon">${escapeHTML(category.icon)}</span>
      <span>${escapeHTML(category.label)}</span>
    </button>
  `).join("");

  document.querySelectorAll("[data-category]").forEach((button) => {
    button.addEventListener("click", () => selectCategory(button.dataset.category));
  });

  document.querySelectorAll("[data-top-board]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.topBoard === forumState.category);
  });
}

function selectCategory(slug, options = {}) {
  if (forumState.category === slug && !forumState.selectedThreadID) return;
  forumState.category = slug;
  forumState.page = 1;
  forumState.selectedThreadID = "";
  forumState.selectedPostPage = 1;
  if (options.updatePath !== false) {
    window.history.pushState({}, "", `/categories/${encodeURIComponent(slug)}`);
  }
  renderCategories();
  updateBoardHeading();
  setFeedMode("list");
  resetThreadDetail();
  loadThreads();
}

async function loadThreads() {
  const params = new URLSearchParams({
    page: forumState.page,
    page_size: forumState.pageSize,
    sort: forumState.sort,
  });
  if (forumState.search) params.set("q", forumState.search);

  try {
    const endpoint = forumState.category
      ? `/api/forum/categories/${encodeURIComponent(forumState.category)}/threads?${params}`
      : `/api/forum/threads?${params}`;
    const data = await apiFetch(endpoint, { auth: false });
    renderThreads(data);
  } catch (error) {
    renderEmptyThreads("暂时没有内容", "这个板块还没有可展示的主题，或者数据正在迁移中。");
    showToast(error.message, true);
  }
}

function renderThreads(data) {
  if (forumState.viewMode !== "list") return;
  const items = data.items || [];
  if (!items.length) {
    renderEmptyThreads("还没有主题", "成为这个板块第一个发起讨论的人。");
  } else {
    $("#thread-list").innerHTML = items.map(renderThreadRow).join("");
    document.querySelectorAll("[data-thread-id]").forEach((button) => {
      button.addEventListener("click", () => loadThread(button.dataset.threadId, { postPage: 1 }));
    });
  }

  const page = data.pagination || {};
  $("#page-summary").textContent = buildPageSummary(page);
  $("#prev-page").disabled = !page.has_previous;
  $("#next-page").disabled = !page.has_next;
}

function renderThreadRow(thread) {
  const category = thread.category || currentCategory();
  const lastPoster = thread.last_post_author?.name || thread.author?.name || "Unknown";
  const active = thread.id === forumState.selectedThreadID;
  return `
    <button class="thread-row" type="button" data-thread-id="${escapeAttr(thread.id)}" aria-current="${active}">
      <span class="avatar" style="--avatar-hue: ${avatarHue(thread.author?.name || thread.title)}">${avatarInitial(thread.author?.name || thread.title)}</span>
      <span class="thread-content">
        <span class="thread-title-line">
          ${thread.is_pinned ? `<span class="status-badge">置顶</span>` : ""}
          ${thread.is_locked ? `<span class="status-badge danger">只读</span>` : ""}
          <strong>${escapeHTML(thread.title)}</strong>
        </span>
        <span class="thread-meta">
          <span>♙ ${escapeHTML(thread.author?.name || "Unknown")}</span>
          <span>◉ ${formatNumber(thread.view_count)}</span>
          <span>▱ ${formatNumber(thread.reply_count)}</span>
          <span>↯ ${escapeHTML(lastPoster)}</span>
          <span>${formatRelativeTime(thread.last_post_at || thread.created_at)}</span>
        </span>
      </span>
      <span class="thread-category">${escapeHTML(category.name || category.label || category.slug)}</span>
    </button>
  `;
}

function renderEmptyThreads(title, message) {
  if (forumState.viewMode !== "list") return;
  setFeedMode("list");
  $("#thread-list").innerHTML = `
    <article class="empty-state">
      <h2>${escapeHTML(title)}</h2>
      <p>${escapeHTML(message)}</p>
    </article>
  `;
  $("#page-summary").textContent = String(forumState.page);
  $("#prev-page").disabled = forumState.page <= 1;
  $("#next-page").disabled = true;
}

async function loadThread(threadID, options = {}) {
  const postPage = normalizePostPage(options.postPage);
  forumState.selectedThreadID = threadID;
  forumState.selectedPostPage = postPage;
  setFeedMode("detail");
  renderThreadLoading();
  if (options.updatePath !== false) {
    window.history.pushState({}, "", postPath(threadID, postPage));
  }

  try {
    const thread = await apiFetch(`/api/forum/threads/${encodeURIComponent(threadID)}`, { auth: false });
    forumState.selectedPostPage = postPage;
    renderThreadDetail(thread);
  } catch (error) {
    renderThreadError("Thread failed to load", error.message);
    showToast(error.message, true);
  }
}

function renderThreadDetail(thread) {
  forumState.selectedThreadID = thread.id;
  setFeedMode("detail");
  const canReply = Boolean(forumState.currentUser);
  $("#thread-detail").innerHTML = `
    <section class="thread-detail-card">
      <p class="eyebrow">${escapeHTML(thread.category?.name || thread.category?.slug || "Forum")}</p>
      <h2>${escapeHTML(thread.title)}</h2>
      <div class="detail-meta">
        ${escapeHTML(thread.author?.name || "Unknown")} · ${formatRelativeTime(thread.created_at)} · ${thread.reply_count} 回复 · ${thread.view_count} 浏览
      </div>
      <p class="thread-body">${escapeHTML(thread.body)}</p>
      <section class="reply-list">
        <h3>回复</h3>
        ${thread.posts?.length ? thread.posts.map(renderReply).join("") : `<div class="notice-row">暂无回复。</div>`}
      </section>
      ${canReply ? renderReplyForm(thread.id) : `<div class="notice-row">登录后参与回复。</div>`}
    </section>
  `;
  const form = $("#reply-form");
  if (form) {
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      const payload = Object.fromEntries(new FormData(form).entries());
      const updated = await apiFetch(`/api/forum/threads/${encodeURIComponent(thread.id)}/posts`, { method: "POST", body: payload });
      form.reset();
      showToast("回复已发布");
      renderThreadDetail(updated);
    });
  }
}

function renderReply(reply) {
  return `
    <article class="reply-row">
      <header>
        <strong>${escapeHTML(reply.author?.name || "Unknown")}</strong>
        <span class="reply-meta">${formatRelativeTime(reply.created_at)}</span>
      </header>
      <p class="reply-body">${escapeHTML(reply.body)}</p>
    </article>
  `;
}

function renderReplyForm(threadID) {
  return `
    <form id="reply-form" class="reply-form" data-thread-id="${escapeAttr(threadID)}">
      <textarea name="body" rows="3" placeholder="添加一条有帮助的回复" required></textarea>
      <button type="submit">回复</button>
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
  updateComposerVisibility();
  $("#current-user-name").textContent = loggedIn ? `${forumState.currentUser.name} 已登录` : "";
}

function setFeedMode(mode) {
  const detailMode = mode === "detail";
  forumState.viewMode = detailMode ? "detail" : "list";
  $(".feed-toolbar").hidden = detailMode;
  $(".board-heading").hidden = detailMode;
  $("#thread-list").hidden = detailMode;
  $("#thread-detail").hidden = !detailMode;
  if (detailMode) {
    $("#thread-list").replaceChildren();
  }
  updateComposerVisibility();
}

function updateComposerVisibility() {
  $("#composer-panel").hidden = !forumState.currentUser || forumState.viewMode === "detail";
}

function renderThreadLoading() {
  $("#thread-detail").innerHTML = `
    <section class="thread-detail-card">
      <div class="empty-state compact">
        <h2>Loading thread</h2>
        <p>Please wait while the discussion loads.</p>
      </div>
    </section>
  `;
}

function renderThreadError(title, message) {
  $("#thread-detail").innerHTML = `
    <section class="thread-detail-card">
      <div class="empty-state compact">
        <h2>${escapeHTML(title)}</h2>
        <p>${escapeHTML(message)}</p>
      </div>
    </section>
  `;
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

function syncSortTabs() {
  document.querySelectorAll("[data-sort-tab]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.sortTab === forumState.sort);
  });
}

function updateBoardHeading() {
  const category = currentCategory();
  $("#board-title").textContent = forumState.category ? (category?.label || category?.name || "论坛") : "全部";
}

function currentCategory() {
  return forumState.categories.find((category) => category.slug === forumState.category) || boardDefinitions[0];
}

function categoryFromPath() {
  const match = window.location.pathname.match(/^\/categories\/([^/]+)\/?$/);
  return match ? decodeURIComponent(match[1]) : "";
}

function postRouteFromPath() {
  const match = window.location.pathname.match(/^\/post-(.+)-(\d+)\/?$/);
  if (!match) return null;
  return {
    threadID: decodeURIComponent(match[1]),
    page: normalizePostPage(match[2]),
  };
}

function postPath(threadID, page = 1) {
  return `/post-${encodeURIComponent(threadID)}-${normalizePostPage(page)}`;
}

function normalizePostPage(page) {
  const number = Number.parseInt(page, 10);
  return Number.isFinite(number) && number > 0 ? number : 1;
}

function resetThreadDetail() {
  $("#thread-detail").innerHTML = `
    <div class="empty-state compact">
      <h2>选择一个主题</h2>
      <p>打开帖子后，可以在这里查看正文与回复。</p>
    </div>
  `;
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

function buildPageSummary(page) {
  const current = page.page || forumState.page;
  if (!page.total_pages || page.total_pages <= 1) return String(current);
  return `${current} / ${page.total_pages}`;
}

function avatarInitial(value) {
  const text = String(value || "?").trim();
  return escapeHTML(text.slice(0, 1).toUpperCase());
}

function avatarHue(value) {
  return Array.from(String(value || "")).reduce((sum, char) => sum + char.charCodeAt(0), 0) % 360;
}

function formatNumber(value) {
  const number = Number(value || 0);
  if (number >= 10000) return `${Math.floor(number / 1000) / 10}w`;
  if (number >= 1000) return `${Math.floor(number / 100) / 10}k`;
  return String(number);
}

function formatRelativeTime(value) {
  if (!value) return "";
  const normalized = value.includes("T") ? value : value.replace(" ", "T") + "Z";
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return value;
  const diff = Date.now() - date.getTime();
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diff < minute) return "刚刚";
  if (diff < hour) return `${Math.floor(diff / minute)}min ago`;
  if (diff < day) return `${Math.floor(diff / hour)}h ago`;
  return `${Math.floor(diff / day)}days ago`;
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
