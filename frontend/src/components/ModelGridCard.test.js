import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import ModelGridCard from "./ModelGridCard.vue";

describe("ModelGridCard", () => {
  const modelID = "4f2f38b2-292d-4da1-b90d-adf346910280";

  it("renders model information and tags", () => {
    const wrapper = mount(ModelGridCard, {
      props: {
        item: {
          id: modelID,
          name: "Toyota Supra",
          modelCode: "MB-777",
          brand: "MatchBox",
          series: "Premium",
          year: 1998,
          scale: "1:64",
          condition: "全新",
          notes: "Special edition",
          tags: ["JDM", "Classic"],
        },
      },
    });

    expect(wrapper.find("h3").text()).toContain("Toyota Supra");
    expect(wrapper.find(".model-code").text()).toContain("MB-777");
    expect(wrapper.findAll(".tag")).toHaveLength(2);
    expect(wrapper.find("a").attributes("href")).toBe(`/model.html?id=${modelID}`);
  });

  it("emits open event for valid id", async () => {
    const wrapper = mount(ModelGridCard, {
      props: {
        item: {
          id: modelID,
          name: "Skyline",
        },
      },
    });

    await wrapper.find("a").trigger("click", {
      preventDefault: vi.fn(),
    });
    expect(wrapper.emitted("open")).toEqual([[modelID]]);
  });

  it("does not emit open event when id is invalid", () => {
    const wrapper = mount(ModelGridCard, {
      props: {
        item: {
          id: "",
          name: "No ID",
        },
      },
    });

    wrapper.find("a").trigger("click", {
      preventDefault: vi.fn(),
    });
    expect(wrapper.emitted("open")).toBeUndefined();
  });
});
