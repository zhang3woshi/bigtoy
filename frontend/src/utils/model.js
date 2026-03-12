export function formatTime(value) {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }

  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function collectImages(item) {
  const seen = new Set();
  const collected = [];

  for (const value of [item?.imageUrl, ...(item?.gallery || [])]) {
    const url = String(value || "").trim();
    if (!url || seen.has(url)) {
      continue;
    }
    seen.add(url);
    collected.push(url);
  }

  return collected;
}

export function getModelCodeLabel(value) {
  const modelCode = String(value || "").trim();
  return modelCode || "未填写";
}

export function buildModelDetailHref(item) {
  if (!item?.id) {
    return "/model.html";
  }
  const query = new URLSearchParams({ id: String(item.id) });
  return `/model.html?${query.toString()}`;
}

export function sortByLatest(items) {
  return [...items].sort((a, b) => {
    const aTime = new Date(a.createdAt).getTime() || 0;
    const bTime = new Date(b.createdAt).getTime() || 0;
    if (aTime !== bTime) {
      return bTime - aTime;
    }
    return Number(b.id || 0) - Number(a.id || 0);
  });
}

export function findLatestModel(items) {
  if (!Array.isArray(items) || items.length === 0) {
    return null;
  }

  return items.reduce((latest, current) => {
    if (!latest) {
      return current;
    }

    const latestTime = new Date(latest.createdAt).getTime() || 0;
    const currentTime = new Date(current.createdAt).getTime() || 0;
    if (currentTime > latestTime) {
      return current;
    }
    if (currentTime < latestTime) {
      return latest;
    }
    return Number(current.id || 0) > Number(latest.id || 0) ? current : latest;
  }, null);
}

export function getBrandList(items) {
  const values = (items || [])
    .map((item) => String(item?.brand || "").trim())
    .filter(Boolean);
  return [...new Set(values)].sort((a, b) => a.localeCompare(b, "zh-CN"));
}

export function countUniqueTags(items) {
  const tagSet = new Set(
    (items || [])
      .flatMap((item) => (Array.isArray(item?.tags) ? item.tags : []))
      .map((tag) => String(tag || "").trim())
      .filter(Boolean),
  );
  return tagSet.size;
}

export function filterModels(items, { query = "", brand = "all" } = {}) {
  const normalizedQuery = String(query || "").trim().toLowerCase();
  const normalizedBrand = String(brand || "all");

  return (items || []).filter((item) => {
    const haystack = [
      item?.name,
      item?.modelCode,
      item?.series,
      item?.notes,
      ...(item?.tags || []),
      ...(item?.gallery || []),
    ]
      .join(" ")
      .toLowerCase();

    const searchMatched = !normalizedQuery || haystack.includes(normalizedQuery);
    const brandMatched = normalizedBrand === "all" || item?.brand === normalizedBrand;
    return searchMatched && brandMatched;
  });
}

export function formatRandomModel(item, fallback = "暂无车模") {
  if (!item) {
    return fallback;
  }
  const name = String(item.name || "未命名车型").trim();
  const brand = String(item.brand || "Unknown").trim();
  const modelCode = String(item.modelCode || "").trim();
  return modelCode ? `${name} · ${brand} · 编号 ${modelCode}` : `${name} · ${brand}`;
}

export function shuffleItems(items) {
  const cloned = [...(items || [])];
  for (let i = cloned.length - 1; i > 0; i -= 1) {
    const randomIndex = Math.floor(Math.random() * (i + 1));
    [cloned[i], cloned[randomIndex]] = [cloned[randomIndex], cloned[i]];
  }
  return cloned;
}
