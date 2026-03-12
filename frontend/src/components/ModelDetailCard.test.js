import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ModelDetailCard from "./ModelDetailCard.vue";

function createModel(overrides = {}) {
  return {
    id: 1,
    name: "Nissan GT-R",
    modelCode: "TM-01",
    brand: "Tomica",
    series: "Collectors",
    year: 2020,
    scale: "1:64",
    condition: "全新",
    color: "Silver",
    material: "Die-cast",
    createdAt: "2026-03-11T10:00:00.000Z",
    notes: "A note",
    imageUrl: "https://img/cover.jpg",
    gallery: ["https://img/detail-1.jpg", "https://img/cover.jpg"],
    tags: ["JDM"],
    ...overrides,
  };
}

describe("ModelDetailCard", () => {
  it("renders main detail information", () => {
    const wrapper = mount(ModelDetailCard, {
      props: {
        item: createModel(),
      },
    });

    expect(wrapper.find("h2").text()).toContain("Nissan GT-R");
    expect(wrapper.find(".model-code").text()).toContain("TM-01");
    expect(wrapper.find(".sub").text()).toContain("Tomica");
    expect(wrapper.findAll(".tag")).toHaveLength(1);
  });

  it("switches active image when thumbnail is clicked", async () => {
    const wrapper = mount(ModelDetailCard, {
      props: {
        item: createModel(),
      },
    });

    const thumbs = wrapper.findAll(".thumb-btn");
    expect(thumbs).toHaveLength(2);
    expect(wrapper.find(".detail-main img").attributes("src")).toBe("https://img/cover.jpg");

    await thumbs[1].trigger("click");
    expect(wrapper.find(".detail-main img").attributes("src")).toBe("https://img/detail-1.jpg");
  });

  it("resets active image index when model id changes", async () => {
    const wrapper = mount(ModelDetailCard, {
      props: {
        item: createModel(),
      },
    });

    await wrapper.findAll(".thumb-btn")[1].trigger("click");
    expect(wrapper.find(".detail-main img").attributes("src")).toBe("https://img/detail-1.jpg");

    await wrapper.setProps({
      item: createModel({
        id: 2,
        imageUrl: "https://img/new-cover.jpg",
        gallery: ["https://img/new-detail.jpg"],
      }),
    });

    expect(wrapper.find(".detail-main img").attributes("src")).toBe("https://img/new-cover.jpg");
  });

  it("shows placeholder when no image is available", () => {
    const wrapper = mount(ModelDetailCard, {
      props: {
        item: createModel({
          imageUrl: "",
          gallery: [],
        }),
      },
    });

    expect(wrapper.find(".detail-main img").exists()).toBe(false);
    expect(wrapper.find(".cover-placeholder").text()).toContain("No Image");
  });
});
