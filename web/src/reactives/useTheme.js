import { computed, ref } from "vue";

const THEME_KEY = "$tiopg/theme";
const themeModes = ["auto", "light", "dark"];
const themeMode = ref("auto");
const effectiveTheme = ref("light");
let mediaQuery;
let mediaQueryBound = false;

const getSystemTheme = () => {
  if (typeof window === "undefined") return "light";
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
};

const normalizeThemeMode = (value) => (themeModes.includes(value) ? value : "auto");

const resolveTheme = (mode) => (mode === "auto" ? getSystemTheme() : mode);

const syncDocumentTheme = () => {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.themeMode = themeMode.value;
  document.documentElement.dataset.theme = effectiveTheme.value;
  document.documentElement.classList.toggle("dark", effectiveTheme.value === "dark");
};

const applyResolvedTheme = () => {
  effectiveTheme.value = resolveTheme(themeMode.value);
  syncDocumentTheme();
};

const bindSystemTheme = () => {
  if (mediaQueryBound || typeof window === "undefined") return;
  mediaQuery = window.matchMedia?.("(prefers-color-scheme: dark)");
  if (!mediaQuery) return;
  mediaQuery.addEventListener("change", () => {
    if (themeMode.value === "auto") {
      applyResolvedTheme();
    }
  });
  mediaQueryBound = true;
};

export const applyTheme = (value) => {
  themeMode.value = normalizeThemeMode(value);
  localStorage.setItem(THEME_KEY, themeMode.value);
  applyResolvedTheme();
};

export const initTheme = () => {
  bindSystemTheme();
  applyTheme(localStorage.getItem(THEME_KEY));
};

export default function useTheme() {
  const theme = effectiveTheme;
  const isDark = computed(() => effectiveTheme.value === "dark");
  const themeLabel = computed(() => {
    if (themeMode.value === "auto") return `Auto (${effectiveTheme.value === "dark" ? "Dark" : "Light"})`;
    return themeMode.value === "dark" ? "Dark" : "Light";
  });
  const nextThemeMode = computed(() => {
    const index = themeModes.indexOf(themeMode.value);
    return themeModes[(index + 1) % themeModes.length];
  });
  const toggleTheme = () => applyTheme(nextThemeMode.value);

  return {
    theme,
    themeMode,
    effectiveTheme,
    isDark,
    themeLabel,
    nextThemeMode,
    toggleTheme,
    applyTheme,
  };
}
