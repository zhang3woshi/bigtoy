<template>
  <header class="topbar">
    <div class="brand-block">
      <p class="brand-kicker">zhang3woshi Diecast Collection</p>
      <h1 class="brand-title">管理后台登录</h1>
    </div>
    <div class="topbar-nav">
      <a class="nav-link" href="/index.html">展厅首页</a>
      <a class="nav-link is-active" href="/login.html">模型录入</a>
    </div>
  </header>

  <main class="auth-shell">
    <section class="auth-card">
      <h2>管理员登录</h2>
      <p class="panel-hint">登录后可进行车型录入、编辑与删除管理。</p>

      <form class="model-form" @submit.prevent="handleSubmit">
        <label>
          用户名
          <input v-model.trim="username" name="username" class="input" autocomplete="username" required />
        </label>
        <label>
          密码
          <input
            v-model="password"
            name="password"
            type="password"
            class="input"
            autocomplete="current-password"
            required
          />
        </label>
        <button type="submit" class="btn-primary" :disabled="pending">登录后台</button>
      </form>

      <p class="form-status" :class="statusClass" role="status" aria-live="polite">{{ statusMessage }}</p>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { fetchAuthState, login } from "../js/api.js";

const username = ref("");
const password = ref("");
const pending = ref(false);
const statusMessage = ref("");
const statusKind = ref("info");

const statusClass = computed(() => `form-status-${statusKind.value}`);

function setStatus(message, kind = "info") {
  statusMessage.value = message;
  statusKind.value = kind;
}

async function ensureGuest() {
  try {
    const authState = await fetchAuthState();
    if (authState?.authenticated) {
      window.location.replace("/admin.html");
      return false;
    }
  } catch (error) {
    setStatus(`认证状态检查失败：${error.message}`, "error");
  }
  return true;
}

async function handleSubmit() {
  if (!username.value || !password.value) {
    setStatus("请输入用户名和密码。", "error");
    return;
  }

  pending.value = true;
  setStatus("正在登录...", "info");

  try {
    await login({
      username: username.value,
      password: password.value,
    });
    window.location.replace("/admin.html");
  } catch (error) {
    setStatus(`登录失败：${error.message}`, "error");
  } finally {
    pending.value = false;
  }
}

onMounted(() => {
  ensureGuest();
});
</script>
