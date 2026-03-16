import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  createModel,
  deleteModel,
  exportBackup,
  fetchAuthState,
  fetchModels,
  importBackup,
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
  const modelID = "4f2f38b2-292d-4da1-b90d-adf346910280";
  const nextModelID = "6f0a6808-4e20-4ed0-89eb-a50db02de818";

  beforeEach(() => {
    globalThis.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches model list from backend", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: [{ id: modelID, name: "R34" }] }));

    const result = await fetchModels();

    expect(result).toEqual([{ id: modelID, name: "R34" }]);
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
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: modelID } }, 201));

    const payload = { name: "Supra", year: 2001 };
    const result = await createModel(payload);

    expect(result).toEqual({ id: modelID });
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
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: modelID } }, 201));

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
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { id: modelID, name: "Updated" } }));
    const result = await updateModel(modelID, { name: "Updated" });
    expect(result).toEqual({ id: modelID, name: "Updated" });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(new RegExp(`/api/models/${modelID}$`)),
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
    await expect(deleteModel(nextModelID)).resolves.toEqual({ deleted: true });
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(new RegExp(`/api/models/${nextModelID}$`)),
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

  it("exports backup and resolves filename from response header", async () => {
    globalThis.fetch.mockResolvedValueOnce(
      new Response("zip-content", {
        status: 200,
        headers: {
          "Content-Type": "application/zip",
          "Content-Disposition": 'attachment; filename="backup_test.zip"',
        },
      }),
    );

    const result = await exportBackup();

    expect(result.fileName).toBe("backup_test.zip");
    expect(result.blob).toEqual(
      expect.objectContaining({
        size: expect.any(Number),
        type: "application/zip",
      }),
    );
    expect(result.blob.size).toBeGreaterThan(0);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/backup\/export$/),
      expect.objectContaining({
        credentials: "include",
      }),
    );
  });

  it("imports backup file with multipart request", async () => {
    globalThis.fetch.mockResolvedValueOnce(jsonResponse({ data: { restored: true, restartRecommended: true } }));

    const file = new File(["zip-content"], "backup.zip", { type: "application/zip" });
    const result = await importBackup(file);

    expect(result).toEqual({ restored: true, restartRecommended: true });
    const [, options] = globalThis.fetch.mock.calls[0];
    expect(options).toEqual(
      expect.objectContaining({
        method: "POST",
      }),
    );
    expect(options.body).toBeInstanceOf(FormData);
    expect(options.body.get("file")).toBe(file);
  });

  it("rejects backup import when file is missing", async () => {
    await expect(importBackup(null)).rejects.toThrow("backup file is required");
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});
