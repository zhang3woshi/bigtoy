<template>
  <header class="topbar">
    <div class="brand-block">
      <p class="brand-kicker">Zhangyu Diecast Collection</p>
      <h1 class="brand-title">车模录入后台</h1>
    </div>
    <div class="topbar-nav">
      <a class="nav-link" href="/index.html">展厅首页</a>
      <button type="button" class="btn-secondary nav-btn" :disabled="logoutPending" @click="handleLogout">
        退出登录
      </button>
    </div>
  </header>

  <main class="layout admin-layout">
    <section class="panel">
      <h2>新增车型</h2>
      <p class="panel-hint">保存后会立即出现在对外展示页。</p>

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

        <label>
          主图文件
          <input
            ref="coverInputRef"
            name="imageFile"
            type="file"
            accept="image/*"
            class="input"
            @change="handleCoverFileChange"
          />
        </label>

        <label>
          更多图片（可多选）
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
          <button type="submit" class="btn-primary" :disabled="submitPending">保存车型</button>
        </div>
      </form>

      <p class="form-status" :class="statusClass" role="status" aria-live="polite">{{ statusMessage }}</p>
    </section>

    <section class="panel">
      <h2>数据备份</h2>
      <p class="panel-hint">导出内容为数据库文件 + 图片目录的 ZIP 包；导入 ZIP 后会覆盖云端当前数据。</p>

      <div class="backup-tools">
        <div class="backup-row">
          <button type="button" class="btn-inline" :disabled="backupPending" @click="handleExportBackup">
            下载当前备份
          </button>
          <button
            type="button"
            class="btn-primary"
            :disabled="backupPending || !backupFile"
            @click="handleImportBackup"
          >
            导入并覆盖
          </button>
        </div>

        <label class="backup-file-picker">
          选择备份文件（ZIP）
          <input
            ref="backupFileInputRef"
            type="file"
            accept="application/zip,.zip"
            class="input"
            :disabled="backupPending"
            @change="handleBackupFileChange"
          />
        </label>

        <p v-if="backupFileName" class="backup-file-name">已选择：{{ backupFileName }}</p>
      </div>

      <p class="form-status" :class="backupStatusClass" role="status" aria-live="polite">{{ backupStatusMessage }}</p>

      <h2>车型列表</h2>
      <p v-if="loadingRecent" class="muted">正在加载车型列表...</p>
      <p v-if="listError" class="state-error">{{ listError }}</p>

      <section class="recent-list-toolbar" aria-label="车型列表排序">
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
      </section>

      <div class="recent-list">
        <p v-if="!loadingRecent && !listError && visibleModels.length === 0" class="muted">
          还没有任何车型，先录入一台吧。
        </p>

        <article v-for="item in visibleModels" :key="item.id" class="recent-item">
          <div class="recent-row">
            <div class="recent-thumb">
              <img v-if="item.imageUrl" :src="item.imageUrl" :alt="`${item.name || '车型'} thumbnail`" loading="lazy" />
              <div v-else class="recent-thumb-placeholder">No Image</div>
            </div>
            <div class="recent-main">
              <strong>{{ item.name || "未命名车型" }}</strong>
              <span>
                #{{ item.id }} · {{ item.brand || "Unknown" }} · 编号 {{ modelCodeLabel(item.modelCode) }} · 附图
                {{ Array.isArray(item.gallery) ? item.gallery.length : 0 }}
              </span>
            </div>
            <div class="recent-actions">
              <button
                type="button"
                class="btn-inline"
                :disabled="submitPending || deletingModelID === item.id"
                @click="goToEdit(item)"
              >
                编辑
              </button>
              <button
                type="button"
                class="btn-inline btn-inline-danger"
                :disabled="submitPending || deletingModelID === item.id"
                @click="handleDelete(item)"
              >
                删除
              </button>
            </div>
          </div>
        </article>
      </div>

      <div v-if="showPagination" class="recent-pagination">
        <p class="recent-pagination-status">{{ pageStatus }}</p>
        <div class="recent-pagination-actions">
          <button type="button" class="btn-inline" :disabled="currentPage <= 1" @click="currentPage -= 1">上一页</button>
          <button
            type="button"
            class="btn-inline"
            :disabled="currentPage >= totalPages"
            @click="currentPage += 1"
          >
            下一页
          </button>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import { createModel, deleteModel, exportBackup, fetchAuthState, fetchModels, importBackup, logout } from "../js/api.js";
import { getModelCodeLabel } from "../utils/model.js";

const pageSize = 10;

const coverInputRef = ref(null);
const galleryInputRef = ref(null);
const backupFileInputRef = ref(null);

const currentModels = ref([]);
const currentPage = ref(1);
const imageFile = ref(null);
const galleryFiles = ref([]);
const backupFile = ref(null);
const sortField = ref("createdAt");
const sortDirection = ref("desc");

const loadingRecent = ref(false);
const listError = ref("");
const submitPending = ref(false);
const backupPending = ref(false);
const logoutPending = ref(false);
const deletingModelID = ref(null);

const statusMessage = ref("");
const statusKind = ref("info");
const backupStatusMessage = ref("");
const backupStatusKind = ref("info");

const form = reactive(createInitialForm());

const statusClass = computed(() => `form-status-${statusKind.value}`);
const backupStatusClass = computed(() => `form-status-${backupStatusKind.value}`);
const backupFileName = computed(() => String(backupFile.value?.name || "").trim());

const sortedModels = computed(() =>
  sortCollectionModels(currentModels.value, {
    field: sortField.value,
    direction: sortDirection.value,
  }),
);

const totalPages = computed(() => Math.max(1, Math.ceil(sortedModels.value.length / pageSize)));
const showPagination = computed(() => sortedModels.value.length > pageSize);

const visibleModels = computed(() => {
  const startIndex = (currentPage.value - 1) * pageSize;
  return sortedModels.value.slice(startIndex, startIndex + pageSize);
});

const pageStatus = computed(() => {
  const total = sortedModels.value.length;
  if (total === 0) {
    return "";
  }
  const start = (currentPage.value - 1) * pageSize + 1;
  const end = Math.min(currentPage.value * pageSize, total);
  return `第 ${currentPage.value} / ${totalPages.value} 页 · 显示 ${start}-${end} / ${total}`;
});
const sortDirectionLabel = computed(() => (sortDirection.value === "desc" ? "降序" : "升序"));
const sortDirectionIcon = computed(() => (sortDirection.value === "desc" ? "↓" : "↑"));
const sortDirectionAriaLabel = computed(() =>
  sortDirection.value === "desc" ? "当前降序，点击切换为升序" : "当前升序，点击切换为降序",
);

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

function modelCodeLabel(value) {
  return getModelCodeLabel(value);
}

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

function setStatus(message, kind = "info") {
  statusMessage.value = message;
  statusKind.value = kind;
}

function setBackupStatus(message, kind = "info") {
  backupStatusMessage.value = message;
  backupStatusKind.value = kind;
}

function redirectToLogin() {
  window.location.replace("/login.html");
}

function isAuthenticationError(error) {
  return /authentication required/i.test(String(error?.message || ""));
}

function clearFileInputs() {
  imageFile.value = null;
  galleryFiles.value = [];
  if (coverInputRef.value) {
    coverInputRef.value.value = "";
  }
  if (galleryInputRef.value) {
    galleryInputRef.value.value = "";
  }
}

function clearBackupInput() {
  backupFile.value = null;
  if (backupFileInputRef.value) {
    backupFileInputRef.value.value = "";
  }
}

function resetForm() {
  Object.assign(form, createInitialForm());
  clearFileInputs();
}

function normalizeBrand(value) {
  const brand = String(value || "").trim();
  if (brand.toLowerCase() === "matchbox") {
    return "MatchBox";
  }
  return brand;
}

function handleCoverFileChange(event) {
  const file = event.target.files?.[0] || null;
  imageFile.value = file;
}

function handleGalleryFilesChange(event) {
  galleryFiles.value = event.target.files ? Array.from(event.target.files) : [];
}

function handleBackupFileChange(event) {
  backupFile.value = event.target.files?.[0] || null;
  if (backupFile.value) {
    setBackupStatus(`已选择备份文件：${backupFile.value.name}`, "info");
  } else {
    setBackupStatus("");
  }
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

function goToEdit(item) {
  const modelID = String(item?.id || "").trim();
  if (!modelID) {
    setStatus("无效的车型 ID，无法进入编辑页。", "error");
    return;
  }

  const query = new URLSearchParams({ id: modelID });
  window.location.href = `/admin-edit.html?${query.toString()}`;
}

async function refreshRecent() {
  loadingRecent.value = true;
  listError.value = "";

  try {
    const items = await fetchModels();
    currentModels.value = Array.isArray(items) ? items : [];
    if (currentPage.value > totalPages.value) {
      currentPage.value = totalPages.value;
    }
    if (currentPage.value < 1) {
      currentPage.value = 1;
    }
  } catch (error) {
    listError.value = `加载失败：${error.message}`;
  } finally {
    loadingRecent.value = false;
  }
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

async function handleSubmit() {
  const modelName = String(form.name || "").trim();
  if (!modelName) {
    setStatus("车型名称为必填项。", "error");
    return;
  }

  submitPending.value = true;

  try {
    const payload = buildPayloadFormData();
    await createModel(payload);
    setStatus("保存成功，展示页已可见。", "success");

    resetForm();
    await refreshRecent();
  } catch (error) {
    if (isAuthenticationError(error)) {
      redirectToLogin();
      return;
    }
    setStatus(`保存失败：${error.message}`, "error");
  } finally {
    submitPending.value = false;
  }
}

async function handleDelete(item) {
  const modelID = String(item.id || "").trim();
  if (!modelID) {
    setStatus("无效的车型 ID。", "error");
    return;
  }

  const modelLabel = item.name || `ID ${modelID}`;
  const confirmed = window.confirm(`确认删除模型「${modelLabel}」吗？此操作不可撤销。`);
  if (!confirmed) {
    return;
  }

  deletingModelID.value = modelID;
  try {
    await deleteModel(modelID);
    setStatus("删除成功。", "success");
    await refreshRecent();
  } catch (error) {
    if (isAuthenticationError(error)) {
      redirectToLogin();
      return;
    }
    setStatus(`删除失败：${error.message}`, "error");
  } finally {
    deletingModelID.value = null;
  }
}

async function handleExportBackup() {
  backupPending.value = true;
  setBackupStatus("正在生成并下载 ZIP 备份，请稍候...", "info");

  try {
    const { blob, fileName } = await exportBackup();
    const objectURL = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = objectURL;
    anchor.download = fileName || "backup.zip";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(objectURL);

    setBackupStatus("备份下载成功。", "success");
  } catch (error) {
    if (isAuthenticationError(error)) {
      redirectToLogin();
      return;
    }
    setBackupStatus(`备份下载失败：${error.message}`, "error");
  } finally {
    backupPending.value = false;
  }
}

async function handleImportBackup() {
  if (!backupFile.value) {
    setBackupStatus("请先选择备份文件。", "error");
    return;
  }

  const confirmed = window.confirm("导入 ZIP 会覆盖当前云端车型与图片数据，确认继续吗？");
  if (!confirmed) {
    return;
  }

  backupPending.value = true;
  setBackupStatus("正在导入 ZIP 备份并还原数据，请稍候...", "info");

  try {
    const result = await importBackup(backupFile.value);
    const restored = Boolean(result?.restored);
    if (restored) {
      setBackupStatus("导入成功，数据已还原。建议重启服务后再次检查展示页。", "success");
    } else {
      setBackupStatus("导入完成。建议重启服务后再次检查展示页。", "success");
    }
    clearBackupInput();
    await refreshRecent();
  } catch (error) {
    if (isAuthenticationError(error)) {
      redirectToLogin();
      return;
    }
    setBackupStatus(`备份导入失败：${error.message}`, "error");
  } finally {
    backupPending.value = false;
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
  resetForm();
  clearBackupInput();
  await refreshRecent();
});
</script>
