import { invoke } from "@tauri-apps/api/core";

export const fsx = {
  readAgentignore: (folder: string) =>
    invoke<string>("read_agentignore", { folder }),

  writeAgentignore: (folder: string, content: string) =>
    invoke<void>("write_agentignore", { folder, content }),

  exists: (path: string) => invoke<boolean>("path_exists", { path }),

  openInExplorer: (path: string) => invoke<void>("open_in_explorer", { path }),

  openInVscode: (path: string) => invoke<void>("open_in_vscode", { path }),

  pickFolder: () => invoke<string | null>("pick_folder"),
};
