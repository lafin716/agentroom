<script setup lang="ts">
import { ref } from "vue";
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from "reka-ui";

defineProps<{ workspacePath: string }>();
const emit = defineEmits<{
  (e: "apply", opts: { withHistory: boolean }): void;
  (e: "cancel"): void;
}>();

const withHistory = ref(false);
</script>

<template>
  <DialogRoot :open="true" @update:open="(v) => !v && emit('cancel')">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 bg-black/60" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 w-[min(560px,90vw)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-zinc-700 bg-zinc-900 shadow-xl focus:outline-none"
      >
        <header class="border-b border-zinc-800 px-5 py-3">
          <DialogTitle class="text-base font-semibold text-zinc-100">복사본 초기화</DialogTitle>
          <DialogDescription class="mt-1 text-xs text-zinc-400">
            워크스페이스 복사본과 baseline 을 삭제하고 다음 Copy 부터 새로 시작합니다.
          </DialogDescription>
        </header>

        <div class="px-5 py-3 text-sm">
          <p class="text-xs text-zinc-400">삭제할 대상</p>
          <ul class="mt-1 space-y-0.5 font-mono text-xs text-zinc-200">
            <li v-if="workspacePath" class="break-all">· {{ workspacePath }}</li>
            <li v-else class="text-zinc-500">· (워크스페이스 없음)</li>
            <li class="text-zinc-400">· .agent/baseline.json</li>
            <li v-if="withHistory" class="text-red-300">· .agent/history/ (sync 기록)</li>
          </ul>

          <p class="mt-3 rounded border border-red-900/40 bg-red-950/30 px-3 py-2 text-xs text-red-300">
            이 작업은 되돌릴 수 없습니다. 메인 프로젝트 파일은 건드리지 않습니다.
          </p>

          <label class="mt-4 flex items-start gap-2 border-t border-zinc-800 pt-3 text-xs text-zinc-300">
            <input v-model="withHistory" type="checkbox" class="mt-0.5" />
            <span>
              sync history 도 함께 삭제
              <span class="block text-[11px] text-zinc-500">
                .agent/history 가 사라져 과거 sync 의 undo 가 불가능해집니다.
              </span>
            </span>
          </label>
        </div>

        <footer class="flex justify-end gap-2 border-t border-zinc-800 px-5 py-3">
          <button class="btn btn-secondary" @click="emit('cancel')">취소</button>
          <button class="btn btn-danger" @click="emit('apply', { withHistory })">삭제</button>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
