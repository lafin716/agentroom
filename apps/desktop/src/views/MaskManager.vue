<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ar } from "../services/ar";
import { useFoldersStore } from "../stores/folders";
import { useOutputStore } from "../stores/output";
import type { Mask } from "../types/ar";

const route = useRoute();
const router = useRouter();
const folders = useFoldersStore();
const output = useOutputStore();

const folder = computed(() => folders.byId(route.params.id as string));

const masks = ref<Mask[]>([]);
const loading = ref(true);
const busy = ref(false);
const error = ref<string | null>(null);

const newFile = ref("");
const newPath = ref("");

async function load() {
  if (!folder.value) return;
  loading.value = true;
  error.value = null;
  try {
    const res = await ar.maskList(folder.value.path);
    masks.value = res.masks ?? [];
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    output.error(`✖ ar mask list: ${msg}`);
  } finally {
    loading.value = false;
  }
}

watch(() => route.params.id, load);
onMounted(load);

function back() {
  if (folder.value) router.push(`/folders/${folder.value.id}`);
  else router.push("/");
}

async function onAdd() {
  if (!folder.value) return;
  const file = newFile.value.trim();
  const path = newPath.value.trim();
  if (!file || !path) {
    error.value = "파일 경로와 마스크 경로를 모두 입력하세요.";
    return;
  }
  error.value = null;
  busy.value = true;
  output.info(`▶ ar mask add ${file} ${path}`);
  try {
    await ar.maskAdd(folder.value.path, file, [path]);
    output.info(`✔ ar mask add ${file} ${path}`);
    newPath.value = "";
    await load();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    output.error(`✖ ar mask add: ${msg}`);
  } finally {
    busy.value = false;
  }
}

async function onRemovePath(file: string, path: string) {
  if (!folder.value) return;
  busy.value = true;
  output.info(`▶ ar mask rm ${file} ${path}`);
  try {
    await ar.maskRm(folder.value.path, file, path);
    output.info(`✔ ar mask rm ${file} ${path}`);
    await load();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    output.error(`✖ ar mask rm: ${msg}`);
  } finally {
    busy.value = false;
  }
}

async function onRemoveFile(file: string) {
  if (!folder.value) return;
  if (!confirm(`${file} 의 모든 마스크를 제거합니다. 진행할까요?`)) return;
  busy.value = true;
  output.info(`▶ ar mask rm ${file}`);
  try {
    await ar.maskRm(folder.value.path, file);
    output.info(`✔ ar mask rm ${file}`);
    await load();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    output.error(`✖ ar mask rm: ${msg}`);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div v-if="folder" class="flex h-full flex-col">
    <header class="flex items-center justify-between border-b border-zinc-800 px-6 py-3">
      <div>
        <h2 class="text-sm font-semibold text-zinc-100">마스크 편집</h2>
        <p class="text-[11px] text-zinc-500">{{ folder.path }}</p>
      </div>
      <button class="btn btn-secondary" @click="back">뒤로</button>
    </header>

    <p class="border-b border-zinc-800 bg-zinc-900/60 px-6 py-2 text-[11px] text-zinc-400">
      YAML / JSON 파일 안의 특정 경로만 <code>***MASKED***</code> 로 가립니다.
      Sync 시 마스크된 값은 항상 메인의 원본으로 보존됩니다 (에이전트가 수정해도 무시).
      경로는 점 표기법 (예: <code>spring.datasource.password</code>, <code>users.0.email</code>).
    </p>

    <div v-if="error" class="border-b border-red-900 bg-red-950/50 px-6 py-2 text-xs text-red-300">
      {{ error }}
    </div>

    <div class="flex-1 overflow-auto px-6 py-4">
      <section class="mb-6 rounded border border-zinc-800 bg-zinc-900/40 p-4">
        <h3 class="mb-3 text-sm font-medium text-zinc-200">새 마스크 추가</h3>
        <div class="flex flex-col gap-2 sm:flex-row">
          <input
            v-model="newFile"
            type="text"
            placeholder="config/application.yaml"
            class="flex-1 rounded border border-zinc-700 bg-zinc-950 px-2 py-1 font-mono text-xs text-zinc-100 placeholder-zinc-600 focus:border-zinc-500 focus:outline-none"
            :disabled="busy"
          />
          <input
            v-model="newPath"
            type="text"
            placeholder="spring.datasource.password"
            class="flex-1 rounded border border-zinc-700 bg-zinc-950 px-2 py-1 font-mono text-xs text-zinc-100 placeholder-zinc-600 focus:border-zinc-500 focus:outline-none"
            :disabled="busy"
            @keydown.enter="onAdd"
          />
          <button class="btn btn-primary" :disabled="busy" @click="onAdd">추가</button>
        </div>
      </section>

      <section>
        <h3 class="mb-3 text-sm font-medium text-zinc-200">
          등록된 마스크
          <span class="ml-1 text-xs text-zinc-500">({{ masks.length }})</span>
        </h3>

        <div v-if="loading" class="text-xs text-zinc-500">불러오는 중...</div>
        <div v-else-if="masks.length === 0" class="text-xs text-zinc-500">
          (마스크가 등록되지 않았습니다)
        </div>
        <ul v-else class="space-y-3">
          <li
            v-for="m in masks"
            :key="m.file"
            class="rounded border border-zinc-800 bg-zinc-900/40 p-3"
          >
            <div class="flex items-center justify-between">
              <code class="text-xs font-medium text-zinc-100">{{ m.file }}</code>
              <button
                class="btn btn-danger text-[11px]"
                :disabled="busy"
                @click="onRemoveFile(m.file)"
              >
                파일 전체 제거
              </button>
            </div>
            <ul class="mt-2 space-y-1">
              <li
                v-for="p in m.paths"
                :key="p"
                class="flex items-center justify-between rounded bg-zinc-950 px-2 py-1"
              >
                <code class="text-[11px] text-zinc-300">{{ p }}</code>
                <button
                  class="text-[11px] text-zinc-500 hover:text-red-400"
                  :disabled="busy"
                  @click="onRemovePath(m.file, p)"
                >
                  ✕
                </button>
              </li>
            </ul>
          </li>
        </ul>
      </section>
    </div>
  </div>
  <div v-else class="flex h-full items-center justify-center text-zinc-500">
    폴더를 찾을 수 없습니다.
  </div>
</template>
