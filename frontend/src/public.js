import { fetchModels } from "./js/api.js";

const loadingEl = document.getElementById("loading");
const errorEl = document.getElementById("error");
const emptyEl = document.getElementById("empty");
const gridEl = document.getElementById("model-grid");
const searchEl = document.getElementById("search-input");
const brandEl = document.getElementById("brand-filter");
const totalEl = document.getElementById("total-count");
const detailModalEl = document.getElementById("detail-modal");
const detailModalBackdropEl = document.getElementById("detail-modal-backdrop");
const detailModalCloseEl = document.getElementById("detail-modal-close");
const detailModalEmptyEl = document.getElementById("detail-modal-empty");
const detailModalCardEl = document.getElementById("detail-modal-card");

const detailNameEl = document.getElementById("detail-modal-name");
const detailCodeEl = document.getElementById("detail-modal-code");
const detailSubEl = document.getElementById("detail-modal-sub");
const detailMetaEl = document.getElementById("detail-modal-meta");
const detailColorEl = document.getElementById("detail-modal-color");
const detailMaterialEl = document.getElementById("detail-modal-material");
const detailCreatedAtEl = document.getElementById("detail-modal-created-at");
const detailNotesEl = document.getElementById("detail-modal-notes");
const detailTagsEl = document.getElementById("detail-modal-tags");

const detailMainImageEl = document.getElementById("detail-modal-main-image");
const detailMainPlaceholderEl = document.getElementById("detail-modal-main-placeholder");
const detailThumbsEl = document.getElementById("detail-modal-thumbs");

const state = {
  all: [],
  filtered: [],
};

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

function renderTags(tags) {
  if (!Array.isArray(tags) || tags.length === 0) {
    return "";
  }

  return `<div class="tags">${tags
    .map((tag) => `<span class="tag">${escapeHTML(tag)}</span>`)
    .join("")}</div>`;
}

function buildDetailHref(item) {
  if (!item?.id) {
    return "/model.html";
  }
  const query = new URLSearchParams({ id: String(item.id) });
  return `/model.html?${query.toString()}`;
}

function renderCard(item) {
  const brand = item.brand || "Unknown";
  const modelCode = String(item.modelCode || "").trim();
  const scale = item.scale || "-";
  const condition = item.condition || "-";
  const year = item.year || "-";
  const image = item.imageUrl
    ? `<img src="${escapeHTML(item.imageUrl)}" alt="${escapeHTML(item.name)}" loading="lazy" />`
    : '<div class="cover-placeholder">No Image</div>';
  const modelID = Number(item.id);
  const hasModelID = Number.isInteger(modelID) && modelID > 0;

  return `
    <a
      class="card-link"
      href="${buildDetailHref(item)}"
      ${hasModelID ? `data-model-id="${modelID}"` : ""}
      aria-label="查看 ${escapeHTML(item.name)} 详情"
    >
      <article class="model-card">
        <div class="cover">${image}</div>
        <div class="card-body">
          <h3>${escapeHTML(item.name)}</h3>
          <p class="model-code">编号 ${escapeHTML(modelCode || "未填写")}</p>
          <p class="sub">${escapeHTML(brand)} · ${escapeHTML(item.series || "未分类")}</p>
          <p class="meta">年份 ${escapeHTML(year)} · 比例 ${escapeHTML(scale)} · 品相 ${escapeHTML(condition)}</p>
          ${renderTags(item.tags)}
          <p class="note">${escapeHTML(item.notes || "暂无备注")}</p>
        </div>
      </article>
    </a>
  `;
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
    detailMainImageEl.removeAttribute("src");
    detailMainImageEl.classList.add("hidden");
    detailMainPlaceholderEl.classList.remove("hidden");
  } else {
    detailMainImageEl.src = activeImage;
    detailMainImageEl.classList.remove("hidden");
    detailMainPlaceholderEl.classList.add("hidden");
  }

  Array.from(detailThumbsEl.querySelectorAll("button")).forEach((button) => {
    const thumbIndex = Number(button.dataset.index);
    button.classList.toggle("active", thumbIndex === activeImageIndex);
  });
}

function renderThumbs() {
  if (imageList.length <= 1) {
    detailThumbsEl.innerHTML = "";
    detailThumbsEl.classList.add("hidden");
    return;
  }

  detailThumbsEl.classList.remove("hidden");
  detailThumbsEl.innerHTML = imageList
    .map(
      (url, index) => `
      <button type="button" class="thumb-btn ${index === activeImageIndex ? "active" : ""}" data-index="${index}" aria-label="查看第 ${index + 1} 张图片">
        <img src="${escapeHTML(url)}" alt="缩略图 ${index + 1}" loading="lazy" />
      </button>
    `,
    )
    .join("");

  detailThumbsEl.querySelectorAll(".thumb-btn").forEach((button) => {
    button.addEventListener("click", () => {
      const index = Number(button.dataset.index);
      if (!Number.isInteger(index)) {
        return;
      }
      setActiveImage(index);
    });
  });
}

function renderDetailTags(tags) {
  if (!Array.isArray(tags) || tags.length === 0) {
    detailTagsEl.innerHTML = "";
    return;
  }

  detailTagsEl.innerHTML = tags.map((tag) => `<span class="tag">${escapeHTML(tag)}</span>`).join("");
}

function renderDetail(item) {
  const brand = item.brand || "Unknown";
  const scale = item.scale || "-";
  const condition = item.condition || "-";
  const year = item.year || "-";
  const modelCode = String(item.modelCode || "").trim();

  detailNameEl.textContent = item.name || "未命名车型";
  detailCodeEl.textContent = `编号 ${modelCode || "未填写"}`;
  detailSubEl.textContent = `${brand} · ${item.series || "未分类"}`;
  detailMetaEl.textContent = `年份 ${year} · 比例 ${scale} · 品相 ${condition}`;
  detailColorEl.textContent = item.color || "-";
  detailMaterialEl.textContent = item.material || "-";
  detailCreatedAtEl.textContent = formatTime(item.createdAt);
  detailNotesEl.textContent = item.notes || "暂无备注";
  renderDetailTags(item.tags || []);

  imageList = collectImages(item);
  activeImageIndex = 0;
  renderThumbs();
  setActiveImage(activeImageIndex);
}

function closeDetailModal() {
  detailModalEl.classList.add("hidden");
  detailModalEl.setAttribute("aria-hidden", "true");
  document.body.classList.remove("modal-open");
}

function openDetailModal(modelID) {
  const item = state.all.find((entry) => Number(entry.id) === Number(modelID));

  if (!item) {
    detailModalCardEl.classList.add("hidden");
    detailModalEmptyEl.classList.remove("hidden");
  } else {
    renderDetail(item);
    detailModalEmptyEl.classList.add("hidden");
    detailModalCardEl.classList.remove("hidden");
  }

  detailModalEl.classList.remove("hidden");
  detailModalEl.setAttribute("aria-hidden", "false");
  document.body.classList.add("modal-open");
}

function updateState() {
  const query = searchEl.value.trim().toLowerCase();
  const brand = brandEl.value;

  state.filtered = state.all.filter((item) => {
    const haystack = [
      item.name,
      item.modelCode,
      item.series,
      item.notes,
      ...(item.tags || []),
      ...(item.gallery || []),
    ]
      .join(" ")
      .toLowerCase();
    const searchMatched = !query || haystack.includes(query);
    const brandMatched = brand === "all" || item.brand === brand;

    return searchMatched && brandMatched;
  });

  renderGrid();
}

function renderGrid() {
  totalEl.textContent = String(state.all.length);

  if (state.filtered.length === 0) {
    emptyEl.classList.remove("hidden");
    gridEl.innerHTML = "";
    return;
  }

  emptyEl.classList.add("hidden");
  gridEl.innerHTML = state.filtered.map(renderCard).join("");
}

function populateBrands() {
  const brands = [...new Set(state.all.map((item) => item.brand).filter(Boolean))].sort();
  brands.forEach((brand) => {
    const option = document.createElement("option");
    option.value = brand;
    option.textContent = brand;
    brandEl.appendChild(option);
  });
}

async function bootstrap() {
  try {
    const items = await fetchModels();
    state.all = items;
    state.filtered = items;

    loadingEl.classList.add("hidden");
    populateBrands();
    renderGrid();
  } catch (error) {
    loadingEl.classList.add("hidden");
    errorEl.classList.remove("hidden");
    errorEl.textContent = `加载失败：${error.message}`;
  }
}

searchEl.addEventListener("input", updateState);
brandEl.addEventListener("change", updateState);
gridEl.addEventListener("click", (event) => {
  const trigger = event.target.closest(".card-link[data-model-id]");
  if (!trigger || !gridEl.contains(trigger)) {
    return;
  }

  const modelID = Number.parseInt(trigger.dataset.modelId || "", 10);
  if (!Number.isInteger(modelID) || modelID <= 0) {
    return;
  }

  event.preventDefault();
  openDetailModal(modelID);
});
detailModalCloseEl.addEventListener("click", closeDetailModal);
detailModalBackdropEl.addEventListener("click", closeDetailModal);
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !detailModalEl.classList.contains("hidden")) {
    closeDetailModal();
  }
});

bootstrap();
