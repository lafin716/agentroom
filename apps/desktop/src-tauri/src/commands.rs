use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager};
use tauri_plugin_dialog::DialogExt;
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;
use tokio::io::AsyncWriteExt;

#[derive(Debug, thiserror::Error)]
pub enum CommandError {
    #[error("io: {0}")]
    Io(#[from] std::io::Error),
    #[error("tauri: {0}")]
    Tauri(#[from] tauri::Error),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    #[error("{0}")]
    Msg(String),
}

impl serde::Serialize for CommandError {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

type CmdResult<T> = Result<T, CommandError>;

#[derive(Serialize)]
pub struct ArResult {
    pub stdout: String,
    pub stderr: String,
    pub code: i32,
}

#[tauri::command]
pub async fn run_ar(app: AppHandle, folder: String, args: Vec<String>) -> CmdResult<ArResult> {
    let folder_path = PathBuf::from(&folder);
    if !folder_path.is_dir() {
        return Err(CommandError::Msg(format!("not a directory: {}", folder)));
    }

    let sidecar = app
        .shell()
        .sidecar("ar")
        .map_err(|e| CommandError::Msg(format!("sidecar not found: {}", e)))?
        .args(args)
        .current_dir(&folder_path);

    let (mut rx, _child) = sidecar
        .spawn()
        .map_err(|e| CommandError::Msg(format!("ar spawn 실패: {}", e)))?;

    let mut stdout = String::new();
    let mut stderr = String::new();
    let mut code: i32 = -1;

    while let Some(event) = rx.recv().await {
        match event {
            CommandEvent::Stdout(bytes) => {
                stdout.push_str(&String::from_utf8_lossy(&bytes));
            }
            CommandEvent::Stderr(bytes) => {
                stderr.push_str(&String::from_utf8_lossy(&bytes));
            }
            CommandEvent::Terminated(payload) => {
                code = payload.code.unwrap_or(-1);
            }
            CommandEvent::Error(err) => {
                stderr.push_str(&err);
            }
            _ => {}
        }
    }

    Ok(ArResult { stdout, stderr, code })
}

fn ignore_path(folder: &str) -> PathBuf {
    PathBuf::from(folder).join(".agentignore")
}

#[tauri::command]
pub async fn read_agentignore(folder: String) -> CmdResult<String> {
    let p = ignore_path(&folder);
    if !p.exists() {
        return Ok(String::new());
    }
    let bytes = tokio::fs::read(&p).await?;
    Ok(String::from_utf8_lossy(&bytes).to_string())
}

#[tauri::command]
pub async fn write_agentignore(folder: String, content: String) -> CmdResult<()> {
    let p = ignore_path(&folder);
    let dir = p
        .parent()
        .ok_or_else(|| CommandError::Msg("invalid folder".into()))?;
    tokio::fs::create_dir_all(dir).await?;

    let tmp = dir.join(format!(".agentignore.tmp.{}", std::process::id()));
    {
        let mut f = tokio::fs::File::create(&tmp).await?;
        f.write_all(content.as_bytes()).await?;
        f.sync_all().await?;
    }
    tokio::fs::rename(&tmp, &p).await?;
    Ok(())
}

#[tauri::command]
pub async fn path_exists(path: String) -> bool {
    Path::new(&path).exists()
}

#[tauri::command]
pub async fn open_in_explorer(app: AppHandle, path: String) -> CmdResult<()> {
    use tauri_plugin_opener::OpenerExt;
    app.opener()
        .open_path(path, None::<&str>)
        .map_err(|e| CommandError::Msg(e.to_string()))?;
    Ok(())
}

#[tauri::command]
pub async fn open_in_vscode(app: AppHandle, path: String) -> CmdResult<()> {
    let program = if cfg!(target_os = "windows") { "code.cmd" } else { "code" };
    let output = app
        .shell()
        .command(program)
        .args([&path])
        .output()
        .await;
    match output {
        Ok(out) if out.status.success() => Ok(()),
        Ok(out) => Err(CommandError::Msg(format!(
            "VS Code 실행 실패 (exit {}): {}",
            out.status.code().unwrap_or(-1),
            String::from_utf8_lossy(&out.stderr)
        ))),
        Err(e) => Err(CommandError::Msg(format!(
            "VS Code 실행 실패. PATH 에 `code` 가 등록되어 있는지 확인하세요. ({})",
            e
        ))),
    }
}

#[tauri::command]
pub async fn pick_folder(app: AppHandle) -> CmdResult<Option<String>> {
    let (tx, rx) = tokio::sync::oneshot::channel();
    app.dialog().file().pick_folder(move |result| {
        let _ = tx.send(result);
    });
    let picked = rx
        .await
        .map_err(|_| CommandError::Msg("dialog cancelled".into()))?;
    Ok(picked.map(|fp| fp.to_string()))
}

#[derive(Serialize, Deserialize, Clone)]
pub struct RegisteredFolder {
    pub id: String,
    pub path: String,
    pub name: String,
    pub added_at: String,
}

fn folders_file(app: &AppHandle) -> CmdResult<PathBuf> {
    let dir = app
        .path()
        .app_data_dir()
        .map_err(|e| CommandError::Msg(e.to_string()))?;
    std::fs::create_dir_all(&dir)?;
    Ok(dir.join("folders.json"))
}

#[tauri::command]
pub async fn load_folders(app: AppHandle) -> CmdResult<Vec<RegisteredFolder>> {
    let path = folders_file(&app)?;
    if !path.exists() {
        return Ok(Vec::new());
    }
    let bytes = tokio::fs::read(&path).await?;
    if bytes.is_empty() {
        return Ok(Vec::new());
    }
    let folders: Vec<RegisteredFolder> = serde_json::from_slice(&bytes)?;
    Ok(folders)
}

#[tauri::command]
pub async fn save_folders(app: AppHandle, folders: Vec<RegisteredFolder>) -> CmdResult<()> {
    let path = folders_file(&app)?;
    let dir = path
        .parent()
        .ok_or_else(|| CommandError::Msg("invalid app data dir".into()))?;
    tokio::fs::create_dir_all(dir).await?;
    let data = serde_json::to_vec_pretty(&folders)?;
    let tmp = dir.join(format!("folders.json.tmp.{}", std::process::id()));
    {
        let mut f = tokio::fs::File::create(&tmp).await?;
        f.write_all(&data).await?;
        f.sync_all().await?;
    }
    tokio::fs::rename(&tmp, &path).await?;
    Ok(())
}
