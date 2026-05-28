<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ar } from "../services/ar";
import { fsx } from "../services/fs";
import { useFoldersStore } from "../stores/folders";
import { useOutputStore } from "../stores/output";
import type { InfoResult, StatusResult, SyncResult } from "../types/ar";
import SyncDialog from "../components/SyncDialog.vue";
import HistoryDialog from "../components/HistoryDialog.vue";
import ResetDialog from "../components/ResetDialog.vue";
import OpenTargetMenu from "../components/OpenTargetMenu.vue";
import OutputPanel from "../components/OutputPanel.vue";

const route = useRoute();
const router = useRouter();
const folders = useFoldersStore();
const output = useOutputStore();

const folder = computed(() => folders.byId(route.params.id as string));
const info = ref<InfoResult | null>(null);
const status = ref<StatusResult | null>(null);
const workspaceExists = ref<boolean | null>(null);
const initialized = computed(() => info.value?.initialized === true);
const hasWorkspace = computed(() => !!info.value?.workspace_path && workspaceExists.value === true);
const showSync = ref(false);
const showHistory = ref(false);
const showReset = ref(false);
const busy = ref(false);

async function withBusy<T>(label: string, fn: () => Promise<T>): Promise<T | null> {
  busy.value = true;
  output.info(`▶ ${label}`);
  try {
    const r = await fn();
    output.info(`✔ ${label}`);
    return r;
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    output.error(`✖ ${label}: ${msg}`);
    return null;
  } finally {
    busy.value = false;
  }
}

async function refresh() {
  if (!folder.value) return;
  info.value = null;
  status.value = null;
  workspaceExists.value = null;
  try {
    info.value = await ar.info(folder.value.path);
  } catch {
    info.value = null;
  }
  if (info.value?.workspace_path) {
    workspaceExists.value = await fsx.exists(info.value.workspace_path);
  } else {
    workspaceExists.value = false;
  }
  if (hasWorkspace.value) {
    try {
      status.value = await ar.status(folder.value.path);
    } catch {
      status.value = null;
    }
  }
}

watch(() => route.params.id, refresh, { immediate: false });
onMounted(refresh);

async function onInit() {
  if (!folder.value) return;
  await withBusy("ar init", () => ar.init(folder.value!.path));
  await refresh();
}

async function onCopy(reCopy = false) {
  if (!folder.value) return;
  if (reCopy && !confirm("기존 워크스페이스를 덮어씁니다. 진행할까요?")) return;
  await withBusy("ar copy" + (reCopy ? " --force" : ""), () =>
    ar.copy(folder.value!.path, { force: reCopy }),
  );
  await refresh();
}

async function onStatus() {
  if (!folder.value) return;
  await withBusy("ar status", async () => {
    status.value = await ar.status(folder.value!.path);
  });
}

function openSyncDialog() {
  if (!status.value) return;
  showSync.value = true;
}

async function applySync(opts: { force: boolean; withDelete: boolean }): Promise<SyncResult | null> {
  if (!folder.value) return null;
  const r = await withBusy("ar sync" + (opts.force ? " --force" : ""), () =>
    ar.sync(folder.value!.path, opts),
  );
  await refresh();
  return r;
}

async function onUndo() {
  if (!folder.value) return;
  if (!confirm("가장 최근 sync 를 되돌립니다. 진행할까요?")) return;
  await withBusy("ar undo", () => ar.undo(folder.value!.path, 1));
  await refresh();
}

async function onOpenExplorer(targetPath: string) {
  await withBusy("탐색기 열기", () => fsx.openInExplorer(targetPath));
}

async function onOpenVscode(targetPath: string) {
  await withBusy("VS Code 열기", () => fsx.openInVscode(targetPath));
}

async function applyReset(opts: { withHistory: boolean }) {
  if (!folder.value) return;
  await withBusy(
    "ar reset" + (opts.withHistory ? " --with-history" : ""),
    () => ar.reset(folder.value!.path, opts),
  );
  await refresh();
}

function goEditIgnore() {
  if (!folder.value) return;
  router.push(`/folders/${folder.value.id}/ignore`);
}

function goEditMasks() {
  if (!folder.value) return;
  router.push(`/folders/${folder.value.id}/masks`);
}

const changeCount = computed(() => status.value?.workspace_changes.length ?? 0);
const conflictCount = computed(() => status.value?.conflicts.length ?? 0);
</script>

<template>
  <div v-if="folder" class="flex h-full flex-col">
    <header class="border-b border-zinc-800 px-6 py-4">
      <div class="flex items-baseline gap-3">
        <h2 class="text-lg font-semibold text-zinc-100">{{ folder.name }}</h2>
        <span class="text-xs text-zinc-500">{{ folder.path }}</span>
      </div>
      <p v-if="info?.workspace_path" class="mt-1 text-xs text-zinc-500">
        workspace: {{ info.workspace_path }}
        <span v-if="workspaceExists === false" class="ml-2 text-red-400">(존재하지 않음)</span>
      </p>
      <p v-if="info?.last_sync_at" class="text-xs text-zinc-500">
        마지막 sync: {{ new Date(info.last_sync_at).toLocaleString() }}
      </p>
    </header>

    <section class="border-b border-zinc-800 px-6 py-4">
      <div class="flex flex-wrap items-center gap-x-1 gap-y-2">
        <button class="btn btn-primary" :disabled="busy || initialized" @click="onInit">
          Init
        </button>
        <button class="btn btn-secondary" :disabled="busy || !initialized" @click="goEditIgnore">
          .agentignore 편집
        </button>
        <button class="btn btn-secondary" :disabled="busy || !initialized" @click="goEditMasks">
          마스크 편집
        </button>

        <div class="mx-2 h-5 w-px bg-zinc-700" />

        <button
          class="btn btn-primary"
          :disabled="busy || !initialized || hasWorkspace"
          @click="onCopy(false)"
        >
          Copy
        </button>
        <button
          class="btn btn-secondary"
          :disabled="busy || !hasWorkspace"
          @click="onCopy(true)"
        >
          Re-copy
        </button>
        <button
          class="btn btn-danger"
          :disabled="busy || !hasWorkspace"
          @click="showReset = true"
        >
          Reset
        </button>

        <div class="mx-2 h-5 w-px bg-zinc-700" />

        <button class="btn btn-secondary" :disabled="busy || !hasWorkspace" @click="onStatus">
          Status
        </button>
        <button
          class="btn btn-primary"
          :class="{ 'ring-2 ring-yellow-400': changeCount > 0 }"
          :disabled="busy || !hasWorkspace || !status || changeCount === 0"
          @click="openSyncDialog"
        >
          Sync
          <span v-if="changeCount > 0" class="ml-1 rounded bg-yellow-400 px-1 text-[10px] text-zinc-900">
            {{ changeCount }}
          </span>
        </button>
        <button class="btn btn-secondary" :disabled="busy" @click="showHistory = true">
          History
        </button>
        <button class="btn btn-secondary" :disabled="busy" @click="onUndo">Undo</button>

        <div class="mx-2 h-5 w-px bg-zinc-700" />

        <OpenTargetMenu
          label="탐색기"
          :main-path="folder.path"
          :workspace-path="info?.workspace_path"
          :has-workspace="hasWorkspace"
          :busy="busy"
          @open="onOpenExplorer"
        />
        <OpenTargetMenu
          label="VS Code"
          :main-path="folder.path"
          :workspace-path="info?.workspace_path"
          :has-workspace="hasWorkspace"
          :busy="busy"
          @open="onOpenVscode"
        />
      </div>
      <p v-if="!initialized" class="mt-3 text-xs text-zinc-500">
        먼저 <strong>Init</strong> 으로 <code>.agent/config.json</code> 과 <code>.agentignore</code> 를 생성하세요.
      </p>
    </section>

    <section v-if="status" class="border-b border-zinc-800 px-6 py-4 text-sm">
      <div class="flex gap-6">
        <div>
          <h3 class="font-medium text-zinc-300">변경 ({{ changeCount }})</h3>
          <ul class="mt-1 max-h-32 overflow-auto font-mono text-xs">
            <li v-for="c in status.workspace_changes" :key="c.path">
              <span class="text-zinc-500">{{ c.op[0] }}</span>
              {{ c.path }}
            </li>
            <li v-if="changeCount === 0" class="text-zinc-500">(없음)</li>
          </ul>
        </div>
        <div v-if="conflictCount > 0">
          <h3 class="font-medium text-red-400">충돌 ({{ conflictCount }})</h3>
          <ul class="mt-1 max-h-32 overflow-auto font-mono text-xs text-red-300">
            <li v-for="p in status.conflicts" :key="p">! {{ p }}</li>
          </ul>
        </div>
      </div>
    </section>

    <OutputPanel class="flex-1" />

    <SyncDialog
      v-if="showSync && status"
      :status="status"
      @apply="async (opts) => { showSync = false; await applySync(opts); }"
      @cancel="showSync = false"
    />
    <HistoryDialog
      v-if="showHistory && folder"
      :folder-path="folder.path"
      @close="showHistory = false"
    />
    <ResetDialog
      v-if="showReset"
      :workspace-path="info?.workspace_path ?? ''"
      @apply="async (opts) => { showReset = false; await applyReset(opts); }"
      @cancel="showReset = false"
    />
  </div>
  <div v-else class="flex h-full items-center justify-center text-zinc-500">
    폴더를 선택하세요.
  </div>
</template>
