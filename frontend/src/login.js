import { fetchAuthState, login } from "./js/api.js";

const formEl = document.getElementById("login-form");
const statusEl = document.getElementById("login-status");
const submitBtn = document.getElementById("login-btn");

function setStatus(message, kind = "info") {
  statusEl.textContent = message;
  statusEl.className = `form-status form-status-${kind}`;
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

formEl.addEventListener("submit", async (event) => {
  event.preventDefault();

  const payload = {
    username: String(new FormData(formEl).get("username") || "").trim(),
    password: String(new FormData(formEl).get("password") || ""),
  };

  if (!payload.username || !payload.password) {
    setStatus("Username and password are required.", "error");
    return;
  }

  submitBtn.disabled = true;
  setStatus("Signing in...", "info");

  try {
    await login(payload);
    window.location.replace("/admin.html");
  } catch (error) {
    setStatus(`Sign-in failed: ${error.message}`, "error");
  } finally {
    submitBtn.disabled = false;
  }
});

ensureGuest();
