<script setup lang="ts">
import { useFoldersStore } from "../stores/folders";
import { useRouter } from "vue-router";

const folders = useFoldersStore();
const router = useRouter();

async function onAdd() {
  const entry = await folders.add();
  if (entry) router.push(`/folders/${entry.id}`);
}
</script>

<template>
  <div class="flex h-full items-center justify-center">
    <div class="max-w-md text-center">
      <h2 class="text-xl font-semibold text-zinc-200">시작하기</h2>
      <p class="mt-2 text-sm text-zinc-400">
        메인 프로젝트 폴더를 추가하면 <code class="text-accent">ar</code> 명령을 버튼으로 실행할 수 있습니다.
      </p>
      <button class="btn btn-primary mt-6" @click="onAdd">+ 폴더 추가</button>
    </div>
  </div>
</template>
