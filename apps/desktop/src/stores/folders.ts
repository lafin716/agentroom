import { defineStore } from "pinia";
import { ref } from "vue";
import { storage } from "../services/storage";
import { fsx } from "../services/fs";
import type { RegisteredFolder } from "../types/ar";

function makeId(): string {
  return Math.random().toString(36).slice(2, 10) + Date.now().toString(36);
}

export const useFoldersStore = defineStore("folders", () => {
  const folders = ref<RegisteredFolder[]>([]);
  const loaded = ref(false);

  async function load() {
    folders.value = await storage.loadFolders();
    loaded.value = true;
  }

  async function persist() {
    await storage.saveFolders(folders.value);
  }

  async function add(): Promise<RegisteredFolder | null> {
    const picked = await fsx.pickFolder();
    if (!picked) return null;
    if (folders.value.some((f) => f.path === picked)) {
      return folders.value.find((f) => f.path === picked) ?? null;
    }
    const name = picked.split(/[\\/]/).filter(Boolean).pop() ?? picked;
    const entry: RegisteredFolder = {
      id: makeId(),
      path: picked,
      name,
      added_at: new Date().toISOString(),
    };
    folders.value.push(entry);
    await persist();
    return entry;
  }

  async function remove(id: string) {
    folders.value = folders.value.filter((f) => f.id !== id);
    await persist();
  }

  function byId(id: string): RegisteredFolder | undefined {
    return folders.value.find((f) => f.id === id);
  }

  return { folders, loaded, load, add, remove, byId };
});
