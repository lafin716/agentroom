mod commands;

use commands::*;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            run_ar,
            read_agentignore,
            write_agentignore,
            path_exists,
            open_in_explorer,
            open_in_vscode,
            pick_folder,
            load_folders,
            save_folders,
        ])
        .run(tauri::generate_context!())
        .expect("error while running agentroom desktop");
}
