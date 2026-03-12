import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import PublicApp from "./PublicApp.vue";

vi.mock("../js/api.js", () => {
  return {
    fetchModels: vi.fn(),
  };
});

import { fetchModels } from "../js/api.js";

describe("PublicApp", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("opens detail modal when clicking a model card", async () => {
    fetchModels.mockResolvedValueOnce([
      {
        id: "model-1",
        name: "Supra",
        brand: "MatchBox",
        createdAt: "2026-03-12T00:00:00.000Z",
      },
    ]);

    const wrapper = mount(PublicApp);
    await flushPromises();

    const card = wrapper.find(".card-link");
    expect(card.exists()).toBe(true);

    await card.trigger("click");
    await flushPromises();

    const modal = wrapper.find(".detail-modal");
    expect(modal.classes("hidden")).toBe(false);
    expect(wrapper.text()).toContain("Supra");
  });
});
