# OBS 热键触发工具

这是一个基于 Go 的工具，用于通过热键切换 OBS Studio 中场景项目的可见性（例如图像遮罩）。该工具通过 WebSocket 连接到 OBS Studio，允许使用自定义键盘快捷键（例如 `Ctrl+M`）控制场景项目的显示/隐藏。
单纯使用请下载可执行版本，即下即用:https://github.com/Findlayzhou/OBS_hotkey_trigger/releases/tag/v1.0.0
## 功能

- **热键支持**：使用热键（例如 `Ctrl+M`）切换 OBS 场景项目的可见性。
- **动态场景检测**：如果未指定场景，自动查找包含指定源的场景。
- **YAML 配置**：通过 `conf.yaml` 文件定义 OBS 连接信息和遮罩配置。

## 前提条件

- **OBS Studio**：版本 28.0 或更高，启用 WebSocket 5.x（默认端口：4455）。
- **Go**：版本 1.18 或更高，用于构建工具。
- **Windows**：当前为 Windows 优化（因热键库限制）。

## 安装

1. **克隆仓库**：

   ```bash
   git clone https://github.com/Findlayzhou/OBS_hotkey_trigger.git
   cd OBS_hotkey_trigger
   ```

2. **安装依赖**：

   ```bash
   go get github.com/andreykaipov/goobs@v1.5.6
   go get golang.design/x/hotkey
   go get gopkg.in/yaml.v3@v3.0.1
   go mod tidy
   ```

3. **构建工具**：

   ```bash
   go build -o obs_hotkey_trigger.exe ./cmd/main.go
   ```

## 配置

在项目根目录创建 `conf.yaml` 文件（或通过 `-f` 指定自定义路径）。示例配置：

```yaml
obs:
  address: "localhost"   
  port: "4455"        
  password: "w21iEcHv8EwtKBPL" 

# 遮罩配置列表（可扩展多个遮罩）
masks:
  - name: "mask"   # 随便起
    source: "mask" 
    scene: ""      
    hotkey:        # 组合热键，此处示例为 ctrl + alt + M
      key: "M"     
      modifiers: ["ctrl","alt"]
```

- **obs.address**：OBS WebSocket 服务器地址（默认：`localhost`）。
- **obs.port**：WebSocket 端口（默认：`4455`）。
- **obs.password**：WebSocket 密码（在 OBS 的“工具 > WebSocket 服务器设置”中设置）。
- **masks**：遮罩配置列表：
  - **name**：遮罩显示名称（例如 `主遮罩`）。
  - **source**：OBS 源名称（例如 `mask`，必须精确匹配）。
  - **scene**：可选的场景名称，留空则自动检测包含源的场景。
  - **hotkey.key**：触发热键（例如 `M`）。
  - **hotkey.modifiers**：修饰键（例如 `ctrl`、`shift`、`alt`、`win`）。

## 设置 OBS

1. **启用 WebSocket**：
   - 在 OBS Studio 中，进入“工具” > “WebSocket 服务器设置”。
   - 启用服务器，设置端口（默认：`4455`），记录密码。
   - 建议禁用“连接时显示确认对话框”以实现自动化。

2. **创建源**：
   - 在 OBS 的“来源”面板添加图像源（例如 `mask`）：`来源` > `+` > `图像`。
   - 设置图像文件。

3. **验证场景和源**：
   - 确保 `conf.yaml` 中的 `source` 与 OBS 源名称精确匹配（区分大小写）。

## 使用方法

1. **运行工具**：

   - 默认配置：

     ```bash
     ./obs_hotkey_trigger.exe
     ```

   - 自定义配置：

     ```bash
     ./obs_hotkey_trigger.exe -f path/to/custom_conf.yaml
     ```

2. **预期输出**：

    无明显报错即可。
   ```plaintext
   PS C:\Users\findlay\Desktop\obs_mask_test> .\main.exe
    2025/09/16 22:31:54 No -f specified, using default config: conf.yaml
    2025/09/16 22:31:54 Connecting to OBS WebSocket at: localhost:4455
    2025/09/16 22:31:54 Available scenes:
    0 main
    2025/09/16 22:31:54 No scene found containing mask mask (source: mask1), assuming hidden
    2025/09/16 22:31:54 Available scenes:
    0 main
    2025/09/16 22:31:54 Found mask mask2 in current scene: main
    2025/09/16 22:31:54 Synced mask mask2 (scene item in main) initial state: true
    2025/09/16 22:31:54 Available scenes:
    0 main
    2025/09/16 22:31:54 Found mask mask3 in current scene: main
    2025/09/16 22:31:54 Synced mask mask3 (scene item in main) initial state: true
    2025/09/16 22:31:54 Mask state for mask not found or invalid scene, skipping hotkey registration
    2025/09/16 22:31:54 Registered hotkey ctrl+Z for mask mask2
    2025/09/16 22:31:54 Registered hotkey shift+Q for mask mask3
    2025/09/16 22:31:54 Hotkey listeners started. Press hotkeys to toggle masks...
    2025/09/16 22:31:54 To exit, press Ctrl+C
   ```
