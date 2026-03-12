import { describe, expect, it } from "vitest";
import {
  buildModelDetailHref,
  collectImages,
  countUniqueTags,
  filterModels,
  findLatestModel,
  formatRandomModel,
  getBrandList,
  getModelCodeLabel,
} from "./model.js";

describe("model utils", () => {
  it("deduplicates and orders images from cover + gallery", () => {
    const images = collectImages({
      imageUrl: "https://img/a.jpg",
      gallery: ["https://img/b.jpg", "https://img/a.jpg", "", null],
    });
    expect(images).toEqual(["https://img/a.jpg", "https://img/b.jpg"]);
  });

  it("returns display label for model code", () => {
    expect(getModelCodeLabel(" MB-1 ")).toBe("MB-1");
    expect(getModelCodeLabel("")).toBe("未填写");
  });

  it("builds detail href with model id", () => {
    expect(buildModelDetailHref({ id: 12 })).toBe("/model.html?id=12");
    expect(buildModelDetailHref({})).toBe("/model.html");
  });

  it("finds latest model using createdAt then id", () => {
    const latest = findLatestModel([
      { id: 3, createdAt: "2026-03-10T10:00:00Z" },
      { id: 5, createdAt: "2026-03-10T10:00:00Z" },
      { id: 4, createdAt: "2026-03-09T10:00:00Z" },
    ]);
    expect(latest).toEqual({ id: 5, createdAt: "2026-03-10T10:00:00Z" });
  });

  it("lists unique brands in locale sort order", () => {
    const brands = getBrandList([
      { brand: "Tomica" },
      { brand: "Hot Wheels" },
      { brand: "Tomica" },
      { brand: "" },
    ]);
    expect(brands).toEqual(["Hot Wheels", "Tomica"]);
  });

  it("counts unique tags", () => {
    const count = countUniqueTags([
      { tags: ["JDM", "Classic"] },
      { tags: ["Classic", "Race"] },
      {},
    ]);
    expect(count).toBe(3);
  });

  it("filters models by query and brand", () => {
    const filtered = filterModels(
      [
        { name: "Supra", modelCode: "MB12", brand: "MatchBox", tags: ["JDM"] },
        { name: "Civic", modelCode: "HW99", brand: "Hot Wheels", tags: ["Track"] },
      ],
      { query: "jdm", brand: "MatchBox" },
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0].name).toBe("Supra");
  });

  it("formats random model text with fallback", () => {
    expect(formatRandomModel(null, "加载中...")).toBe("加载中...");
    expect(formatRandomModel({ name: "R34", brand: "Tomica", modelCode: "TM-7" })).toBe("R34 · Tomica · 编号 TM-7");
  });
});
