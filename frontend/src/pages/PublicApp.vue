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
        <p class="brand-kicker">Collector's Catalog</p>
        <h1 class="brand-title brand-title-art">zhang3woshi的车库</h1>
      </div>
    </div>
    <div class="topbar-actions">
      <a class="nav-link hero-entry-link" href="/login.html">模型录入</a>
      <div class="hero-stat topbar-stat">
        <span>{{ totalCount }}</span>
        <small>收藏总数</small>
      </div>
    </div>
  </header>

  <main class="layout">
    <section class="hero">
      <div class="hero-left">
        <div class="hero-content">
          <section class="hero-panel hero-copy-panel" aria-label="页面介绍">
            <div class="hero-copy">
              <h2>把你的火柴盒与风火轮收藏做成在线车模库</h2>
              <p>按品牌、系列、年份快速浏览，把收藏内容清晰展示在一个页面里。</p>
            </div>
          </section>

          <section class="hero-panel hero-search-panel" aria-label="搜索筛选">
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

          <section class="hero-board hero-panel hero-board-panel" aria-label="收藏看板">
            <p class="hero-board-title">收藏看板</p>
            <div class="hero-board-grid">
              <article class="hero-board-item">
                <small>品牌数</small>
                <strong>{{ brandCount }}</strong>
              </article>
              <article class="hero-board-item">
                <small>标签数</small>
                <strong>{{ tagCount }}</strong>
              </article>
              <article class="hero-board-item hero-board-item-wide">
                <small>最近更新</small>
                <strong>{{ latestModelName }}</strong>
                <span class="hero-board-meta">{{ latestModelMeta }}</span>
              </article>
            </div>
          </section>
        </div>
      </div>

      <div class="hero-random">
        <div class="hero-random-head">
          <span class="hero-random-badge">热门</span>
          <span class="hero-random-line"></span>
        </div>
        <div class="hero-random-window">
          <button class="hero-random-nav hero-random-nav-prev" type="button" aria-label="上一条" @click="prevRandom">
            ‹
          </button>
          <div class="hero-random-slide" @click="handleRandomOpen">
            <img
              v-if="randomCurrent?.imageUrl"
              :src="randomCurrent.imageUrl"
              :alt="randomCurrent.name || '车模图片'"
              loading="lazy"
            />
            <div v-else class="hero-random-empty">
              {{ randomCurrent ? "No Image" : randomEmptyLabel }}
            </div>
          </div>
          <button class="hero-random-nav hero-random-nav-next" type="button" aria-label="下一条" @click="nextRandom">
            ›
          </button>
          <div class="hero-random-overlay">
            <p class="hero-random-caption">{{ randomCaption }}</p>
            <div class="hero-random-dots">
              <button
                v-for="(_item, index) in randomModels"
                :key="`random-dot-${index}`"
                type="button"
                class="hero-random-dot"
                :class="{ active: index === randomIndex }"
                :aria-label="`切换到第 ${index + 1} 条`"
                @click="goToRandom(index)"
              ></button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section>
      <div v-if="loading" class="state-card">正在加载车模数据...</div>
      <div v-if="errorMessage" class="state-card state-error">{{ errorMessage }}</div>
      <div v-if="showEmpty" class="state-card">没有匹配结果，试试其他关键词。</div>
      <div v-if="filteredModels.length > 0" class="card-grid">
        <ModelGridCard v-for="item in filteredModels" :key="item.id || item.name" :item="item" @open="openDetailModal" />
      </div>
    </section>
  </main>

  <footer class="site-footer">
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
  countUniqueTags,
  filterModels,
  findLatestModel,
  formatRandomModel,
  getBrandList,
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
const brandCount = computed(() => brands.value.length);
const tagCount = computed(() => countUniqueTags(allModels.value));

const latestModel = computed(() => findLatestModel(allModels.value));
const latestModelName = computed(() => String(latestModel.value?.name || "-").trim() || "-");
const latestModelMeta = computed(() => {
  if (!latestModel.value) {
    return "暂无数据";
  }
  const brand = String(latestModel.value.brand || "Unknown").trim();
  const modelCode = String(latestModel.value.modelCode || "").trim();
  return modelCode ? `${brand} · 编号 ${modelCode}` : brand || "暂无数据";
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
const randomCaption = computed(() => formatRandomModel(randomCurrent.value, randomEmptyLabel.value));

function closeDetailModal() {
  detailVisible.value = false;
}

function openDetailModal(modelID) {
  const item = allModels.value.find((entry) => Number(entry.id) === Number(modelID)) || null;
  activeDetailItem.value = item;
  detailVisible.value = true;
}

function handleRandomOpen() {
  const modelID = Number(randomCurrent.value?.id);
  if (!Number.isInteger(modelID) || modelID <= 0) {
    return;
  }
  openDetailModal(modelID);
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
