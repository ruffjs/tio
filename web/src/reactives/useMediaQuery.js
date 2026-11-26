import { onBeforeUnmount, ref } from "vue";

export default function useMediaQuery(query) {
  const matches = ref(false);
  if (typeof window === "undefined" || !window.matchMedia) return matches;

  const media = window.matchMedia(query);
  matches.value = media.matches;

  const onChange = (event) => {
    matches.value = event.matches;
  };
  media.addEventListener("change", onChange);
  onBeforeUnmount(() => media.removeEventListener("change", onChange));

  return matches;
}
