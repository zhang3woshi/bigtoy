const DEFAULT_API_BASE = import.meta.env.DEV ? "http://localhost:8080" : window.location.origin;
const API_BASE = (import.meta.env.VITE_API_BASE || DEFAULT_API_BASE).replace(/\/$/, "");
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

async function parseJSONResponse(response) {
  const payload = await response.json().catch(() => ({}));

  if (!response.ok) {
    const message = payload?.error || `Request failed with status ${response.status}`;
    throw new Error(message);
  }

  return payload;
}

export async function fetchModels() {
  const response = await fetch(`${API_BASE}/api/models`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  });
  const payload = await parseJSONResponse(response);
  return payload.data || [];
}

export async function createModel(input) {
  const isFormData = typeof FormData !== "undefined" && input instanceof FormData;
  const headers = {
    Accept: "application/json",
  };

  let body = input;
  if (!isFormData) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(input);
  }

  const response = await fetch(`${API_BASE}/api/models`, {
    method: "POST",
    credentials: "include",
    headers,
    body,
  });

  const payload = await parseJSONResponse(response);
  return payload.data;
}

export async function updateModel(id, input) {
  const modelID = String(id || "").trim();
  if (!UUID_PATTERN.test(modelID)) {
    throw new Error("invalid model id");
  }

  const isFormData = typeof FormData !== "undefined" && input instanceof FormData;
  const headers = {
    Accept: "application/json",
  };

  let body = input;
  if (!isFormData) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(input);
  }

  const response = await fetch(`${API_BASE}/api/models/${modelID}`, {
    method: "PUT",
    credentials: "include",
    headers,
    body,
  });

  const payload = await parseJSONResponse(response);
  return payload.data;
}

export async function deleteModel(id) {
  const modelID = String(id || "").trim();
  if (!UUID_PATTERN.test(modelID)) {
    throw new Error("invalid model id");
  }

  const response = await fetch(`${API_BASE}/api/models/${modelID}`, {
    method: "DELETE",
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  });

  const payload = await parseJSONResponse(response);
  return payload.data;
}

export async function login(input) {
  const response = await fetch(`${API_BASE}/api/auth/login`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify(input),
  });

  const payload = await parseJSONResponse(response);
  return payload.data;
}

export async function logout() {
  const response = await fetch(`${API_BASE}/api/auth/logout`, {
    method: "POST",
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  });

  const payload = await parseJSONResponse(response);
  return payload.data;
}

export async function fetchAuthState() {
  const response = await fetch(`${API_BASE}/api/auth/me`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  });

  const payload = await parseJSONResponse(response);
  return payload.data || { authenticated: false };
}
