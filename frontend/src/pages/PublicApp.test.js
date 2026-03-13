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

  it("does not open detail modal when clicking browse-all link in hero", async () => {
    fetchModels.mockResolvedValueOnce([
      {
        id: "model-1",
        name: "Supra",
        brand: "MatchBox",
        imageUrl: "/uploads/model-1/cover.jpg",
        createdAt: "2026-03-12T00:00:00.000Z",
      },
    ]);

    const wrapper = mount(PublicApp);
    await flushPromises();

    const browseAllLink = wrapper.find(".showcase-hero-btn");
    expect(browseAllLink.exists()).toBe(true);

    await browseAllLink.trigger("click");
    await flushPromises();

    const modal = wrapper.find(".detail-modal");
    expect(modal.classes("hidden")).toBe(true);
  });

  it("sorts collection cards by selected sort rule", async () => {
    fetchModels.mockResolvedValueOnce([
      {
        id: "1",
        name: "Car Alpha",
        modelCode: "MB25",
        brand: "MatchBox",
        createdAt: "2026-03-10T00:00:00.000Z",
      },
      {
        id: "2",
        name: "Car Beta",
        modelCode: "MB1039",
        brand: "MatchBox",
        createdAt: "2026-03-12T00:00:00.000Z",
      },
      {
        id: "3",
        name: "Car Gamma",
        modelCode: "MB3",
        brand: "Hot Wheels",
        createdAt: "2026-03-11T00:00:00.000Z",
      },
    ]);

    const wrapper = mount(PublicApp);
    await flushPromises();

    const namesBefore = wrapper.findAll(".card-grid .model-card h3").map((node) => node.text());
    expect(namesBefore).toEqual(["Car Beta", "Car Gamma", "Car Alpha"]);

    const sortSelect = wrapper.find("select[aria-label='排序规则']");
    expect(sortSelect.exists()).toBe(true);
    await sortSelect.setValue("modelCode-asc");
    await flushPromises();

    const namesAfter = wrapper.findAll(".card-grid .model-card h3").map((node) => node.text());
    expect(namesAfter).toEqual(["Car Gamma", "Car Alpha", "Car Beta"]);
  });
});
