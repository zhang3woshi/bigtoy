const DEFAULT_SITE_FILING_URL = "https://beian.miit.gov.cn";
const TEMPLATE_PLACEHOLDER_PATTERN = /^\{\{.*\}\}$/;

function normalizeMetaContent(value) {
  const normalized = String(value || "").trim();
  if (!normalized || normalized === "<no value>" || TEMPLATE_PLACEHOLDER_PATTERN.test(normalized)) {
    return "";
  }
  return normalized;
}

function readMetaContent(name) {
  if (typeof document === "undefined") {
    return "";
  }

  const element = document.querySelector(`meta[name="${name}"]`);
  return normalizeMetaContent(element?.getAttribute("content"));
}

export function getPublicRuntimeConfig() {
  const siteFilingText = readMetaContent("bigtoy-site-filing-text");
  const siteFilingURL = readMetaContent("bigtoy-site-filing-url");

  return {
    siteFilingText,
    siteFilingURL: siteFilingText ? siteFilingURL || DEFAULT_SITE_FILING_URL : "",
  };
}
