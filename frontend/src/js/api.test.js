import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createModel,
  deleteModel,
  fetchAuthState,
  fetchModels,
  login,
  logout,
  updateModel,
} from "./api.js";

function jsonResponse(payload, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

describe("api client", () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches model list from backend", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: [{ id: 1, name: "R34" }] }));

    const result = await fetchModels();

    expect(result).toEqual([{ id: 1, name: "R34" }]);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/models$/),
      expect.objectContaining({
        credentials: "include",
        headers: expect.objectContaining({ Accept: "application/json" }),
      }),
    );
  });

  it("returns empty model list when payload has no data", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({}));
    await expect(fetchModels()).resolves.toEqual([]);
  });

  it("throws server error message from JSON payload", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ error: "boom" }, 400));
    await expect(fetchModels()).rejects.toThrow("boom");
  });

  it("throws fallback status message when error body is not JSON", async () => {
    globalThis.fetch.mockResolvedValueOnce(
      new Response("plain-text-error", {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    await expect(fetchModels()).rejects.toThrow("Request failed with status 502");
  });

  it("creates model with JSON payload", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: 2 } }, 201));

    const payload = { name: "Supra", year: 2001 };
    const result = await createModel(payload);

    expect(result).toEqual({ id: 2 });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/models$/),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(payload),
        headers: expect.objectContaining({
          Accept: "application/json",
          "Content-Type": "application/json",
        }),
      }),
    );
  });

  it("creates model with FormData payload", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: 3 } }, 201));

    const formData = new FormData();
    formData.append("name", "Skyline");
    await createModel(formData);

    const [, options] = globalThis.fetch.mock.calls[0];
    expect(options.body).toBe(formData);
    expect(options.headers).toEqual(
      expect.objectContaining({
        Accept: "application/json",
      }),
    );
    expect(options.headers).not.toHaveProperty("Content-Type");
  });

  it("rejects update for invalid model id", async () => {
    await expect(updateModel("bad-id", { name: "A" })).rejects.toThrow("invalid model id");
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("updates model for valid model id", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: 7, name: "Updated" } }));
    const result = await updateModel("7", { name: "Updated" });
    expect(result).toEqual({ id: 7, name: "Updated" });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/models\/7$/),
      expect.objectContaining({
        method: "PUT",
      }),
    );
  });

  it("rejects delete for invalid model id", async () => {
    await expect(deleteModel(0)).rejects.toThrow("invalid model id");
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("deletes model with valid id", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { deleted: true } }));
    await expect(deleteModel("11")).resolves.toEqual({ deleted: true });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/models\/11$/),
      expect.objectContaining({
        method: "DELETE",
      }),
    );
  });

  it("logs in and logs out", async () => {
    globalThis.fetch
      .mockResolvedValueOnce(jsonResponse({ data: { authenticated: true } }))
      .mockResolvedValueOnce(jsonResponse({ data: { authenticated: false } }));

    const authData = await login({ username: "admin", password: "secret" });
    expect(authData).toEqual({ authenticated: true });
    expect(globalThis.fetch.mock.calls[0][1]).toEqual(
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
        }),
      }),
    );

    const logoutData = await logout();
    expect(logoutData).toEqual({ authenticated: false });
    expect(globalThis.fetch.mock.calls[1][1]).toEqual(
      expect.objectContaining({
        method: "POST",
      }),
    );
  });

  it("returns fallback auth state when payload has no data", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({}));
    await expect(fetchAuthState()).resolves.toEqual({ authenticated: false });
  });
});
