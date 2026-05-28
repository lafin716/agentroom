<script setup lang="ts">
import {
  DropdownMenuRoot,
  DropdownMenuTrigger,
  DropdownMenuPortal,
  DropdownMenuContent,
  DropdownMenuItem,
} from "reka-ui";

defineProps<{
  label: string;
  mainPath: string;
  workspacePath?: string;
  hasWorkspace: boolean;
  busy: boolean;
}>();

const emit = defineEmits<{ (e: "open", path: string): void }>();
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <button class="btn btn-secondary" :disabled="busy">
        {{ label }}
        <span class="ml-1 text-zinc-400">▾</span>
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :side-offset="4"
        align="start"
        class="z-50 min-w-[180px] rounded-md border border-zinc-700 bg-zinc-900 p-1 text-sm shadow-lg focus:outline-none"
      >
        <DropdownMenuItem
          class="cursor-pointer rounded px-2 py-1.5 text-zinc-200 outline-none data-[highlighted]:bg-zinc-800"
          @select="emit('open', mainPath)"
        >
          메인 프로젝트
        </DropdownMenuItem>
        <DropdownMenuItem
          v-if="hasWorkspace && workspacePath"
          class="cursor-pointer rounded px-2 py-1.5 text-zinc-200 outline-none data-[highlighted]:bg-zinc-800"
          @select="emit('open', workspacePath)"
        >
          워크스페이스 복사본
        </DropdownMenuItem>
        <DropdownMenuItem
          v-else
          disabled
          class="cursor-not-allowed rounded px-2 py-1.5 text-zinc-500 opacity-50 outline-none"
        >
          워크스페이스 복사본 (없음)
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
