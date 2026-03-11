import { fetchModels } from "./js/api.js";

const loadingEl = document.getElementById("loading");
const errorEl = document.getElementById("error");
const emptyEl = document.getElementById("empty");
const gridEl = document.getElementById("model-grid");
const searchEl = document.getElementById("search-input");
const brandEl = document.getElementById("brand-filter");
const totalEl = document.getElementById("total-count");

const state = {
  all: [],
  filtered: [],
};

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

  return `
    <a class="card-link" href="${buildDetailHref(item)}" aria-label="查看 ${escapeHTML(item.name)} 详情">
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

bootstrap();
