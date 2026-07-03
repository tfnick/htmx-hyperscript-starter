const boardDefinitions = [
  { slug: "daily", label: "日常", icon: "U" },
  { slug: "tech", label: "技术", icon: "T" },
  { slug: "info", label: "情报", icon: "Q" },
  { slug: "review", label: "测评", icon: "*" },
  { slug: "trade", label: "交易", icon: "$" },
  { slug: "carpool", label: "拼车", icon: "#" },
  { slug: "promotion", label: "推广", icon: "+" },
  { slug: "life", label: "生活", icon: "L" },
  { slug: "dev", label: "Dev", icon: ">" },
  { slug: "image", label: "贴图", icon: "I" },
  { slug: "exposure", label: "曝光", icon: "!" },
  { slug: "sandbox", label: "沙盒", icon: "S" },
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
  renderStaticBoardLinks();
  bindForumEvents();
  refreshAuthState();
  loadCategories();
});

function bindForumEvents() {
  bindListEvents();
  bindAuthEvents();
  bindNewPostEvents();

  const searchForm = $("#search-form");
  if (searchForm) {
    searchForm.addEventListener("submit", (event) => {
      event.preventDefault();
      const searchInput = $("#search-input");
      forumState.search = searchInput ? searchInput.value.trim() : "";
      forumState.page = 1;
      forumState.selectedThreadID = "";
      forumState.selectedPostPage = 1;
      const listPath = forumState.category ? `/categories/${encodeURIComponent(forumState.category)}` : "/";
      if (!$("#thread-list")) {
        window.location.href = listPath;
        return;
      }
      window.history.pushState({}, "", listPath);
      loadThreads();
    });
  }

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
      });
      return;
    }
    loadThreads();
  });
}

function bindAuthEvents() {
  const loginForm = $("#login-form");
  if (loginForm) {
    loginForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const submitButton = form.querySelector('button[type="submit"]');
      const payload = Object.fromEntries(new FormData(form).entries());
      if (submitButton) submitButton.disabled = true;

      try {
        const auth = await apiFetch("/api/auth/login", { method: "POST", body: payload, auth: false });
        storeAuth(auth);
        form.reset();
        window.location.assign("/");
      } catch (error) {
        const message = error instanceof Error && error.message ? error.message : "登录失败，请稍后再试";
        showToast(message, true);
      } finally {
        if (submitButton) submitButton.disabled = false;
      }
    });
  }

  const registerForm = $("#register-form");
  if (registerForm) {
    registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const submitButton = form.querySelector('button[type="submit"]');
      const payload = Object.fromEntries(new FormData(form).entries());
      if (submitButton) submitButton.disabled = true;

      try {
        const auth = await apiFetch("/api/auth/register", { method: "POST", body: payload, auth: false });
        storeAuth(auth);
        form.reset();
        window.location.assign("/");
      } catch (error) {
        const message = error instanceof Error && error.message ? error.message : "注册失败，请稍后再试";
        showToast(message, true);
      } finally {
        if (submitButton) submitButton.disabled = false;
      }
    });
  }

  const logoutButton = $("#logout-button");
  if (logoutButton) {
    logoutButton.addEventListener("click", () => {
      forumState.token = "";
      forumState.refreshToken = "";
      forumState.currentUser = null;
      localStorage.removeItem("forum_access_token");
      localStorage.removeItem("forum_refresh_token");
      renderAuthState();
      showToast("已退出登录");
    });
  }
}

function bindListEvents() {
  const sortSelect = $("#sort-select");
  if (sortSelect) {
    sortSelect.addEventListener("change", (event) => {
      forumState.sort = event.currentTarget.value;
      forumState.page = 1;
      syncSortTabs();
      loadThreads();
    });
  }

  document.querySelectorAll("[data-sort-tab]").forEach((button) => {
    button.addEventListener("click", () => {
      forumState.sort = button.dataset.sortTab;
      if (sortSelect) sortSelect.value = forumState.sort;
      forumState.page = 1;
      syncSortTabs();
      loadThreads();
    });
  });

  const prevPage = $("#prev-page");
  if (prevPage) {
    prevPage.addEventListener("click", () => {
      if (forumState.page <= 1) return;
      forumState.page -= 1;
      loadThreads();
    });
  }

  const nextPage = $("#next-page");
  if (nextPage) {
    nextPage.addEventListener("click", () => {
      forumState.page += 1;
      loadThreads();
    });
  }

}

function bindNewPostEvents() {
  const newThreadForm = $("#new-thread-form");
  if (!newThreadForm) return;

  newThreadForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const submitButton = form.querySelector('button[type="submit"]');
    const payload = Object.fromEntries(new FormData(form).entries());
    if (submitButton) submitButton.disabled = true;

    try {
      const thread = await apiFetch("/api/forum/threads", { method: "POST", body: payload });
      form.reset();
      window.location.href = postPath(thread.id, 1);
    } catch (error) {
      const message = error instanceof Error && error.message ? error.message : "发帖失败，请稍后再试";
      showToast(message, true);
    } finally {
      if (submitButton) submitButton.disabled = false;
    }
  });
}

function renderStaticBoardLinks() {
  const topBoardLinks = $("#top-board-links");
  if (!topBoardLinks) return;
  topBoardLinks.innerHTML = boardDefinitions.slice(0, 8).map((board) => `
    <button type="button" data-top-board="${escapeAttr(board.slug)}">${escapeHTML(board.label)}</button>
  `).join("");
  document.querySelectorAll("[data-top-board]").forEach((button) => {
    button.addEventListener("click", () => selectCategory(button.dataset.topBoard));
  });
}

async function loadCategories() {
  if (!$("#category-list") && !$("#thread-list") && !$("#thread-detail") && !$("#new-thread-category")) return;
  try {
    const data = await apiFetch("/api/forum/categories", { auth: false });
    forumState.categories = mergeBoardDefinitions(data.items || []);
    if (forumState.category && !forumState.categories.some((category) => category.slug === forumState.category)) {
      forumState.category = "";
      window.history.replaceState({}, "", "/");
    }
    renderCategories();
    renderNewThreadCategoryOptions();
    updateBoardHeading();
    if ($("#new-thread-category") && !$("#thread-list") && !$("#thread-detail")) return;
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
    renderNewThreadCategoryOptions();
    updateBoardHeading();
    renderEmptyThreads("暂时没有内容", error.message);
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
  const categoryList = $("#category-list");
  if (categoryList) {
    categoryList.innerHTML = forumState.categories.map((category) => `
      <button class="category-button" type="button" data-category="${escapeAttr(category.slug)}" aria-current="${category.slug === forumState.category}">
        <span class="category-icon">${escapeHTML(category.icon)}</span>
        <span>${escapeHTML(category.label)}</span>
      </button>
    `).join("");
  }

  document.querySelectorAll("[data-category]").forEach((button) => {
    button.addEventListener("click", () => selectCategory(button.dataset.category));
  });

  document.querySelectorAll("[data-top-board]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.topBoard === forumState.category);
  });
  updateCreatePostLink();
}

function selectCategory(slug) {
  const categoryPath = `/categories/${encodeURIComponent(slug)}`;
  if (!$("#thread-list")) {
    window.location.href = categoryPath;
    return;
  }
  if (forumState.category === slug && !forumState.selectedThreadID) return;
  forumState.category = slug;
  forumState.page = 1;
  forumState.selectedThreadID = "";
  forumState.selectedPostPage = 1;
  window.history.pushState({}, "", categoryPath);
  renderCategories();
  updateBoardHeading();
  loadThreads();
}

async function loadThreads() {
  const threadList = $("#thread-list");
  if (!threadList) return;
  forumState.viewMode = "list";
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
  const threadList = $("#thread-list");
  if (!threadList) return;
  const items = data.items || [];
  if (!items.length) {
    renderEmptyThreads("还没有主题", "成为这个板块第一个发起讨论的人。");
  } else {
    threadList.innerHTML = items.map(renderThreadRow).join("");
    document.querySelectorAll("[data-thread-id]").forEach((button) => {
      button.addEventListener("click", () => loadThread(button.dataset.threadId, { postPage: 1 }));
    });
  }

  const page = data.pagination || {};
  const pageSummary = $("#page-summary");
  const prevPage = $("#prev-page");
  const nextPage = $("#next-page");
  if (pageSummary) pageSummary.textContent = buildPageSummary(page);
  if (prevPage) prevPage.disabled = !page.has_previous;
  if (nextPage) nextPage.disabled = !page.has_next;
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
          <span>作者 ${escapeHTML(thread.author?.name || "Unknown")}</span>
          <span>浏览 ${formatNumber(thread.view_count)}</span>
          <span>回复 ${formatNumber(thread.reply_count)}</span>
          <span>最后 ${escapeHTML(lastPoster)}</span>
          <span>${formatRelativeTime(thread.last_post_at || thread.created_at)}</span>
        </span>
      </span>
      <span class="thread-category">${escapeHTML(category.name || category.label || category.slug)}</span>
    </button>
  `;
}

function renderEmptyThreads(title, message) {
  const threadList = $("#thread-list");
  if (!threadList) return;
  threadList.innerHTML = `
    <article class="empty-state">
      <h2>${escapeHTML(title)}</h2>
      <p>${escapeHTML(message)}</p>
    </article>
  `;
  const pageSummary = $("#page-summary");
  const prevPage = $("#prev-page");
  const nextPage = $("#next-page");
  if (pageSummary) pageSummary.textContent = String(forumState.page);
  if (prevPage) prevPage.disabled = forumState.page <= 1;
  if (nextPage) nextPage.disabled = true;
}

async function loadThread(threadID, options = {}) {
  const postPage = normalizePostPage(options.postPage);
  const path = postPath(threadID, postPage);
  if (!$("#thread-detail")) {
    window.location.href = path;
    return;
  }
  forumState.selectedThreadID = threadID;
  forumState.selectedPostPage = postPage;
  forumState.viewMode = "detail";
  renderThreadLoading();
  if (options.updatePath !== false) {
    window.history.pushState({}, "", path);
  }

  try {
    const thread = await apiFetch(`/api/forum/threads/${encodeURIComponent(threadID)}`);
    renderThreadDetail(thread);
  } catch (error) {
    renderThreadError("Thread failed to load", error.message);
    showToast(error.message, true);
  }
}

function renderThreadDetail(thread) {
  const threadDetail = $("#thread-detail");
  if (!threadDetail) return;
  forumState.selectedThreadID = thread.id;
  forumState.viewMode = "detail";
  const canReply = Boolean(forumState.currentUser);
  threadDetail.innerHTML = `
    <section class="thread-detail-card">
      <p class="eyebrow">${escapeHTML(thread.category?.name || thread.category?.slug || "Forum")}</p>
      <h2>${escapeHTML(thread.title)}</h2>
      <div class="detail-meta">
        ${escapeHTML(thread.author?.name || "Unknown")} · ${formatRelativeTime(thread.created_at)} · ${formatNumber(thread.reply_count)} 回复 · ${formatNumber(thread.view_count)} 浏览
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
  const loggedOutPanel = $("#logged-out-panel");
  const loggedInPanel = $("#logged-in-panel");
  const currentUserName = $("#current-user-name");
  if (loggedOutPanel) loggedOutPanel.hidden = loggedIn;
  if (loggedInPanel) loggedInPanel.hidden = !loggedIn;
  if (currentUserName) currentUserName.textContent = loggedIn ? `${forumState.currentUser.name} 已登录` : "";
  updateCreatePostLink();
  updateComposerVisibility();
  updateNewPostVisibility();
}

function updateComposerVisibility() {
  const composer = $("#composer-panel");
  if (!composer) return;
  composer.hidden = !forumState.currentUser || forumState.viewMode === "detail";
}

function updateCreatePostLink() {
  const link = $("#create-post-link");
  if (!link) return;
  link.href = forumState.category ? `/new-post?category=${encodeURIComponent(forumState.category)}` : "/new-post";
}

function updateNewPostVisibility() {
  const panel = $("#new-post-panel");
  const authRequired = $("#new-post-auth-required");
  if (!panel && !authRequired) return;

  const loggedIn = Boolean(forumState.currentUser);
  if (panel) panel.hidden = !loggedIn;
  if (authRequired) authRequired.hidden = loggedIn;
}

function renderNewThreadCategoryOptions() {
  const select = $("#new-thread-category");
  if (!select) return;

  const categories = forumState.categories.filter((category) => category.enabled !== false);
  if (!categories.length) {
    select.innerHTML = `<option value="daily">日常</option>`;
    select.value = "daily";
    return;
  }

  select.innerHTML = categories.map((category) => `
    <option value="${escapeAttr(category.slug)}">${escapeHTML(category.label || category.name || category.slug)}</option>
  `).join("");

  const requestedCategory = categoryFromQuery();
  const fallbackCategory = categories.find((category) => category.slug === "daily")?.slug || categories[0].slug;
  select.value = categories.some((category) => category.slug === requestedCategory) ? requestedCategory : fallbackCategory;
}

function renderThreadLoading() {
  const threadDetail = $("#thread-detail");
  if (!threadDetail) return;
  threadDetail.innerHTML = `
    <section class="thread-detail-card">
      <div class="empty-state compact">
        <h2>Loading thread</h2>
        <p>Please wait while the discussion loads.</p>
      </div>
    </section>
  `;
}

function renderThreadError(title, message) {
  const threadDetail = $("#thread-detail");
  if (!threadDetail) return;
  threadDetail.innerHTML = `
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
  const boardTitle = $("#board-title");
  if (!boardTitle) return;
  const category = currentCategory();
  boardTitle.textContent = forumState.category ? (category?.label || category?.name || "论坛") : "全部";
}

function currentCategory() {
  return forumState.categories.find((category) => category.slug === forumState.category) || boardDefinitions[0];
}

function categoryFromPath() {
  const match = window.location.pathname.match(/^\/categories\/([^/]+)\/?$/);
  return match ? decodeURIComponent(match[1]) : "";
}

function categoryFromQuery() {
  return new URLSearchParams(window.location.search).get("category") || "";
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

function showToast(message, error = false) {
  const toast = $("#toast");
  if (!toast) return;
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
