<template>
  <header class="topbar">
    <div class="brand-block">
      <p class="brand-kicker">Secure Access</p>
      <h1 class="brand-title">Admin Authentication</h1>
    </div>
    <a class="nav-link" href="/index.html">Back to Gallery</a>
  </header>

  <main class="auth-shell">
    <section class="auth-card">
      <h2>Sign In</h2>
      <p class="panel-hint">Authenticate before entering model management.</p>

      <form class="model-form" @submit.prevent="handleSubmit">
        <label>
          Username
          <input v-model.trim="username" name="username" class="input" autocomplete="username" required />
        </label>
        <label>
          Password
          <input
            v-model="password"
            name="password"
            type="password"
            class="input"
            autocomplete="current-password"
            required
          />
        </label>
        <button type="submit" class="btn-primary" :disabled="pending">Sign In</button>
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
    setStatus(`Failed to verify auth state: ${error.message}`, "error");
  }
  return true;
}

async function handleSubmit() {
  if (!username.value || !password.value) {
    setStatus("Username and password are required.", "error");
    return;
  }

  pending.value = true;
  setStatus("Signing in...", "info");

  try {
    await login({
      username: username.value,
      password: password.value,
    });
    window.location.replace("/admin.html");
  } catch (error) {
    setStatus(`Sign-in failed: ${error.message}`, "error");
  } finally {
    pending.value = false;
  }
}

onMounted(() => {
  ensureGuest();
});
</script>
