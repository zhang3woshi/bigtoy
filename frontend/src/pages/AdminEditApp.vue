<template>
  <header class="topbar">
    <div class="brand-block">
      <p class="brand-kicker">Zhangyu Diecast Collection</p>
      <h1 class="brand-title">车型编辑</h1>
    </div>
    <div class="topbar-nav">
      <a class="nav-link" href="/index.html">展厅首页</a>
      <a class="nav-link" href="/admin.html">新增车型</a>
      <button type="button" class="btn-secondary nav-btn" :disabled="logoutPending" @click="handleLogout">
        退出登录
      </button>
    </div>
  </header>

  <main class="layout">
    <section class="panel">
      <h2>编辑车型</h2>
      <p class="panel-hint">你可以在这里修改全部字段，包括主图和附图。若不上传新图片，将保留原图。</p>

      <p v-if="loadingModel" class="muted">正在加载车型数据...</p>
      <p v-if="loadError" class="state-error">{{ loadError }}</p>

      <div v-if="!loadingModel && !loadError">
        <p class="muted">车型 ID：{{ modelID }}</p>

        <form class="model-form" enctype="multipart/form-data" @submit.prevent="handleSubmit">
          <div class="grid-two">
            <label>
              车型名称 *
              <input
                v-model.trim="form.name"
                name="name"
                class="input"
                required
                placeholder="例如：Toyota Supra MK4"
              />
            </label>
            <label>
              编号
              <input
                v-model.trim="form.modelCode"
                name="modelCode"
                class="input"
                placeholder="例如：MB1177 / HW-202"
              />
            </label>
          </div>

          <div class="grid-two">
            <label>
              品牌
              <select v-model="form.brand" name="brand" class="input">
                <option value="">请选择</option>
                <option value="Hot Wheels">Hot Wheels</option>
                <option value="MatchBox">MatchBox</option>
                <option value="Tomica">Tomica</option>
                <option value="Mini GT">Mini GT</option>
              </select>
            </label>
            <label>
              系列
              <input v-model.trim="form.series" name="series" class="input" placeholder="例如：Premium / Collectors" />
            </label>
          </div>

          <div class="grid-two">
            <label>
              比例
              <input v-model.trim="form.scale" name="scale" class="input" placeholder="例如：1:64" />
            </label>
            <label>
              年份
              <input v-model.trim="form.year" name="year" type="number" min="0" class="input" placeholder="例如：1995" />
            </label>
          </div>

          <div class="grid-two">
            <label>
              颜色
              <input v-model.trim="form.color" name="color" class="input" placeholder="例如：Metallic Blue" />
            </label>
            <label>
              品相
              <input v-model.trim="form.condition" name="condition" class="input" placeholder="例如：Mint / Near Mint" />
            </label>
          </div>

          <label>
            材质
            <input v-model.trim="form.material" name="material" class="input" placeholder="例如：Die-cast" />
          </label>

          <div class="edit-image-section">
            <p class="edit-image-label">当前主图</p>
            <div class="edit-cover-preview">
              <img v-if="currentCoverUrl" :src="currentCoverUrl" :alt="`${form.name || '车型'} current cover`" loading="lazy" />
              <div v-else class="recent-thumb-placeholder">暂无主图</div>
            </div>
          </div>

          <label>
            更换主图文件
            <input
              ref="coverInputRef"
              name="imageFile"
              type="file"
              accept="image/*"
              class="input"
              @change="handleCoverFileChange"
            />
          </label>
          <p v-if="imageFile" class="muted">已选择新主图：{{ imageFile.name }}</p>

          <div class="edit-image-section">
            <p class="edit-image-label">当前附图（{{ currentGallery.length }}）</p>
            <div class="edit-gallery-grid">
              <div v-if="currentGallery.length === 0" class="edit-gallery-empty">暂无附图</div>
              <div v-for="(imageUrl, index) in currentGallery" :key="`${imageUrl}-${index}`" class="edit-gallery-item">
                <img :src="imageUrl" :alt="`${form.name || '车型'} gallery ${index + 1}`" loading="lazy" />
              </div>
            </div>
          </div>

          <label>
            更换附图（可多选）
            <input
              ref="galleryInputRef"
              name="galleryFiles"
              type="file"
              accept="image/*"
              multiple
              class="input"
              @change="handleGalleryFilesChange"
            />
          </label>
          <p v-if="galleryFiles.length > 0" class="muted">
            已选择 {{ galleryFiles.length }} 张新附图，保存后将替换当前附图。
          </p>

          <label>
            标签（逗号分隔）
            <input v-model.trim="form.tags" name="tags" class="input" placeholder="JDM, Rally, Classic" />
          </label>

          <label>
            备注
            <textarea
              v-model.trim="form.notes"
              name="notes"
              class="input textarea"
              rows="3"
              placeholder="可填写来源、批次、版本等信息"
            ></textarea>
          </label>

          <div class="form-actions">
            <button type="submit" class="btn-primary" :disabled="submitPending">
              {{ submitPending ? "保存中..." : "保存修改" }}
            </button>
            <button type="button" class="btn-secondary" :disabled="submitPending" @click="clearSelectedFiles">
              清空已选图片
            </button>
            <a class="btn-secondary btn-secondary-link" href="/admin.html">返回新增页</a>
          </div>
        </form>
      </div>

      <p class="form-status" :class="statusClass" role="status" aria-live="polite">{{ statusMessage }}</p>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import { fetchAuthState, fetchModels, logout, updateModel } from "../js/api.js";

const coverInputRef = ref(null);
const galleryInputRef = ref(null);

const modelID = ref("");
const currentCoverUrl = ref("");
const currentGallery = ref([]);

const imageFile = ref(null);
const galleryFiles = ref([]);

const loadingModel = ref(true);
const loadError = ref("");
const submitPending = ref(false);
const logoutPending = ref(false);

const statusMessage = ref("");
const statusKind = ref("info");

const form = reactive(createInitialForm());

const statusClass = computed(() => `form-status-${statusKind.value}`);

function createInitialForm() {
  return {
    name: "",
    modelCode: "",
    brand: "MatchBox",
    series: "",
    scale: "1:64",
    year: "",
    color: "",
    condition: "全新",
    material: "Die-cast",
    tags: "",
    notes: "",
  };
}

function normalizeBrand(value) {
  const brand = String(value || "").trim();
  if (brand.toLowerCase() === "matchbox") {
    return "MatchBox";
  }
  return brand;
}

function setStatus(message, kind = "info") {
  statusMessage.value = message;
  statusKind.value = kind;
}

function redirectToLogin() {
  window.location.replace("/login.html");
}

function clearSelectedFiles() {
  imageFile.value = null;
  galleryFiles.value = [];

  if (coverInputRef.value) {
    coverInputRef.value.value = "";
  }
  if (galleryInputRef.value) {
    galleryInputRef.value.value = "";
  }
}

function applyModel(item) {
  Object.assign(form, {
    name: item?.name || "",
    modelCode: item?.modelCode || "",
    brand: normalizeBrand(item?.brand),
    series: item?.series || "",
    scale: item?.scale || "",
    year: Number.isFinite(item?.year) ? String(item.year) : "",
    color: item?.color || "",
    condition: item?.condition || "",
    material: item?.material || "",
    tags: Array.isArray(item?.tags) ? item.tags.join(", ") : "",
    notes: item?.notes || "",
  });

  currentCoverUrl.value = String(item?.imageUrl || "").trim();
  currentGallery.value = Array.isArray(item?.gallery)
    ? item.gallery
        .map((value) => String(value || "").trim())
        .filter(Boolean)
    : [];
}

function handleCoverFileChange(event) {
  imageFile.value = event.target.files?.[0] || null;
}

function handleGalleryFilesChange(event) {
  galleryFiles.value = event.target.files ? Array.from(event.target.files) : [];
}

function buildPayloadFormData() {
  const payload = new FormData();
  payload.set("name", String(form.name || "").trim());
  payload.set("modelCode", String(form.modelCode || "").trim());
  payload.set("brand", normalizeBrand(form.brand));
  payload.set("series", String(form.series || "").trim());
  payload.set("scale", String(form.scale || "").trim());
  payload.set("color", String(form.color || "").trim());
  payload.set("condition", String(form.condition || "").trim());
  payload.set("material", String(form.material || "").trim());
  payload.set("notes", String(form.notes || "").trim());

  const normalizedTags = String(form.tags || "")
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean)
    .join(",");
  payload.set("tags", normalizedTags);

  const yearRaw = String(form.year || "").trim();
  if (!yearRaw) {
    payload.set("year", "0");
  } else {
    const parsedYear = Number.parseInt(yearRaw, 10);
    payload.set("year", Number.isNaN(parsedYear) || parsedYear < 0 ? "0" : String(parsedYear));
  }

  if (imageFile.value) {
    payload.append("imageFile", imageFile.value);
  }

  for (const file of galleryFiles.value) {
    payload.append("galleryFiles", file);
  }

  return payload;
}

async function ensureAuthenticated() {
  try {
    const authState = await fetchAuthState();
    if (!authState?.authenticated) {
      redirectToLogin();
      return false;
    }
    return true;
  } catch (error) {
    setStatus(`认证检查失败：${error.message}`, "error");
    return false;
  }
}

async function loadModelForEdit() {
  modelID.value = String(new URLSearchParams(window.location.search).get("id") || "").trim();
  if (!modelID.value) {
    loadError.value = "编辑链接缺少有效的车型 ID。";
    loadingModel.value = false;
    return;
  }

  try {
    const items = await fetchModels();
    const item = items.find((entry) => String(entry?.id || "").trim() === modelID.value);
    if (!item) {
      loadError.value = "未找到该车型，可能已被删除。";
      return;
    }

    applyModel(item);
    document.title = `编辑 ${item.name || "车型"} | BigToy Garage`;
  } catch (error) {
    if (/authentication required/i.test(String(error.message || ""))) {
      redirectToLogin();
      return;
    }
    loadError.value = `加载车型失败：${error.message}`;
  } finally {
    loadingModel.value = false;
  }
}

async function handleSubmit() {
  const modelName = String(form.name || "").trim();
  if (!modelID.value) {
    setStatus("无效的车型 ID。", "error");
    return;
  }
  if (!modelName) {
    setStatus("车型名称为必填项。", "error");
    return;
  }

  submitPending.value = true;

  try {
    const payload = buildPayloadFormData();
    const updatedItem = await updateModel(modelID.value, payload);
    applyModel(updatedItem);
    clearSelectedFiles();
    setStatus("修改成功，全部信息已更新。", "success");
  } catch (error) {
    if (/authentication required/i.test(String(error.message || ""))) {
      redirectToLogin();
      return;
    }
    setStatus(`保存失败：${error.message}`, "error");
  } finally {
    submitPending.value = false;
  }
}

async function handleLogout() {
  logoutPending.value = true;
  try {
    await logout();
  } catch (_error) {
    // Ignore response; auth cookie may have expired.
  } finally {
    redirectToLogin();
  }
}

onMounted(async () => {
  const authenticated = await ensureAuthenticated();
  if (!authenticated) {
    return;
  }

  clearSelectedFiles();
  await loadModelForEdit();
});
</script>
