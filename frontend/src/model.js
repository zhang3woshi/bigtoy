import { fetchModels } from "./js/api.js";

const loadingEl = document.getElementById("detail-loading");
const errorEl = document.getElementById("detail-error");
const emptyEl = document.getElementById("detail-empty");
const cardEl = document.getElementById("detail-card");

const nameEl = document.getElementById("detail-name");
const codeEl = document.getElementById("detail-code");
const subEl = document.getElementById("detail-sub");
const metaEl = document.getElementById("detail-meta");
const colorEl = document.getElementById("detail-color");
const materialEl = document.getElementById("detail-material");
const createdAtEl = document.getElementById("detail-created-at");
const notesEl = document.getElementById("detail-notes");
const tagsEl = document.getElementById("detail-tags");

const mainImageEl = document.getElementById("detail-main-image");
const mainPlaceholderEl = document.getElementById("detail-main-placeholder");
const thumbsEl = document.getElementById("detail-thumbs");

let imageList = [];
let activeImageIndex = 0;

function escapeHTML(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function setState({ loading = false, error = "", empty = false, showCard = false }) {
  loadingEl.classList.toggle("hidden", !loading);
  errorEl.classList.toggle("hidden", !error);
  emptyEl.classList.toggle("hidden", !empty);
  cardEl.classList.toggle("hidden", !showCard);

  errorEl.textContent = error || "";
}

function formatTime(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function collectImages(item) {
  const seen = new Set();
  const collected = [];

  for (const value of [item.imageUrl, ...(item.gallery || [])]) {
    const url = String(value || "").trim();
    if (!url || seen.has(url)) {
      continue;
    }
    seen.add(url);
    collected.push(url);
  }

  return collected;
}

function setActiveImage(index) {
  activeImageIndex = index;
  const activeImage = imageList[activeImageIndex];

  if (!activeImage) {
    mainImageEl.classList.add("hidden");
    mainPlaceholderEl.classList.remove("hidden");
  } else {
    mainImageEl.src = activeImage;
    mainImageEl.classList.remove("hidden");
    mainPlaceholderEl.classList.add("hidden");
  }

  Array.from(thumbsEl.querySelectorAll("button")).forEach((button) => {
    const thumbIndex = Number(button.dataset.index);
    button.classList.toggle("active", thumbIndex === activeImageIndex);
  });
}

function renderThumbs() {
  if (imageList.length <= 1) {
    thumbsEl.innerHTML = "";
    thumbsEl.classList.add("hidden");
    return;
  }

  thumbsEl.classList.remove("hidden");
  thumbsEl.innerHTML = imageList
    .map(
      (url, index) => `
      <button type="button" class="thumb-btn ${index === activeImageIndex ? "active" : ""}" data-index="${index}" aria-label="查看第 ${index + 1} 张图片">
        <img src="${escapeHTML(url)}" alt="缩略图 ${index + 1}" loading="lazy" />
      </button>
    `,
    )
    .join("");

  thumbsEl.querySelectorAll(".thumb-btn").forEach((button) => {
    button.addEventListener("click", () => {
      const index = Number(button.dataset.index);
      if (!Number.isInteger(index)) {
        return;
      }
      setActiveImage(index);
    });
  });
}

function renderTags(tags) {
  if (!Array.isArray(tags) || tags.length === 0) {
    tagsEl.innerHTML = "";
    return;
  }

  tagsEl.innerHTML = tags.map((tag) => `<span class="tag">${escapeHTML(tag)}</span>`).join("");
}

function renderModel(item) {
  const brand = item.brand || "Unknown";
  const scale = item.scale || "-";
  const condition = item.condition || "-";
  const year = item.year || "-";
  const modelCode = String(item.modelCode || "").trim();

  nameEl.textContent = item.name || "未命名车型";
  codeEl.textContent = `编号 ${modelCode || "未填写"}`;
  subEl.textContent = `${brand} · ${item.series || "未分类"}`;
  metaEl.textContent = `年份 ${year} · 比例 ${scale} · 品相 ${condition}`;

  colorEl.textContent = item.color || "-";
  materialEl.textContent = item.material || "-";
  createdAtEl.textContent = formatTime(item.createdAt);
  notesEl.textContent = item.notes || "暂无备注";

  renderTags(item.tags || []);

  imageList = collectImages(item);
  activeImageIndex = 0;
  renderThumbs();
  setActiveImage(activeImageIndex);

  document.title = `${item.name || "车模详情"} | BigToy Garage`;
}

async function bootstrap() {
  const idValue = new URLSearchParams(window.location.search).get("id");
  const modelID = Number.parseInt(String(idValue || ""), 10);
  if (!Number.isInteger(modelID) || modelID <= 0) {
    setState({ error: "详情链接缺少有效的车型 ID。" });
    return;
  }

  setState({ loading: true });

  try {
    const models = await fetchModels();
    const item = models.find((entry) => Number(entry.id) === modelID);

    if (!item) {
      setState({ empty: true });
      return;
    }

    renderModel(item);
    setState({ showCard: true });
  } catch (error) {
    setState({ error: `加载详情失败：${error.message}` });
  }
}

bootstrap();
