import { computed, ref } from "vue";
import { shuffleItems } from "../utils/model.js";

export function useRandomShowcase(
  sourceRef,
  {
    count = 5,
    slideInterval = 4200,
    refreshInterval = 30000,
  } = {},
) {
  const randomModels = ref([]);
  const randomIndex = ref(0);

  let randomSlideTimer = null;
  let randomRefreshTimer = null;

  const randomCurrent = computed(() => randomModels.value[randomIndex.value] || null);

  function setRandomSlide(index) {
    if (randomModels.value.length === 0) {
      randomIndex.value = 0;
      return;
    }
    const max = randomModels.value.length;
    randomIndex.value = ((index % max) + max) % max;
  }

  function renderRandomShowcase() {
    const source = sourceRef.value || [];
    if (source.length === 0) {
      randomModels.value = [];
      randomIndex.value = 0;
      return;
    }

    const selected = shuffleItems(source).slice(0, Math.min(count, source.length));
    if (selected.length === 0) {
      randomModels.value = [];
      randomIndex.value = 0;
      return;
    }

    randomModels.value = Array.from({ length: count }, (_item, index) => selected[index % selected.length]);
    randomIndex.value = 0;
  }

  function stopRandomShowcase() {
    if (randomSlideTimer) {
      window.clearInterval(randomSlideTimer);
      randomSlideTimer = null;
    }
    if (randomRefreshTimer) {
      window.clearInterval(randomRefreshTimer);
      randomRefreshTimer = null;
    }
  }

  function restartRandomSlideTimer() {
    if (randomSlideTimer) {
      window.clearInterval(randomSlideTimer);
      randomSlideTimer = null;
    }
    if (randomModels.value.length > 1) {
      randomSlideTimer = window.setInterval(() => {
        setRandomSlide(randomIndex.value + 1);
      }, slideInterval);
    }
  }

  function startRandomShowcase() {
    stopRandomShowcase();
    renderRandomShowcase();
    restartRandomSlideTimer();

    if ((sourceRef.value || []).length > count) {
      randomRefreshTimer = window.setInterval(renderRandomShowcase, refreshInterval);
    }
  }

  function prevRandom() {
    if (randomModels.value.length === 0) {
      return;
    }
    setRandomSlide(randomIndex.value - 1);
    restartRandomSlideTimer();
  }

  function nextRandom() {
    if (randomModels.value.length === 0) {
      return;
    }
    setRandomSlide(randomIndex.value + 1);
    restartRandomSlideTimer();
  }

  function goToRandom(index) {
    setRandomSlide(index);
    restartRandomSlideTimer();
  }

  return {
    randomModels,
    randomIndex,
    randomCurrent,
    startRandomShowcase,
    stopRandomShowcase,
    prevRandom,
    nextRandom,
    goToRandom,
  };
}
