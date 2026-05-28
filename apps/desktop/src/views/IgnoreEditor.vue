<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { EditorState, Compartment } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, Decoration, type DecorationSet, ViewPlugin, ViewUpdate } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { fsx } from "../services/fs";
import { useFoldersStore } from "../stores/folders";
import { useOutputStore } from "../stores/output";

const route = useRoute();
const router = useRouter();
const folders = useFoldersStore();
const output = useOutputStore();

const folder = computed(() => folders.byId(route.params.id as string));

const host = ref<HTMLDivElement | null>(null);
const original = ref<string>("");
const current = ref<string>("");
const loading = ref(true);
const saving = ref(false);
const error = ref<string | null>(null);

const dirty = computed(() => current.value !== original.value);

let view: EditorView | null = null;
const themeCompartment = new Compartment();

const ignoreHighlighter = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = this.build(view);
    }
    update(u: ViewUpdate) {
      if (u.docChanged || u.viewportChanged) {
        this.decorations = this.build(u.view);
      }
    }
    build(view: EditorView): DecorationSet {
      const builder: { from: number; to: number; deco: Decoration }[] = [];
      for (const { from, to } of view.visibleRanges) {
        for (let pos = from; pos <= to; ) {
          const line = view.state.doc.lineAt(pos);
          const text = line.text;
          const trimmed = text.trimStart();
          if (trimmed.startsWith("#")) {
            builder.push({
              from: line.from,
              to: line.to,
              deco: Decoration.line({ attributes: { class: "cm-ign-comment" } }),
            });
          } else if (trimmed.startsWith("!")) {
            builder.push({
              from: line.from,
              to: line.to,
              deco: Decoration.line({ attributes: { class: "cm-ign-negate" } }),
            });
          }
          pos = line.to + 1;
          if (line.to >= to) break;
        }
      }
      return Decoration.set(
        builder.map((b) => b.deco.range(b.from)),
        true,
      );
    }
  },
  { decorations: (v) => v.decorations },
);

const editorTheme = EditorView.theme(
  {
    "&": {
      height: "100%",
      backgroundColor: "transparent",
      color: "#e4e4e7",
      fontSize: "13px",
    },
    ".cm-scroller": { fontFamily: "'JetBrains Mono', 'Fira Code', ui-monospace, monospace" },
    ".cm-gutters": {
      backgroundColor: "#0a0a0a",
      borderRight: "1px solid #27272a",
      color: "#52525b",
    },
    ".cm-activeLineGutter": { backgroundColor: "#18181b" },
    ".cm-activeLine": { backgroundColor: "rgba(63,63,70,0.3)" },
    ".cm-content": { caretColor: "#fafafa" },
    ".cm-ign-comment": { color: "#71717a", fontStyle: "italic" },
    ".cm-ign-negate": { color: "#facc15" },
  },
  { dark: true },
);

async function load() {
  if (!folder.value) return;
  loading.value = true;
  error.value = null;
  try {
    const txt = await fsx.readAgentignore(folder.value.path);
    original.value = txt;
    current.value = txt;
    if (view) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: txt },
      });
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    output.error(`✖ .agentignore 읽기: ${msg}`);
  } finally {
    loading.value = false;
  }
}

function mountEditor() {
  if (!host.value || view) return;
  const state = EditorState.create({
    doc: current.value,
    extensions: [
      lineNumbers(),
      highlightActiveLineGutter(),
      highlightActiveLine(),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
      EditorView.lineWrapping,
      ignoreHighlighter,
      themeCompartment.of(editorTheme),
      EditorView.updateListener.of((u) => {
        if (u.docChanged) {
          current.value = u.state.doc.toString();
        }
      }),
    ],
  });
  view = new EditorView({ state, parent: host.value });
}

async function onSave() {
  if (!folder.value || !dirty.value) return;
  saving.value = true;
  output.info("▶ .agentignore 저장");
  try {
    await fsx.writeAgentignore(folder.value.path, current.value);
    original.value = current.value;
    output.info("✔ .agentignore 저장");
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    output.error(`✖ .agentignore 저장: ${msg}`);
  } finally {
    saving.value = false;
  }
}

function onCancel() {
  if (dirty.value && !confirm("저장하지 않은 변경이 있습니다. 나가시겠습니까?")) return;
  if (folder.value) router.push(`/folders/${folder.value.id}`);
  else router.push("/");
}

function onRevert() {
  if (!dirty.value) return;
  if (!confirm("변경사항을 버리고 마지막 저장본으로 되돌립니다.")) return;
  current.value = original.value;
  if (view) {
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: original.value },
    });
  }
}

watch(
  () => route.params.id,
  () => load(),
);

onMounted(async () => {
  await load();
  await nextTick();
  mountEditor();
});

onBeforeUnmount(() => {
  view?.destroy();
  view = null;
});
</script>

<template>
  <div v-if="folder" class="flex h-full flex-col">
    <header class="flex items-center justify-between border-b border-zinc-800 px-6 py-3">
      <div>
        <h2 class="text-sm font-semibold text-zinc-100">.agentignore 편집</h2>
        <p class="text-[11px] text-zinc-500">{{ folder.path }}</p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary" @click="onCancel">뒤로</button>
        <button class="btn btn-secondary" :disabled="!dirty || saving" @click="onRevert">
          되돌리기
        </button>
        <button class="btn btn-primary" :disabled="!dirty || saving" @click="onSave">
          {{ saving ? "저장 중..." : "저장" }}
          <span v-if="dirty" class="ml-1 text-yellow-300">●</span>
        </button>
      </div>
    </header>

    <p class="border-b border-zinc-800 bg-zinc-900/60 px-6 py-2 text-[11px] text-zinc-400">
      gitignore 와 동일한 문법입니다. <code>#</code> 주석, <code>!</code> 부정 패턴 지원.
      여기에 적힌 파일은 워크스페이스로 복사되지 않고, sync 에서도 절대 건드리지 않습니다.
    </p>

    <div v-if="error" class="border-b border-red-900 bg-red-950/50 px-6 py-2 text-xs text-red-300">
      {{ error }}
    </div>

    <div v-if="loading" class="flex-1 px-6 py-4 text-xs text-zinc-500">불러오는 중...</div>
    <div v-else ref="host" class="flex-1 overflow-hidden bg-zinc-950"></div>
  </div>
  <div v-else class="flex h-full items-center justify-center text-zinc-500">
    폴더를 찾을 수 없습니다.
  </div>
</template>
