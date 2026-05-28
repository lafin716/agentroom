<script setup lang="ts">
import { onMounted } from "vue";
import { useRouter } from "vue-router";
import Sidebar from "./components/Sidebar.vue";
import { useFoldersStore } from "./stores/folders";

const folders = useFoldersStore();
const router = useRouter();

onMounted(async () => {
  await folders.load();
  if (folders.folders.length > 0) {
    router.replace(`/folders/${folders.folders[0].id}`);
  }
});
</script>

<template>
  <div class="flex h-full">
    <Sidebar class="w-64 shrink-0 border-r border-zinc-800" />
    <main class="flex-1 overflow-hidden">
      <router-view />
    </main>
  </div>
</template>
