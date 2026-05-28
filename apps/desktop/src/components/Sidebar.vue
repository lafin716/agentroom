<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useFoldersStore } from "../stores/folders";

const folders = useFoldersStore();
const route = useRoute();
const router = useRouter();

const activeId = computed(() => route.params.id as string | undefined);

async function onAdd() {
  const entry = await folders.add();
  if (entry) router.push(`/folders/${entry.id}`);
}

async function onRemove(id: string, e: MouseEvent) {
  e.stopPropagation();
  if (!confirm("이 폴더를 목록에서 제거할까요? (메인 프로젝트 파일은 건드리지 않습니다.)"))
    return;
  await folders.remove(id);
  if (activeId.value === id) {
    if (folders.folders.length > 0) router.replace(`/folders/${folders.folders[0].id}`);
    else router.replace("/");
  }
}
</script>

<template>
  <aside class="flex h-full flex-col bg-zinc-900">
    <header class="border-b border-zinc-800 px-4 py-3">
      <h1 class="text-sm font-semibold text-zinc-200">agentroom</h1>
      <p class="mt-0.5 text-[11px] text-zinc-500">민감 정보를 격리한 에이전트 작업실</p>
    </header>
    <div class="flex-1 overflow-y-auto py-2">
      <p v-if="folders.folders.length === 0" class="px-4 py-6 text-xs text-zinc-500">
        등록된 폴더가 없습니다. 아래 버튼으로 추가하세요.
      </p>
      <ul>
        <li v-for="f in folders.folders" :key="f.id">
          <button
            class="group flex w-full items-start gap-2 px-4 py-2 text-left text-sm hover:bg-zinc-800"
            :class="{ 'bg-zinc-800 text-white': activeId === f.id, 'text-zinc-300': activeId !== f.id }"
            @click="router.push(`/folders/${f.id}`)"
          >
            <span class="flex-1 truncate">
              <span class="block font-medium">{{ f.name }}</span>
              <span class="block truncate text-[11px] text-zinc-500">{{ f.path }}</span>
            </span>
            <span
              role="button"
              class="invisible rounded px-1 text-zinc-500 hover:bg-zinc-700 hover:text-zinc-200 group-hover:visible"
              @click="onRemove(f.id, $event)"
              title="목록에서 제거"
            >×</span>
          </button>
        </li>
      </ul>
    </div>
    <footer class="border-t border-zinc-800 p-3">
      <button class="btn btn-primary w-full" @click="onAdd">+ 폴더 추가</button>
    </footer>
  </aside>
</template>
