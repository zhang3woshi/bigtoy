<template>
  <header class="topbar">
    <div class="brand-block">
      <p class="brand-kicker">Zhangyu Diecast Collection</p>
      <h1 class="brand-title">车模详情</h1>
    </div>
    <div class="topbar-nav">
      <a class="nav-link" href="/index.html">展厅首页</a>
      <a class="nav-link" href="/login.html">模型录入</a>
    </div>
  </header>

  <main class="layout detail-layout">
    <div v-if="loading" class="state-card">正在加载详情...</div>
    <div v-if="errorMessage" class="state-card state-error">{{ errorMessage }}</div>
    <div v-if="showEmpty" class="state-card">未找到该车模，可能已被删除。</div>
    <ModelDetailCard v-if="showCard" :item="model" />
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import ModelDetailCard from "../components/ModelDetailCard.vue";
import { fetchModels } from "../js/api.js";

const loading = ref(true);
const errorMessage = ref("");
const model = ref(null);

const showEmpty = computed(() => !loading.value && !errorMessage.value && !model.value);
const showCard = computed(() => !loading.value && !errorMessage.value && !!model.value);

onMounted(async () => {
  const modelID = String(new URLSearchParams(window.location.search).get("id") || "").trim();
  if (!modelID) {
    errorMessage.value = "详情链接缺少有效的车型 ID。";
    loading.value = false;
    return;
  }

  try {
    const models = await fetchModels();
    model.value = models.find((entry) => String(entry?.id || "").trim() === modelID) || null;
    if (model.value) {
      document.title = `${model.value.name || "车模详情"} | BigToy Garage`;
    }
  } catch (error) {
    errorMessage.value = `加载详情失败：${error.message}`;
  } finally {
    loading.value = false;
  }
});
</script>
