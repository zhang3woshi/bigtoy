import { ref } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useRandomShowcase } from "./useRandomShowcase.js";

describe("useRandomShowcase", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("normalizes selected models to configured count and supports index navigation", () => {
    const sourceRef = ref([{ id: 1 }, { id: 2 }]);
    const showcase = useRandomShowcase(sourceRef, {
      count: 5,
      slideInterval: 1000,
      refreshInterval: 5000,
    });

    showcase.startRandomShowcase();

    expect(showcase.randomModels.value).toHaveLength(5);
    expect(showcase.randomIndex.value).toBe(0);
    expect(showcase.randomCurrent.value).not.toBeNull();

    showcase.prevRandom();
    expect(showcase.randomIndex.value).toBe(4);

    showcase.nextRandom();
    expect(showcase.randomIndex.value).toBe(0);

    showcase.goToRandom(2);
    expect(showcase.randomIndex.value).toBe(2);

    showcase.goToRandom(-1);
    expect(showcase.randomIndex.value).toBe(4);
  });

  it("advances slide index by timer", () => {
    const sourceRef = ref([{ id: 1 }, { id: 2 }, { id: 3 }]);
    const showcase = useRandomShowcase(sourceRef, {
      count: 3,
      slideInterval: 1000,
      refreshInterval: 5000,
    });

    showcase.startRandomShowcase();
    expect(showcase.randomIndex.value).toBe(0);

    vi.advanceTimersByTime(1000);
    expect(showcase.randomIndex.value).toBe(1);

    vi.advanceTimersByTime(1000);
    expect(showcase.randomIndex.value).toBe(2);
  });

  it("keeps state stable when source list is empty", () => {
    const sourceRef = ref([]);
    const showcase = useRandomShowcase(sourceRef, {
      count: 3,
      slideInterval: 1000,
      refreshInterval: 5000,
    });

    showcase.startRandomShowcase();
    expect(showcase.randomModels.value).toEqual([]);
    expect(showcase.randomIndex.value).toBe(0);
    expect(showcase.randomCurrent.value).toBeNull();

    showcase.nextRandom();
    showcase.prevRandom();
    showcase.goToRandom(1);
    expect(showcase.randomIndex.value).toBe(0);
  });
});
