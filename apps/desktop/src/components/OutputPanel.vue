<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useOutputStore } from "../stores/output";

const output = useOutputStore();
const scroller = ref<HTMLDivElement | null>(null);

const lines = computed(() => output.lines);

watch(
  () => lines.value.length,
  async () => {
    await nextTick();
    if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight;
  },
);

function fmt(at: number) {
  const d = new Date(at);
  return d.toLocaleTimeString();
}
</script>

<template>
  <div class="flex min-h-0 flex-col border-t border-zinc-800 bg-zinc-950">
    <header class="flex items-center justify-between px-6 py-2">
      <h3 class="text-xs font-semibold uppercase tracking-wider text-zinc-500">Output</h3>
      <button
        class="text-[11px] text-zinc-500 hover:text-zinc-300"
        @click="output.clear()"
        :disabled="lines.length === 0"
      >
        Clear
      </button>
    </header>
    <div
      ref="scroller"
      class="flex-1 overflow-y-auto px-6 pb-3 font-mono text-xs leading-relaxed"
    >
      <p v-if="lines.length === 0" class="text-zinc-600">아직 실행된 명령이 없습니다.</p>
      <div
        v-for="(l, idx) in lines"
        :key="idx"
        :class="l.level === 'error' ? 'text-red-300' : 'text-zinc-300'"
      >
        <span class="mr-2 text-zinc-600">{{ fmt(l.at) }}</span>
        <span class="whitespace-pre-wrap break-all">{{ l.text }}</span>
      </div>
    </div>
  </div>
</template>
