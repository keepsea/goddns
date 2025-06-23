# Go-DDNS: 一个支持多用户的动态域名解析工具 (V2.1.0)

![Go Version](https://img.shields.io/badge/Go-1.18+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macos-lightgrey.svg)

这是一个使用 Go 语言编写的、功能强大的动态域名解析（DDNS）服务，旨在为您和您的用户提供一个稳定、安全且易于管理的多用户DDNS解决方案。

---

## ✨ V2.1.0 核心功能

相较于之前的版本，V2.1.0带来了架构级和功能性的全面升级：

- **多用户支持**: 服务端可通过`users.json`文件轻松管理多个用户，每个用户拥有独立的密钥和域名配置。
- **自动域名注册**: 用户首次请求解析新域名时，服务端会自动检查冲突并在阿里云创建A记录，无需手动预先配置。
- **域名配额管理**: 可为每个用户设置可拥有的域名数量上限（默认为1），有效防止资源滥用。
- **客户端CLI管理**: 客户端升级为功能强大的命令行工具，支持查看已用域名、手动注销域名、以及安全地重置加密密钥等自助管理操作。
- **应用层加密**: 客户端与服务端之间的所有核心通信都使用用户独立的密钥进行AES-GCM加密，确保数据在传输过程中的机密性。
- **模块化架构**: 服务端和客户端代码均经过重构，权责分明，更易于维护和二次开发。
- **安全增强**: 引入了速率限制、请求大小限制和严格的输入验证，提升了服务的健壮性。

## 🏗️ 架构

本工具由一个服务端程序和一个客户端程序组成：

- **服务端 (`ddns-server`)**: 部署在具有固定公网IP的服务器上（如阿里云ECS）。它现在是一个多用户服务中心，负责用户认证、域名校验与注册、以及调用阿里云API执行DNS更新。
- **客户端 (`ddns-client`)**: 部署在任意需要DDNS服务的机器上（如家庭服务器、办公室电脑）。它现在是一个命令行工具，不仅能自动更新IP，还能让用户管理自己的域名记录和密钥。

## ⚙️ 使用方法 (针对普通用户)

如果您不想自己编译，最简单的方式是从本仓库的 **Releases** 页面下载已编译好的可执行程序。

### 第1步：下载程序

访问本项目的 [Releases 页面](https://github.com/keepsea/goddns/releases) (请将 `keepsea/goddns` 替换为您的实际仓库地址)，根据您的操作系统下载对应的服务端和客户端程序。

### 第2步：配置

下载后，您需要为服务端和客户端分别创建配置文件。

**服务端配置 (放置在 `ddns-server-linux` 同一目录):**

1.  **`server.ini`**:
    ```ini
    [server]
    # 服务监听的端口号
    listen_port = 9876
    ```

2.  **`users.json`**:
    ```json
    {
      "users": [
        {
          "username": "okrj",
          "secret_token": "a-very-strong-token-for-okrj",
          "encryption_key": "a-32-byte-long-unique-encryption-key-!",
          "domain_limit": 2,
          "records": []
        },
        {
          "username": "friend1",
          "secret_token": "another-secret-token-for-friend",
          "encryption_key": "another-32-byte-unique-key-for-friend-!",
          "domain_limit": 1,
          "records": []
        }
      ]
    }
    ```

**客户端 `config.ini` (放置在客户端程序同一目录):**
```ini
[client]
# DDNS服务端的完整URL地址
server_url = http://YOUR_ECS_PUBLIC_IP:9876

# 您在这个系统中的用户名 (必须与服务端 users.json 中的配置匹配)
username = okrj
# 您的认证密钥 (secret_token)，用于向服务端证明您的权限
secret_token = a-very-strong-token-for-okrj
# 您的独立加密密钥 (encryption_key)，用于加密所有通信内容
encryption_key = a-32-byte-long-unique-encryption-key-!

# --- 以下配置仅在运行IP更新时需要 ---
# 您希望注册和更新的主域名
domain_name = anxinred.com
# 您希望注册和更新的主机记录 (例如 'www', 'nas')
rr = homehost
# 检查公网IP的时间间隔（秒）
check_interval_seconds = 300
```

### 第3步：运行程序

#### 服务端 (在阿里云ECS上)
1.  **设置阿里云凭证**:
    ```bash
    export ALIBABA_CLOUD_ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
    export ALIBABA_CLOUD_ACCESS_KEY_SECRET="YOUR_ACCESS_KEY_SECRET"
    ```

2.  **配置安全组**：登录阿里云控制台，确保ECS的安全组已经放行了您在`server.ini`中设置的`listen_port`。

3.  **启动服务**:
    ```bash
    chmod +x ./ddns-server-linux
    nohup ./ddns-server-linux > ddns.log 2>&1 &
    ```

#### 客户端 (在家庭服务器/Windows/macOS上)
客户端现在是一个命令行工具 (CLI)。

* **启动后台IP更新 (默认操作)**:
    ```bash
    # 在Linux/macOS上
    nohup ./ddns-client-linux -update > ddns-client.log 2>&1 &
    # 在Windows上直接运行
    .\ddns-client.exe -update
    ```
* **查看已注册的域名**:
    ```bash
    ./ddns-client-linux -list
    ```
* **注销一个域名**:
    ```bash
    ./ddns-client-linux -remove home.example.com
    ```
* **查看加密密钥**:
    ```bash
    ./ddns-client-linux -view-key
    ```
* **重置加密密钥**:
    ```bash
    ./ddns-client-linux -reset-key
    ```
* **查看帮助**:
    ```bash
    ./ddns-client-linux -help
    ```

## 👨‍💻 从源码编译 (针对开发者)

1.  **克隆仓库**:
    ```bash
    git clone [https://github.com/keepsea/goddns.git](https://github.com/keepsea/goddns.git)
    cd goddns
    ```

2.  **编译**:
    本项目已包含所有依赖 (`vendor`目录)，您无需下载任何东西，可以直接编译。例如，为Linux编译服务端：
    ```bash
    cd ddns-server
    go build -mod=vendor -o ddns-server-linux .
    ```

## ⚠️ 注意事项
- 请务必保证服务端和客户端的 `secret_token`, `username`, `encryption_key` 完全匹配。
- 请妥善保管您的阿里云AccessKey和用户密钥，不要泄露。
- 建议在生产环境中配合Nginx等反向代理，为服务端启用HTTPS加密通信。
