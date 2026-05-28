<script setup lang="ts">
import { onMounted, ref } from "vue";
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from "reka-ui";
import { ar } from "../services/ar";
import { useOutputStore } from "../stores/output";
import type { HistoryEntry } from "../types/ar";

const props = defineProps<{ folderPath: string }>();
const emit = defineEmits<{ (e: "close"): void }>();

const output = useOutputStore();
const entries = ref<HistoryEntry[]>([]);
const loading = ref(false);
const undoing = ref(false);

async function load() {
  loading.value = true;
  try {
    const r = await ar.history(props.folderPath);
    entries.value = r.entries;
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    output.error(`✖ ar history: ${msg}`);
  } finally {
    loading.value = false;
  }
}

async function onUndoOne() {
  if (entries.value.length === 0) return;
  if (!confirm("가장 최근 sync 를 되돌립니다. 진행할까요?")) return;
  undoing.value = true;
  output.info("▶ ar undo");
  try {
    const r = await ar.undo(props.folderPath, 1);
    output.info(`✔ ar undo (남은 history: ${r.remaining})`);
    await load();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    output.error(`✖ ar undo: ${msg}`);
  } finally {
    undoing.value = false;
  }
}

function fmtAt(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

onMounted(load);
</script>

<template>
  <DialogRoot :open="true" @update:open="(v) => !v && emit('close')">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 bg-black/60" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 w-[min(640px,90vw)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-zinc-700 bg-zinc-900 shadow-xl focus:outline-none"
      >
        <header class="flex items-center justify-between border-b border-zinc-800 px-5 py-3">
          <DialogTitle class="text-base font-semibold text-zinc-100">Sync 히스토리</DialogTitle>
          <button
            class="btn btn-danger text-xs"
            :disabled="undoing || entries.length === 0"
            @click="onUndoOne"
          >
            가장 최근 되돌리기
          </button>
        </header>

        <div class="max-h-[60vh] overflow-y-auto px-5 py-3">
          <p v-if="loading" class="text-xs text-zinc-500">불러오는 중...</p>
          <p v-else-if="entries.length === 0" class="text-xs text-zinc-500">
            아직 적용된 sync 가 없습니다.
          </p>
          <ul v-else class="space-y-2">
            <li
              v-for="e in entries"
              :key="e.id"
              class="rounded border border-zinc-800 bg-zinc-950 px-3 py-2 text-xs"
            >
              <div class="flex items-baseline justify-between gap-3">
                <span class="font-mono text-zinc-400">#{{ e.seq }}</span>
                <span class="text-[11px] text-zinc-500">{{ fmtAt(e.applied_at) }}</span>
              </div>
              <p class="mt-1 text-zinc-200">{{ e.summary }}</p>
              <p class="mt-1 font-mono text-[10px] text-zinc-600">{{ e.id }}</p>
            </li>
          </ul>
        </div>

        <footer class="flex justify-end border-t border-zinc-800 px-5 py-3">
          <button class="btn btn-secondary" @click="emit('close')">닫기</button>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
