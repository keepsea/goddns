# Go-DDNS: 一个简单的动态域名解析工具

![Go Version](https://img.shields.io/badge/Go-1.18+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macos-lightgrey.svg)

使用 Go 语言编写的轻量级动态域名解析（DDNS）工具，用于解决在家庭或办公宽带等动态公网IP环境下，随时通过固定域名访问内部服务器的需求。

---

## 🚀 项目特点

- **简单高效**: 核心代码简洁，资源占用低，易于理解和部署。
- **跨平台**: 基于Go语言的特性，可轻松交叉编译，在 Linux, Windows, macOS 等主流操作系统上运行。
- **配置灵活**: 所有关键参数（如密钥、域名、端口等）均通过配置文件管理，无需修改代码。
- **高可用性**: 项目内置了所有依赖（`vendor`目录），克隆后即可直接编译，无需担心因网络问题导致的依赖下载失败。
- **安全**: 客户端与服务端通过自定义的`secret_token`进行认证，防止API被恶意调用。阿里云凭证通过环境变量配置，不暴露在代码或配置文件中。

## 🏗️ 架构

本工具由一个服务端程序和一个客户端程序组成：

- **服务端 (`ddns-server`)**: 部署在具有固定公网IP的服务器上（如阿里云ECS）。它负责接收客户端的IP更新请求，并调用阿里云的“云解析DNS”API来修改域名的A记录。
- **客户端 (`ddns-client`)**: 部署在公网IP动态变化的服务器上（如家庭网络中的服务器）。它会定期检测本机的公网IP，并在IP发生变化时通知服务端进行更新。

## ⚙️ 使用方法 (针对普通用户)

如果您不想自己编译，最简单的方式是从本仓库的 **Releases** 页面下载已编译好的可执行程序。

### 第1步：下载程序

访问本项目的 [Releases 页面](https://github.com/keepsea/goddns/releases) (请将 `keepsea/goddns` 替换为您的实际仓库地址)，根据您的操作系统下载对应的服务端和客户端程序。

- `ddns-server-linux`: 用于Linux服务器的服务端。
- `ddns-client-linux`: 用于Linux服务器的客户端。
- `ddns-client.exe`: 用于Windows的客户端。
- `ddns-client-macos-intel`: 用于macOS (Intel芯片)的客户端。
- `ddns-client-macos-arm`: 用于macOS (Apple芯片)的客户端。

### 第2步：配置

下载后，您需要为服务端和客户端分别创建一个`config.ini`配置文件。

**服务端 `config.ini` (放置在 `ddns-server-linux` 同一目录):**
```ini
[server]
# 服务监听的端口
listen_port = 9876
# 用于与客户端通信的认证密钥，请务必修改为一个复杂的随机字符串
secret_token = YOUR_SUPER_SECRET_TOKEN
```

**客户端 `config.ini` (放置在客户端程序同一目录):**
```ini
[client]
# DDNS服务端的完整URL地址，请务必修改为您的ECS公网IP和您设置的端口
server_url = http://YOUR_ECS_PUBLIC_IP:9876/update-dns
# 认证密钥，必须与服务端设置的完全一致
secret_token = YOUR_SUPER_SECRET_TOKEN
# 您的主域名，例如 yourdomain.com
domain_name = yourdomain.com
# 您要更新的子域名 (主机记录)，例如 home
rr = home
# 检查公网IP的时间间隔（秒），例如300秒（5分钟）
check_interval_seconds = 300
```

### 第3步：运行程序

#### 服务端 (在阿里云ECS上)
1.  **设置阿里云凭证** (只需首次配置):
    ```bash
    export ALIBABA_CLOUD_ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
    export ALIBABA_CLOUD_ACCESS_KEY_SECRET="YOUR_ACCESS_KEY_SECRET"
    ```
    *建议将这两行写入 `/etc/profile` 或 `~/.bash_profile` 使其永久生效。*

2.  **配置安全组**：登录阿里云控制台，确保ECS的安全组已经放行了您在`config.ini`中设置的`listen_port`。

3.  **启动服务**:
    ```bash
    chmod +x ./ddns-server-linux
    nohup ./ddns-server-linux > ddns.log 2>&1 &
    ```

#### 客户端 (在家庭服务器/Windows/macOS上)
- **在 Linux / macOS 上**:
    ```bash
    chmod +x ./ddns-client-linux
    nohup ./ddns-client-linux > ddns-client.log 2>&1 &
    ```

- **在 Windows 上**:
    直接**双击 `ddns-client.exe`** 即可在后台运行。

## 👨‍💻 从源码编译 (针对开发者)

如果您想自行修改和编译，请按以下步骤操作。

1.  **克隆仓库**:
    ```bash
    git clone [https://github.com/keepsea/goddns.git](https://github.com/keepsea/goddns.git)
    cd goddns
    ```
2.  **编译**:
    本项目已包含所有依赖 (`vendor`目录)，您无需下载任何东西，可以直接编译。在相应的项目目录下，使用 `go build` 命令进行编译。例如，为Linux编译服务端：
    ```bash
    cd ddns-server
    go build -mod=vendor -o ddns-server-linux main.go
    ```-mod=vendor` 标志告诉Go命令使用项目内置的依赖进行编译。
