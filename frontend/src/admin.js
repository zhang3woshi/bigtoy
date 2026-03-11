import { createModel, deleteModel, fetchAuthState, fetchModels, logout, updateModel } from "./js/api.js";

const formEl = document.getElementById("model-form");
const formTitleEl = document.getElementById("form-title");
const formHintEl = document.getElementById("form-hint");
const statusEl = document.getElementById("form-status");
const recentEl = document.getElementById("recent-list");
const submitBtn = document.getElementById("submit-btn");
const cancelEditBtn = document.getElementById("cancel-edit-btn");
const logoutBtn = document.getElementById("logout-btn");

let currentModels = [];
let editingModelID = null;

function escapeHTML(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function setStatus(message, kind = "info") {
  statusEl.textContent = message;
  statusEl.className = `form-status form-status-${kind}`;
}

function redirectToLogin() {
  window.location.replace("/login.html");
}

function setCreateMode() {
  editingModelID = null;
  formTitleEl.textContent = "新增车型";
  formHintEl.textContent = "保存后会立即出现在对外展示页。";
  submitBtn.textContent = "保存车型";
  cancelEditBtn.classList.add("hidden");
}

function setEditMode(item) {
  editingModelID = item.id;
  formTitleEl.textContent = `编辑车型 #${item.id}`;
  formHintEl.textContent = "可修改文本信息；若重新上传图片文件，会覆盖该模型原有图片。";
  submitBtn.textContent = "保存修改";
  cancelEditBtn.classList.remove("hidden");
}

function clearFileInputs() {
  const coverInput = formEl.elements.namedItem("imageFile");
  const galleryInput = formEl.elements.namedItem("galleryFiles");
  if (coverInput) {
    coverInput.value = "";
  }
  if (galleryInput) {
    galleryInput.value = "";
  }
}

function fillFormForEdit(item) {
  const normalizedBrand = String(item.brand || "").trim().toLowerCase() === "matchbox" ? "MatchBox" : item.brand || "";
  formEl.elements.namedItem("name").value = item.name || "";
  formEl.elements.namedItem("modelCode").value = item.modelCode || "";
  formEl.elements.namedItem("brand").value = normalizedBrand;
  formEl.elements.namedItem("series").value = item.series || "";
  formEl.elements.namedItem("scale").value = item.scale || "";
  formEl.elements.namedItem("year").value = Number.isFinite(item.year) ? String(item.year) : "";
  formEl.elements.namedItem("color").value = item.color || "";
  formEl.elements.namedItem("condition").value = item.condition || "";
  formEl.elements.namedItem("material").value = item.material || "";
  formEl.elements.namedItem("tags").value = Array.isArray(item.tags) ? item.tags.join(", ") : "";
  formEl.elements.namedItem("notes").value = item.notes || "";

  clearFileInputs();
  setEditMode(item);
  setStatus(`正在编辑：${item.name}`, "info");

  formEl.scrollIntoView({
    behavior: "smooth",
    block: "start",
  });
}

function resetFormAndMode() {
  formEl.reset();
  clearFileInputs();
  setCreateMode();
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

function buildPayloadFormData(form) {
  const formData = new FormData(form);

  const normalizedTags = String(formData.get("tags") || "")
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean)
    .join(",");
  formData.set("tags", normalizedTags);

  const yearRaw = String(formData.get("year") || "").trim();
  if (!yearRaw) {
    formData.set("year", "0");
  } else {
    const parsedYear = Number.parseInt(yearRaw, 10);
    formData.set("year", Number.isNaN(parsedYear) || parsedYear < 0 ? "0" : String(parsedYear));
  }

  return formData;
}

function renderRecent(items) {
  if (items.length === 0) {
    recentEl.innerHTML = '<p class="muted">还没有任何车型，先录入一台吧。</p>';
    return;
  }

  recentEl.innerHTML = items
    .map((item) => {
      const galleryCount = Array.isArray(item.gallery) ? item.gallery.length : 0;
      return `
      <article class="recent-item">
        <div class="recent-row">
          <div class="recent-main">
            <strong>${escapeHTML(item.name)}</strong>
            <span>#${escapeHTML(item.id)} · ${escapeHTML(item.brand || "Unknown")} · 编号 ${escapeHTML(item.modelCode || "未填写")} · 附图 ${galleryCount}</span>
          </div>
          <div class="recent-actions">
            <button type="button" class="btn-inline" data-action="edit" data-id="${escapeHTML(item.id)}">编辑</button>
            <button type="button" class="btn-inline btn-inline-danger" data-action="delete" data-id="${escapeHTML(item.id)}">删除</button>
          </div>
        </div>
      </article>
    `;
    })
    .join("");
}

function findModelByID(id) {
  return currentModels.find((item) => Number(item.id) === Number(id)) || null;
}

async function refreshRecent() {
  try {
    const items = await fetchModels();
    const sorted = [...items].sort((a, b) => {
      const aTime = new Date(a.createdAt).getTime() || 0;
      const bTime = new Date(b.createdAt).getTime() || 0;
      if (aTime !== bTime) {
        return bTime - aTime;
      }
      return Number(b.id || 0) - Number(a.id || 0);
    });

    currentModels = sorted;
    renderRecent(sorted);
  } catch (error) {
    recentEl.innerHTML = `<p class="state-error">加载失败：${escapeHTML(error.message)}</p>`;
  }
}

async function handleDeleteModel(modelID) {
  const item = findModelByID(modelID);
  const label = item?.name || `ID ${modelID}`;
  const confirmed = window.confirm(`确认删除模型「${label}」吗？此操作不可撤销。`);
  if (!confirmed) {
    return;
  }

  try {
    await deleteModel(modelID);
    if (Number(editingModelID) === Number(modelID)) {
      resetFormAndMode();
    }
    setStatus("删除成功。", "success");
    await refreshRecent();
  } catch (error) {
    if (/authentication required/i.test(String(error.message || ""))) {
      redirectToLogin();
      return;
    }
    setStatus(`删除失败：${error.message}`, "error");
  }
}

recentEl.addEventListener("click", async (event) => {
  const trigger = event.target.closest("button[data-action][data-id]");
  if (!trigger) {
    return;
  }

  const action = trigger.dataset.action;
  const modelID = Number.parseInt(trigger.dataset.id || "", 10);
  if (!Number.isFinite(modelID) || modelID <= 0) {
    setStatus("无效的模型 ID", "error");
    return;
  }

  if (action === "edit") {
    const item = findModelByID(modelID);
    if (!item) {
      setStatus("未找到该模型，请刷新后重试。", "error");
      return;
    }
    fillFormForEdit(item);
    return;
  }

  if (action === "delete") {
    await handleDeleteModel(modelID);
  }
});

cancelEditBtn?.addEventListener("click", () => {
  resetFormAndMode();
  setStatus("已取消编辑。", "info");
});

formEl.addEventListener("submit", async (event) => {
  event.preventDefault();
  submitBtn.disabled = true;

  const modelName = String(new FormData(formEl).get("name") || "").trim();
  if (!modelName) {
    setStatus("车型名称为必填项", "error");
    submitBtn.disabled = false;
    return;
  }

  try {
    const payload = buildPayloadFormData(formEl);
    if (editingModelID) {
      await updateModel(editingModelID, payload);
      setStatus("修改成功。", "success");
    } else {
      await createModel(payload);
      setStatus("保存成功，展示页已可见。", "success");
    }

    resetFormAndMode();
    await refreshRecent();
  } catch (error) {
    if (/authentication required/i.test(String(error.message || ""))) {
      redirectToLogin();
      return;
    }
    setStatus(`保存失败：${error.message}`, "error");
  } finally {
    submitBtn.disabled = false;
  }
});

logoutBtn?.addEventListener("click", async () => {
  logoutBtn.disabled = true;
  try {
    await logout();
  } catch (_error) {
    // Ignore logout response, cookie may have already expired.
  } finally {
    redirectToLogin();
  }
});

async function bootstrap() {
  const authenticated = await ensureAuthenticated();
  if (!authenticated) {
    return;
  }
  setCreateMode();
  await refreshRecent();
}

bootstrap();
