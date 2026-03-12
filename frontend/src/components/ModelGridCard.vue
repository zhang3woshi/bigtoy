<template>
  <a class="card-link" :href="detailHref" @click.prevent="handleClick">
    <article class="model-card">
      <div class="cover">
        <img v-if="item.imageUrl" :src="item.imageUrl" :alt="item.name || '车模图片'" loading="lazy" />
        <div v-else class="cover-placeholder">No Image</div>
      </div>
      <div class="card-body">
        <h3>{{ item.name || "未命名车型" }}</h3>
        <p class="model-code">编号 {{ modelCodeLabel }}</p>
        <p class="sub">{{ item.brand || "Unknown" }} · {{ item.series || "未分类" }}</p>
        <p class="meta">
          年份 {{ item.year || "-" }} · 比例 {{ item.scale || "-" }} · 品相 {{ item.condition || "-" }}
        </p>
        <div v-if="Array.isArray(item.tags) && item.tags.length > 0" class="tags">
          <span v-for="tag in item.tags" :key="`${item.id || item.name}-${tag}`" class="tag">{{ tag }}</span>
        </div>
        <p class="note">{{ item.notes || "暂无备注" }}</p>
      </div>
    </article>
  </a>
</template>

<script setup>
import { computed } from "vue";
import { buildModelDetailHref, getModelCodeLabel } from "../utils/model.js";

const props = defineProps({
  item: {
    type: Object,
    required: true,
  },
});

const emit = defineEmits(["open"]);

const detailHref = computed(() => buildModelDetailHref(props.item));
const modelCodeLabel = computed(() => getModelCodeLabel(props.item?.modelCode));

function handleClick(event) {
  void event;
  const modelID = String(props.item?.id || "").trim();
  if (!modelID) {
    return;
  }
  emit("open", modelID);
}
</script>
