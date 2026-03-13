<template>
  <div class="showcase-root">
    <header class="showcase-topbar">
      <a class="showcase-brand" href="/index.html" aria-label="返回首页">
        <span class="showcase-brand-icon" aria-hidden="true">
          <svg viewBox="0 0 48 48" focusable="false">
            <path d="M10 28.5h28v6H10z" />
            <path d="M13 28.5l4.2-8.5h13.6l4.2 8.5z" />
            <circle cx="16.5" cy="36.5" r="3.4" />
            <circle cx="31.5" cy="36.5" r="3.4" />
          </svg>
        </span>
        <span class="showcase-brand-copy">
          <strong>Zhangyu Diecast Collection</strong>
          <small>火柴盒与风火轮车模收藏</small>
        </span>
      </a>

      <nav class="showcase-nav" aria-label="主导航">
        <a class="showcase-nav-link is-active" href="/index.html">首页</a>
        <a class="showcase-nav-link" href="#latest">最新收藏</a>
        <a class="showcase-nav-link" href="#browse">分类浏览</a>
        <a class="showcase-nav-link" href="#collection">全部收藏</a>
        <a class="showcase-nav-link showcase-nav-link-cta" href="/login.html">录入后台</a>
      </nav>

      <div class="showcase-search">
        <input
          v-model.trim="searchQuery"
          class="input"
          type="search"
          placeholder="搜索车型 / 品牌 / 标签"
          aria-label="搜索收藏"
        />
      </div>
    </header>

    <main class="showcase-main">
      <section class="showcase-hero-bleed">
        <div class="showcase-hero-window" role="button" tabindex="0" @click="handleRandomOpen" @keydown.enter="handleHeroEnter">
          <img
            v-if="randomCurrent?.imageUrl"
            class="showcase-hero-image"
            :src="randomCurrent.imageUrl"
            :alt="randomCurrent.name || '车模图片'"
            :style="heroImageStyle"
            loading="lazy"
            @load="handleHeroImageLoad"
          />
          <div v-else class="showcase-hero-empty">{{ randomEmptyLabel }}</div>

          <div class="showcase-hero-mask"></div>

          <div class="showcase-hero-content">
            <p class="showcase-hero-kicker">Zhangyu Diecast Collection</p>
            <h1 class="showcase-hero-title">火柴盒 & 风火轮车模收藏</h1>
            <p class="showcase-hero-meta">
              收藏总量 <strong>{{ totalCount }}</strong> 台
              <span>·</span>
              主力品牌 {{ topBrandSummary }}
              <span>·</span>
              年代跨度 {{ collectionYearRange }}
            </p>
            <a class="showcase-hero-btn" href="#collection" @click.stop @keydown.enter.stop>浏览全部收藏</a>
          </div>

          <button class="showcase-hero-nav showcase-hero-nav-prev" type="button" aria-label="上一张" @click.stop="prevRandom">
            ‹
          </button>
          <button class="showcase-hero-nav showcase-hero-nav-next" type="button" aria-label="下一张" @click.stop="nextRandom">
            ›
          </button>
        </div>

        <div class="showcase-hero-dots" aria-label="轮播图导航">
          <button
            v-for="(_item, index) in randomModels"
            :key="`hero-dot-${index}`"
            type="button"
            class="showcase-hero-dot"
            :class="{ active: index === randomIndex }"
            :aria-label="`切换到第 ${index + 1} 张`"
            @click="goToRandom(index)"
          ></button>
        </div>
      </section>

      <section class="showcase-stats" aria-label="收藏统计">
        <article class="showcase-stat">
          <small>Total Cars</small>
          <strong>{{ totalCount }}</strong>
        </article>
        <article class="showcase-stat">
          <small>Hot Wheels</small>
          <strong>{{ hotWheelsCount }}</strong>
        </article>
        <article class="showcase-stat">
          <small>Matchbox</small>
          <strong>{{ matchboxCount }}</strong>
        </article>
        <article class="showcase-stat">
          <small>Rare Models</small>
          <strong>{{ rareCount }}</strong>
        </article>
      </section>

      <section id="latest" class="showcase-section">
        <div class="showcase-heading">
          <span></span>
          <h2>最新收藏 <em>Latest Acquisitions</em></h2>
          <span></span>
        </div>

        <div v-if="latestModels.length > 0" class="showcase-latest-grid">
          <article
            v-for="item in latestModels"
            :key="item.id || item.name"
            class="showcase-latest-card"
            role="button"
            tabindex="0"
            @click="openDetailModal(item)"
            @keydown.enter.prevent="openDetailModal(item)"
          >
            <div class="showcase-latest-cover">
              <img v-if="item.imageUrl" :src="item.imageUrl" :alt="item.name || '车模图片'" loading="lazy" />
              <div v-else class="cover-placeholder">No Image</div>
            </div>
            <p class="showcase-latest-name">{{ item.name || "未命名车型" }}</p>
            <p class="showcase-latest-date">{{ formatDate(item.createdAt) }} 入库</p>
          </article>
        </div>
        <div v-else class="state-card">暂无最新收藏数据。</div>
      </section>

      <section id="browse" class="showcase-section">
        <div class="showcase-heading">
          <span></span>
          <h2>分类浏览 <em>Browse</em></h2>
          <span></span>
        </div>

        <div class="showcase-browse-grid">
          <article class="showcase-browse-tile" :style="tileBackground(hotWheelsHero?.imageUrl)">
            <h3>Hot Wheels</h3>
            <p>{{ hotWheelsCount }} 台收藏</p>
          </article>
          <article class="showcase-browse-tile" :style="tileBackground(matchboxHero?.imageUrl)">
            <h3>Matchbox</h3>
            <p>{{ matchboxCount }} 台收藏</p>
          </article>
          <article class="showcase-browse-tile">
            <h3>热门标签</h3>
            <p>{{ topTagsLine }}</p>
          </article>
          <article class="showcase-browse-tile">
            <h3>年代分布</h3>
            <ul class="showcase-year-list">
              <li v-for="bucket in yearBuckets" :key="bucket.label">
                <span>{{ bucket.label }}</span>
                <strong>{{ bucket.count }}</strong>
              </li>
            </ul>
          </article>
        </div>
      </section>

      <section id="collection" class="showcase-section">
        <div class="showcase-heading">
          <span></span>
          <h2>全部收藏 <em>Collection</em></h2>
          <span></span>
        </div>

        <section class="collection-tools" aria-label="搜索筛选">
          <div class="filters">
            <input
              v-model.trim="searchQuery"
              class="input"
              type="search"
              placeholder="搜索名称 / 编号 / 系列 / 标签"
            />
            <select v-model="brandFilter" class="input">
              <option value="all">全部品牌</option>
              <option v-for="brand in brands" :key="brand" :value="brand">
                {{ brand }}
              </option>
            </select>
            <div class="sort-controls">
              <select v-model="sortField" class="input" aria-label="排序字段">
                <option value="createdAt">加入时间</option>
                <option value="modelCode">编号</option>
              </select>
              <button
                type="button"
                class="sort-order-toggle"
                :aria-label="sortDirectionAriaLabel"
                :title="sortDirectionAriaLabel"
                @click="toggleSortDirection"
              >
                <span class="sort-order-icon" aria-hidden="true">{{ sortDirectionIcon }}</span>
                <span>{{ sortDirectionLabel }}</span>
              </button>
            </div>
          </div>
        </section>

        <div v-if="loading" class="state-card">正在加载车模数据...</div>
        <div v-else-if="errorMessage" class="state-card state-error">{{ errorMessage }}</div>
        <div v-else-if="showEmpty" class="state-card">没有匹配结果，试试其他关键词。</div>
        <div v-else class="card-grid">
          <ModelGridCard v-for="item in filteredModels" :key="item.id || item.name" :item="item" @open="openDetailModal" />
        </div>
      </section>
    </main>

    <footer class="showcase-footer">
      <p class="showcase-footer-brand">Zhangyu Diecast Collection</p>
      <p>收藏起始 {{ timelineStartYear }} · 当前总数 {{ totalCount }}</p>
      <a href="https://beian.miit.gov.cn" target="_blank" rel="noopener noreferrer">浙ICP备2022001311号-1</a>
    </footer>

    <div class="detail-modal" :class="{ hidden: !detailVisible }" :aria-hidden="(!detailVisible).toString()">
      <div class="detail-modal-backdrop" @click="closeDetailModal"></div>
      <div class="detail-modal-panel" role="dialog" aria-modal="true" aria-labelledby="detail-modal-name">
        <button class="detail-modal-close" type="button" aria-label="关闭详情" @click="closeDetailModal">×</button>
        <div v-if="detailVisible && !activeDetailItem" class="state-card">未找到该车模，可能已被删除。</div>
        <ModelDetailCard v-if="detailVisible && activeDetailItem" :item="activeDetailItem" title-id="detail-modal-name" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import ModelDetailCard from "../components/ModelDetailCard.vue";
import ModelGridCard from "../components/ModelGridCard.vue";
import { useRandomShowcase } from "../composables/useRandomShowcase.js";
import { fetchModels } from "../js/api.js";
import { filterModels, getBrandList, sortByLatest } from "../utils/model.js";

const allModels = ref([]);
const loading = ref(true);
const errorMessage = ref("");
const searchQuery = ref("");
const brandFilter = ref("all");
const sortField = ref("createdAt");
const sortDirection = ref("desc");

const detailVisible = ref(false);
const activeDetailItem = ref(null);

const DEFAULT_HERO_IMAGE_POSITION = "50% 50%";
const heroImageObjectPosition = ref(DEFAULT_HERO_IMAGE_POSITION);
const heroFocusCache = new Map();

const {
  randomModels,
  randomIndex,
  randomCurrent,
  startRandomShowcase,
  stopRandomShowcase,
  prevRandom,
  nextRandom,
  goToRandom,
} = useRandomShowcase(allModels, {
  count: 5,
  slideInterval: 4200,
  refreshInterval: 30000,
});

const brands = computed(() => getBrandList(allModels.value));
const filteredModels = computed(() => {
  const filtered = filterModels(allModels.value, {
    query: searchQuery.value,
    brand: brandFilter.value,
  });
  return sortCollectionModels(filtered, {
    field: sortField.value,
    direction: sortDirection.value,
  });
});
const showEmpty = computed(() => !loading.value && !errorMessage.value && filteredModels.value.length === 0);

const totalCount = computed(() => allModels.value.length);
const latestModels = computed(() => sortByLatest(allModels.value).slice(0, 5));

const hotWheelsCount = computed(
  () => allModels.value.filter((item) => String(item.brand || "").toLowerCase().includes("hot wheels")).length,
);
const matchboxCount = computed(
  () => allModels.value.filter((item) => String(item.brand || "").toLowerCase().includes("matchbox")).length,
);
const rareCount = computed(() =>
  allModels.value.filter((item) => {
    const tags = Array.isArray(item.tags) ? item.tags.join(" ").toLowerCase() : "";
    const text = [item.series, item.notes, item.name].join(" ").toLowerCase();
    return /rare|super treasure hunt|treasure hunt|sth|limited|限量|稀有/.test(`${tags} ${text}`);
  }).length,
);

const hotWheelsHero = computed(() =>
  allModels.value.find((item) => String(item.brand || "").toLowerCase().includes("hot wheels") && item.imageUrl),
);
const matchboxHero = computed(() =>
  allModels.value.find((item) => String(item.brand || "").toLowerCase().includes("matchbox") && item.imageUrl),
);

const topBrandSummary = computed(() => {
  const values = brands.value.slice(0, 2);
  return values.length > 0 ? values.join(" / ") : "暂无";
});

const parsedYears = computed(() =>
  allModels.value
    .map((item) => Number.parseInt(String(item.year || "").trim(), 10))
    .filter((value) => Number.isInteger(value) && value > 0),
);

const collectionYearRange = computed(() => {
  const years = [...parsedYears.value].sort((a, b) => a - b);
  if (years.length === 0) {
    return "-";
  }
  return years[0] === years[years.length - 1] ? String(years[0]) : `${years[0]}-${years[years.length - 1]}`;
});

const timelineStartYear = computed(() => {
  if (parsedYears.value.length === 0) {
    return 2018;
  }
  return Math.min(...parsedYears.value);
});

const yearBuckets = computed(() => {
  const buckets = [
    { label: "1968-1989", count: 0, match: (year) => year >= 1968 && year <= 1989 },
    { label: "1990-2009", count: 0, match: (year) => year >= 1990 && year <= 2009 },
    { label: "2010-2019", count: 0, match: (year) => year >= 2010 && year <= 2019 },
    { label: "2020+", count: 0, match: (year) => year >= 2020 },
  ];

  for (const year of parsedYears.value) {
    for (const bucket of buckets) {
      if (bucket.match(year)) {
        bucket.count += 1;
      }
    }
  }

  return buckets;
});

const topTagsLine = computed(() => {
  const tagMap = new Map();
  for (const item of allModels.value) {
    const tags = Array.isArray(item.tags) ? item.tags : [];
    for (const tag of tags) {
      const normalized = String(tag || "").trim();
      if (!normalized) {
        continue;
      }
      tagMap.set(normalized, (tagMap.get(normalized) || 0) + 1);
    }
  }

  const top = [...tagMap.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 3)
    .map(([tag]) => tag);

  return top.length > 0 ? top.join(" / ") : "Premium / Mainline / Treasure Hunt";
});

const randomEmptyLabel = computed(() => {
  if (loading.value) {
    return "加载中...";
  }
  if (errorMessage.value) {
    return "加载失败";
  }
  return "暂无车模";
});

const heroImageStyle = computed(() => ({
  objectPosition: heroImageObjectPosition.value,
}));
const sortDirectionLabel = computed(() => (sortDirection.value === "desc" ? "降序" : "升序"));
const sortDirectionIcon = computed(() => (sortDirection.value === "desc" ? "↓" : "↑"));
const sortDirectionAriaLabel = computed(() =>
  sortDirection.value === "desc" ? "当前降序，点击切换为升序" : "当前升序，点击切换为降序",
);

function toSafeTimestamp(value) {
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function compareLocaleValue(left, right, direction = "asc") {
  const compared = String(left || "").trim().localeCompare(String(right || "").trim(), "zh-CN", {
    numeric: true,
    sensitivity: "base",
  });
  return direction === "desc" ? -compared : compared;
}

function sortCollectionModels(items, { field = "createdAt", direction = "desc" } = {}) {
  const values = [...(items || [])];
  const normalizedDirection = direction === "asc" ? "asc" : "desc";
  if (field === "modelCode") {
    return values.sort((a, b) => {
      const codeCompared = compareLocaleValue(a?.modelCode, b?.modelCode, normalizedDirection);
      if (codeCompared !== 0) {
        return codeCompared;
      }
      return compareLocaleValue(a?.id, b?.id, normalizedDirection);
    });
  }

  return values.sort((a, b) => {
    const timeCompared =
      normalizedDirection === "asc"
        ? toSafeTimestamp(a?.createdAt) - toSafeTimestamp(b?.createdAt)
        : toSafeTimestamp(b?.createdAt) - toSafeTimestamp(a?.createdAt);
    if (timeCompared !== 0) {
      return timeCompared;
    }
    return compareLocaleValue(a?.id, b?.id, normalizedDirection);
  });
}

function toggleSortDirection() {
  sortDirection.value = sortDirection.value === "desc" ? "asc" : "desc";
}

function clampNumber(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function toPercent(value, min, max) {
  const clamped = clampNumber(value, min, max);
  return `${(Math.round(clamped * 10) / 10).toFixed(1)}%`;
}

function sampleBackgroundColor(imageData, width, height) {
  const pixels = [
    [0, 0],
    [Math.max(0, width - 1), 0],
    [0, Math.max(0, height - 1)],
    [Math.max(0, width - 1), Math.max(0, height - 1)],
    [Math.floor(width / 2), 0],
    [Math.floor(width / 2), Math.max(0, height - 1)],
    [0, Math.floor(height / 2)],
    [Math.max(0, width - 1), Math.floor(height / 2)],
  ];

  let red = 0;
  let green = 0;
  let blue = 0;

  for (const [x, y] of pixels) {
    const offset = (y * width + x) * 4;
    red += imageData[offset] || 0;
    green += imageData[offset + 1] || 0;
    blue += imageData[offset + 2] || 0;
  }

  const divisor = Math.max(1, pixels.length);
  return {
    red: red / divisor,
    green: green / divisor,
    blue: blue / divisor,
  };
}

function detectImageFocusPosition(imageElement) {
  const naturalWidth = Number(imageElement?.naturalWidth || 0);
  const naturalHeight = Number(imageElement?.naturalHeight || 0);
  if (naturalWidth <= 0 || naturalHeight <= 0 || typeof document === "undefined") {
    return DEFAULT_HERO_IMAGE_POSITION;
  }

  const maxEdge = 300;
  const scale = Math.min(1, maxEdge / Math.max(naturalWidth, naturalHeight));
  const width = Math.max(40, Math.round(naturalWidth * scale));
  const height = Math.max(40, Math.round(naturalHeight * scale));

  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;

  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) {
    return DEFAULT_HERO_IMAGE_POSITION;
  }

  try {
    context.drawImage(imageElement, 0, 0, width, height);
    const imageData = context.getImageData(0, 0, width, height).data;
    const luminance = new Float32Array(width * height);

    for (let index = 0; index < width * height; index += 1) {
      const offset = index * 4;
      const red = imageData[offset];
      const green = imageData[offset + 1];
      const blue = imageData[offset + 2];
      luminance[index] = red * 0.299 + green * 0.587 + blue * 0.114;
    }

    const background = sampleBackgroundColor(imageData, width, height);
    let totalScore = 0;
    let weightedX = 0;
    let weightedY = 0;

    for (let y = 1; y < height - 1; y += 1) {
      for (let x = 1; x < width - 1; x += 1) {
        const index = y * width + x;
        const offset = index * 4;
        const alpha = imageData[offset + 3];
        if (alpha < 10) {
          continue;
        }

        const edgeScore =
          Math.abs(luminance[index - 1] - luminance[index + 1]) +
          Math.abs(luminance[index - width] - luminance[index + width]);
        const colorScore =
          Math.abs(imageData[offset] - background.red) +
          Math.abs(imageData[offset + 1] - background.green) +
          Math.abs(imageData[offset + 2] - background.blue);

        let score = edgeScore * 0.58 + colorScore * 0.42;
        if (score < 16) {
          continue;
        }

        const centerX = (x / Math.max(1, width - 1)) * 2 - 1;
        const centerY = (y / Math.max(1, height - 1)) * 2 - 1;
        const radialDistance = Math.min(1, Math.sqrt(centerX * centerX + centerY * centerY));
        score *= 1.12 - radialDistance * 0.28;

        totalScore += score;
        weightedX += score * x;
        weightedY += score * y;
      }
    }

    if (totalScore <= 0) {
      return DEFAULT_HERO_IMAGE_POSITION;
    }

    const xPercent = (weightedX / totalScore / Math.max(1, width - 1)) * 100;
    const yPercent = (weightedY / totalScore / Math.max(1, height - 1)) * 100;
    return `${toPercent(xPercent, 8, 92)} ${toPercent(yPercent, 10, 90)}`;
  } catch (_error) {
    return DEFAULT_HERO_IMAGE_POSITION;
  }
}

function syncHeroImagePosition(imageURL) {
  const normalizedURL = String(imageURL || "").trim();
  if (!normalizedURL) {
    heroImageObjectPosition.value = DEFAULT_HERO_IMAGE_POSITION;
    return;
  }
  heroImageObjectPosition.value = heroFocusCache.get(normalizedURL) || DEFAULT_HERO_IMAGE_POSITION;
}

function handleHeroImageLoad(event) {
  const imageElement = event?.target;
  if (!(imageElement instanceof HTMLImageElement)) {
    return;
  }

  const imageURL = String(randomCurrent.value?.imageUrl || imageElement.currentSrc || imageElement.src || "").trim();
  if (!imageURL) {
    return;
  }

  if (!heroFocusCache.has(imageURL)) {
    heroFocusCache.set(imageURL, detectImageFocusPosition(imageElement));
  }

  if (String(randomCurrent.value?.imageUrl || "").trim() === imageURL) {
    heroImageObjectPosition.value = heroFocusCache.get(imageURL) || DEFAULT_HERO_IMAGE_POSITION;
  }
}

function formatDate(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleDateString("zh-CN");
}

function tileBackground(imageURL) {
  if (!imageURL) {
    return {};
  }
  return {
    backgroundImage: `linear-gradient(120deg, rgba(10, 20, 37, 0.56), rgba(15, 26, 44, 0.42)), url("${imageURL}")`,
  };
}

function closeDetailModal() {
  detailVisible.value = false;
}

function openDetailModal(payload) {
  if (payload && typeof payload === "object") {
    activeDetailItem.value = payload;
    detailVisible.value = true;
    return;
  }

  const normalizedID = String(payload || "").trim();
  const item = normalizedID
    ? allModels.value.find((entry) => String(entry?.id || "").trim() === normalizedID) || null
    : null;
  activeDetailItem.value = item;
  detailVisible.value = true;
}

function handleRandomOpen() {
  if (!randomCurrent.value) {
    return;
  }
  openDetailModal(randomCurrent.value);
}

function handleHeroEnter(event) {
  if (!event || event.target !== event.currentTarget) {
    return;
  }
  event.preventDefault();
  handleRandomOpen();
}

function handleKeydown(event) {
  if (event.key === "Escape" && detailVisible.value) {
    closeDetailModal();
  }
}

watch(detailVisible, (visible) => {
  document.body.classList.toggle("modal-open", visible);
});

watch(
  () => String(randomCurrent.value?.imageUrl || "").trim(),
  (imageURL) => {
    syncHeroImagePosition(imageURL);
  },
  { immediate: true },
);

onMounted(async () => {
  document.addEventListener("keydown", handleKeydown);

  try {
    allModels.value = await fetchModels();
    startRandomShowcase();
  } catch (error) {
    errorMessage.value = `加载失败：${error.message}`;
  } finally {
    loading.value = false;
  }
});

onUnmounted(() => {
  document.removeEventListener("keydown", handleKeydown);
  document.body.classList.remove("modal-open");
  stopRandomShowcase();
});
</script>
