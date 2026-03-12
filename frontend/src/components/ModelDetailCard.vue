<template>
  <article class="detail-card">
    <section class="detail-gallery">
      <div class="detail-main">
        <img v-if="activeImage" :src="activeImage" :alt="detailName" />
        <div v-else class="cover-placeholder">No Image</div>
      </div>
      <div v-if="imageList.length > 1" class="detail-thumbs">
        <button
          v-for="(url, index) in imageList"
          :key="`${detailKey}-thumb-${index}`"
          type="button"
          class="thumb-btn"
          :class="{ active: index === activeImageIndex }"
          :aria-label="`查看第 ${index + 1} 张图片`"
          @click="activeImageIndex = index"
        >
          <img :src="url" :alt="`缩略图 ${index + 1}`" loading="lazy" />
        </button>
      </div>
    </section>

    <section class="detail-info">
      <h2 :id="titleId || undefined">{{ detailName }}</h2>
      <p class="model-code">编号 {{ modelCodeLabel }}</p>
      <p class="sub">{{ item?.brand || "Unknown" }} · {{ item?.series || "未分类" }}</p>
      <p class="meta">
        年份 {{ item?.year || "-" }} · 比例 {{ item?.scale || "-" }} · 品相 {{ item?.condition || "-" }}
      </p>

      <dl class="detail-grid">
        <dt>颜色</dt>
        <dd>{{ item?.color || "-" }}</dd>
        <dt>材质</dt>
        <dd>{{ item?.material || "-" }}</dd>
        <dt>创建时间</dt>
        <dd>{{ formatTime(item?.createdAt) }}</dd>
      </dl>

      <div v-if="Array.isArray(item?.tags) && item.tags.length > 0" class="tags">
        <span v-for="tag in item.tags" :key="`${detailKey}-tag-${tag}`" class="tag">{{ tag }}</span>
      </div>
      <p class="note">{{ item?.notes || "暂无备注" }}</p>
    </section>
  </article>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { collectImages, formatTime, getModelCodeLabel } from "../utils/model.js";

const props = defineProps({
  item: {
    type: Object,
    required: true,
  },
  titleId: {
    type: String,
    default: "",
  },
});

const activeImageIndex = ref(0);

const imageList = computed(() => collectImages(props.item));
const activeImage = computed(() => imageList.value[activeImageIndex.value] || "");
const detailName = computed(() => String(props.item?.name || "未命名车型").trim() || "未命名车型");
const modelCodeLabel = computed(() => getModelCodeLabel(props.item?.modelCode));
const detailKey = computed(() => props.item?.id || props.item?.name || "detail");

watch(
  () => props.item?.id,
  () => {
    activeImageIndex.value = 0;
  },
);
</script>
