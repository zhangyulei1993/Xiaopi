import { computed, ref } from "vue";
import { API_ORIGIN } from "../api/client";
import { getPublicConfig } from "../api/properties";

export type PagePresentation = { title: string; subtitle: string; backgroundImage: string };

const fallback = (title: string, subtitle: string): PagePresentation => ({ title, subtitle, backgroundImage: "" });

export const usePagePresentation = (key: string, title: string, subtitle: string) => {
  const presentation = ref<PagePresentation>(fallback(title, subtitle));
  const loadPresentation = async () => {
    const config = await getPublicConfig().catch(() => null);
    const next = config?.pagePresentations?.[key];
    if (next) presentation.value = { ...presentation.value, ...next };
    if (presentation.value.title) uni.setNavigationBarTitle({ title: presentation.value.title });
    return config;
  };
  const pageStyle = computed(() => {
    const path = presentation.value.backgroundImage;
    if (!path) return {};
    const url = path.startsWith("http") ? path : `${API_ORIGIN.replace(/\/api$/, "")}${path}`;
    return { backgroundImage: `linear-gradient(rgba(247,246,241,.92),rgba(247,246,241,.97)), url(${url})`, backgroundSize: "cover", backgroundAttachment: "fixed" };
  });
  return { presentation, loadPresentation, pageStyle };
};
