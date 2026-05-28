import { invoke } from "@tauri-apps/api/core";
import type { RegisteredFolder } from "../types/ar";

export const storage = {
  loadFolders: () => invoke<RegisteredFolder[]>("load_folders"),
  saveFolders: (folders: RegisteredFolder[]) =>
    invoke<void>("save_folders", { folders }),
};
