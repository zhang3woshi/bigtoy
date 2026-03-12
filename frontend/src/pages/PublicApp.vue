<template>
  <header class="topbar">
    <div class="brand-block">
      <span class="brand-icon" aria-hidden="true">
        <svg viewBox="0 0 48 48" focusable="false">
          <path d="M10 28.5h28v6H10z" />
          <path d="M13 28.5l4.2-8.5h13.6l4.2 8.5z" />
          <circle cx="16.5" cy="36.5" r="3.4" />
          <circle cx="31.5" cy="36.5" r="3.4" />
        </svg>
      </span>
      <div>
        <p class="brand-kicker">Zhangyu Diecast Collection</p>
        <h1 class="brand-title brand-title-art">火柴盒 & 风火轮车模收藏</h1>
      </div>
    </div>

    <nav class="topbar-nav" aria-label="主导航">
      <a class="nav-link is-active" href="/index.html">首页</a>
      <a class="nav-link" href="#latest">最新收藏</a>
      <a class="nav-link" href="#browse">浏览收藏</a>
      <a class="nav-link" href="#collection">全部收藏</a>
      <a class="nav-link hero-entry-link" href="/login.html">模型录入</a>
    </nav>

    <div class="topbar-search">
      <input v-model.trim="searchQuery" class="input" type="search" placeholder="搜索车型 / 品牌 / 标签" />
    </div>
  </header>

  <main class="layout">
    <section class="hero-banner">
      <div class="hero-carousel">
        <div class="hero-carousel-window" role="button" tabindex="0" @click="handleRandomOpen" @keydown.enter.prevent="handleRandomOpen">
          <img
            v-if="randomCurrent?.imageUrl"
            class="hero-carousel-image"
            :src="randomCurrent.imageUrl"
            :alt="randomCurrent.name || '车模图片'"
            loading="lazy"
          />
          <div v-else class="hero-carousel-empty">{{ randomEmptyLabel }}</div>
          <button class="hero-carousel-nav hero-carousel-nav-prev" type="button" aria-label="上一张" @click.stop="prevRandom">
            ‹
          </button>
          <button class="hero-carousel-nav hero-carousel-nav-next" type="button" aria-label="下一张" @click.stop="nextRandom">
            ›
          </button>
        </div>

        <div class="hero-carousel-indicators">
          <button
            v-for="(_item, index) in randomModels"
            :key="`hero-dot-${index}`"
            type="button"
            class="hero-carousel-dot"
            :class="{ active: index === randomIndex }"
            :aria-label="`切换到第 ${index + 1} 张`"
            @click="goToRandom(index)"
          ></button>
        </div>

        <div class="hero-actions">
          <a class="btn-primary hero-action-btn" href="#collection">浏览全部收藏</a>
        </div>
      </div>
    </section>

    <section class="stats-bar" aria-label="收藏统计">
      <article class="stat-chip">
        <small>Total Cars</small>
        <strong>{{ totalCount }}</strong>
      </article>
      <article class="stat-chip">
        <small>Hot Wheels</small>
        <strong>{{ hotWheelsCount }}</strong>
      </article>
      <article class="stat-chip">
        <small>Matchbox</small>
        <strong>{{ matchboxCount }}</strong>
      </article>
      <article class="stat-chip">
        <small>Rare Models</small>
        <strong>{{ rareCount }}</strong>
      </article>
    </section>

    <section id="latest" class="section-shell">
      <div class="section-heading-line">
        <span></span>
        <h3 class="section-title">最新收藏</h3>
        <span></span>
      </div>

      <div v-if="latestModels.length > 0" class="latest-grid">
        <article
          v-for="item in latestModels"
          :key="item.id || item.name"
          class="latest-card"
          role="button"
          tabindex="0"
          @click="openDetailModal(item)"
          @keydown.enter.prevent="openDetailModal(item)"
        >
          <div class="latest-cover">
            <img v-if="item.imageUrl" :src="item.imageUrl" :alt="item.name || '车模图片'" loading="lazy" />
            <div v-else class="cover-placeholder">No Image</div>
          </div>
          <p class="latest-name">{{ item.name || "未命名车型" }}</p>
          <p class="latest-date">{{ formatDate(item.createdAt) }} 入库</p>
        </article>
      </div>
      <div v-else class="state-card">暂无最新收藏数据。</div>
    </section>

    <section id="browse" class="section-shell">
      <div class="section-heading-line">
        <span></span>
        <h3 class="section-title">浏览收藏</h3>
        <span></span>
      </div>

      <div class="browse-tiles">
        <article class="browse-tile" :style="tileBackground(hotWheelsHero?.imageUrl)">
          <h4>Hot Wheels</h4>
          <p>{{ hotWheelsCount }} Models</p>
        </article>
        <article class="browse-tile" :style="tileBackground(matchboxHero?.imageUrl)">
          <h4>Matchbox</h4>
          <p>{{ matchboxCount }} Models</p>
        </article>
        <article class="browse-tile">
          <h4>热门系列</h4>
          <p>{{ topTagsLine }}</p>
        </article>
        <article class="browse-tile browse-tile-year">
          <h4>换年份筛选</h4>
          <ul>
            <li v-for="bucket in yearBuckets" :key="bucket.label">{{ bucket.label }} · {{ bucket.count }}</li>
          </ul>
        </article>
      </div>
    </section>

    <section id="collection" class="section-shell">
      <div class="section-heading-line">
        <span></span>
        <h3 class="section-title">全部收藏</h3>
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

  <footer class="site-footer">
    <p class="footer-brand">Zhangyu Diecast Collection</p>
    <p>收藏年份：{{ timelineStartYear }} | 当前收藏：{{ totalCount }}</p>
    <div class="footer-icons" aria-hidden="true">
      <span>f</span>
      <span>▶</span>
      <span>✉</span>
    </div>
    <a href="https://beian.miit.gov.cn" target="_blank" rel="noopener noreferrer">沪ICP备2022001311号-1</a>
  </footer>

  <div class="detail-modal" :class="{ hidden: !detailVisible }" :aria-hidden="(!detailVisible).toString()">
    <div class="detail-modal-backdrop" @click="closeDetailModal"></div>
    <div class="detail-modal-panel" role="dialog" aria-modal="true" aria-labelledby="detail-modal-name">
      <button class="detail-modal-close" type="button" aria-label="关闭详情" @click="closeDetailModal">×</button>
      <div v-if="detailVisible && !activeDetailItem" class="state-card">未找到该车模，可能已被删除。</div>
      <ModelDetailCard v-if="detailVisible && activeDetailItem" :item="activeDetailItem" title-id="detail-modal-name" />
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import ModelDetailCard from "../components/ModelDetailCard.vue";
import ModelGridCard from "../components/ModelGridCard.vue";
import { useRandomShowcase } from "../composables/useRandomShowcase.js";
import { fetchModels } from "../js/api.js";
import {
  filterModels,
  getBrandList,
  sortByLatest,
} from "../utils/model.js";

const allModels = ref([]);
const loading = ref(true);
const errorMessage = ref("");
const searchQuery = ref("");
const brandFilter = ref("all");

const detailVisible = ref(false);
const activeDetailItem = ref(null);

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
const filteredModels = computed(() =>
  filterModels(allModels.value, {
    query: searchQuery.value,
    brand: brandFilter.value,
  }),
);
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
    return /rare|super treasure hunt|treasure hunt|sth|限量|稀有/.test(`${tags} ${text}`);
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

const collectionYearRange = computed(() => {
  const years = allModels.value
    .map((item) => Number.parseInt(String(item.year || "").trim(), 10))
    .filter((value) => Number.isInteger(value) && value > 0)
    .sort((a, b) => a - b);
  if (years.length === 0) {
    return "-";
  }
  return years[0] === years[years.length - 1] ? String(years[0]) : `${years[0]}-${years[years.length - 1]}`;
});

const parsedYears = computed(() =>
  allModels.value
    .map((item) => Number.parseInt(String(item.year || "").trim(), 10))
    .filter((value) => Number.isInteger(value) && value > 0),
);

const timelineStartYear = computed(() => {
  if (parsedYears.value.length === 0) {
    return 2018;
  }
  return Math.min(...parsedYears.value);
});

const yearBuckets = computed(() => {
  const buckets = [
    { label: "1968-1990", count: 0, match: (year) => year >= 1968 && year <= 1990 },
    { label: "1982-2000", count: 0, match: (year) => year >= 1982 && year <= 2000 },
    { label: "2000-2020", count: 0, match: (year) => year >= 2000 && year <= 2020 },
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
    backgroundImage: `linear-gradient(130deg, rgba(6, 9, 16, 0.66), rgba(9, 12, 22, 0.74)), url("${imageURL}")`,
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

function handleKeydown(event) {
  if (event.key === "Escape" && detailVisible.value) {
    closeDetailModal();
  }
}

watch(detailVisible, (visible) => {
  document.body.classList.toggle("modal-open", visible);
});

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
