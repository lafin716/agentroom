<script setup lang="ts">
import { computed, ref } from "vue";
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from "reka-ui";
import type { StatusResult } from "../types/ar";

const props = defineProps<{ status: StatusResult }>();
const emit = defineEmits<{
  (e: "apply", opts: { force: boolean; withDelete: boolean }): void;
  (e: "cancel"): void;
}>();

const force = ref(false);
const withDelete = ref(true);

const changes = computed(() => props.status.workspace_changes);
const conflicts = computed(() => props.status.conflicts);
const hasConflicts = computed(() => conflicts.value.length > 0);
const canApply = computed(() => !hasConflicts.value || force.value);

function opLabel(op: string): string {
  switch (op) {
    case "added":
      return "추가";
    case "modified":
      return "수정";
    case "deleted":
      return "삭제";
    default:
      return op;
  }
}

function opColor(op: string): string {
  switch (op) {
    case "added":
      return "text-green-400";
    case "modified":
      return "text-yellow-400";
    case "deleted":
      return "text-red-400";
    default:
      return "text-zinc-400";
  }
}
</script>

<template>
  <DialogRoot :open="true" @update:open="(v) => !v && emit('cancel')">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 bg-black/60" />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-50 w-[min(640px,90vw)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-zinc-700 bg-zinc-900 shadow-xl focus:outline-none"
      >
        <header class="border-b border-zinc-800 px-5 py-3">
          <DialogTitle class="text-base font-semibold text-zinc-100">Sync 적용</DialogTitle>
          <DialogDescription class="mt-1 text-xs text-zinc-400">
            워크스페이스의 변경을 메인 프로젝트로 반영합니다.
          </DialogDescription>
        </header>

        <div class="max-h-[60vh] overflow-y-auto px-5 py-3 text-sm">
          <section>
            <h4 class="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500">
              변경 ({{ changes.length }})
            </h4>
            <ul v-if="changes.length > 0" class="space-y-0.5 font-mono text-xs">
              <li v-for="c in changes" :key="c.path" class="flex gap-2">
                <span :class="opColor(c.op)" class="w-12 shrink-0">{{ opLabel(c.op) }}</span>
                <span class="break-all text-zinc-200">{{ c.path }}</span>
              </li>
            </ul>
            <p v-else class="text-xs text-zinc-500">변경 없음</p>
          </section>

          <section v-if="hasConflicts" class="mt-4">
            <h4 class="mb-2 text-xs font-semibold uppercase tracking-wider text-red-400">
              충돌 ({{ conflicts.length }})
            </h4>
            <p class="mb-2 text-xs text-red-300">
              메인과 워크스페이스에서 동시에 수정된 파일입니다. Force 옵션을 켜야 적용됩니다.
            </p>
            <ul class="space-y-0.5 font-mono text-xs text-red-300">
              <li v-for="p in conflicts" :key="p">! {{ p }}</li>
            </ul>
          </section>

          <section class="mt-4 space-y-2 border-t border-zinc-800 pt-3">
            <label class="flex items-start gap-2 text-xs text-zinc-300">
              <input v-model="withDelete" type="checkbox" class="mt-0.5" />
              <span>
                삭제도 반영
                <span class="block text-[11px] text-zinc-500">
                  워크스페이스에서 지워진 파일을 메인에서도 삭제합니다.
                </span>
              </span>
            </label>
            <label class="flex items-start gap-2 text-xs" :class="hasConflicts ? 'text-red-300' : 'text-zinc-300'">
              <input v-model="force" type="checkbox" class="mt-0.5" />
              <span>
                Force (충돌 무시)
                <span class="block text-[11px] text-zinc-500">
                  메인의 현재 내용을 워크스페이스 내용으로 덮어씁니다.
                </span>
              </span>
            </label>
          </section>
        </div>

        <footer class="flex justify-end gap-2 border-t border-zinc-800 px-5 py-3">
          <button class="btn btn-secondary" @click="emit('cancel')">취소</button>
          <button
            class="btn btn-primary"
            :disabled="!canApply || changes.length === 0"
            @click="emit('apply', { force, withDelete })"
          >
            Apply
          </button>
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
