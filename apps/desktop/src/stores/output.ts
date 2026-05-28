import { defineStore } from "pinia";
import { ref } from "vue";

export interface OutputLine {
  at: number;
  level: "info" | "error";
  text: string;
}

export const useOutputStore = defineStore("output", () => {
  const lines = ref<OutputLine[]>([]);

  function push(level: OutputLine["level"], text: string) {
    lines.value.push({ at: Date.now(), level, text });
    if (lines.value.length > 500) lines.value.splice(0, lines.value.length - 500);
  }

  function info(text: string) {
    push("info", text);
  }

  function error(text: string) {
    push("error", text);
  }

  function clear() {
    lines.value = [];
  }

  return { lines, info, error, clear };
});
