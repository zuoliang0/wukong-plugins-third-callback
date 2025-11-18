# 悟空IM 第三方消息回调插件

![Version](https://img.shields.io/badge/version-0.0.1-blue)
![Language](https://img.shields.io/badge/language-Go%201.25-brightgreen)
![Plugin](https://img.shields.io/badge/plugin-wk.plugin.third.msg.callback-orange)

> 为悟空IM提供第三方消息拦截与回调能力，在消息发送前将消息内容推送到外部系统进行审核、修改或拦截。

## 📋 目录

- [核心功能](#核心功能)
- [快速开始](#快速开始)
- [API 规范](#api-规范)
- [签名验证](#签名验证)
- [配置指南](#配置指南)
- [使用示例](#使用示例)
- [常见问题](#常见问题)
- [故障排查](#故障排查)

---

## 核心功能

### 主要能力

| 功能 | 描述 |
|------|------|
| **消息拦截** | 在消息发送前进行第三方审核 |
| **消息修改** | 支持修改消息内容后允许发送 |
| **安全签名** | SHA1 + MD5 企业级签名验证 |
| **自动重试** | 失败时自动重试（可配置次数） |
| **超时控制** | 灵活的超时策略（允许发送或拒绝） |
| **灵活配置** | 支持多种配置参数的动态调整 |

### 工作流程

```
┌─────────────────┐
│   悟空IM消息    │
└────────┬────────┘
         │ (消息发送触发)
         ▼
┌─────────────────────────────────┐
│  本插件 (ThirdMsgCallback)       │
│                                 │
│ 1. 提取消息元数据               │
│ 2. 生成签名 (SHA1 + MD5)       │
│ 3. 构造HTTP请求                 │
│ 4. 发送到第三方URL              │
│ 5. 实现重试机制                 │
└────────┬────────────────────────┘
         │ (HTTP POST)
         ▼
┌─────────────────────────────────┐
│  第三方应用服务器               │
│                                 │
│ 1. 验证签名                      │
│ 2. 执行业务逻辑 (审核/过滤)    │
│ 3. 返回决策 (允许/拒绝/修改)   │
└────────┬────────────────────────┘
         │ (JSON响应)
         ▼
┌─────────────────────────────────┐
│  结果处理                        │
│                                 │
│ - 允许发送 ✓                     │
│ - 拒绝发送 ✗                     │
│ - 修改消息 ↻                     │
└─────────────────────────────────┘
```

---

## 快速开始

### 环境要求

- **Go**: 1.25 或更高版本
- **悟空IM**: 最新版本（支持插件加载）
- **网络**: 第三方应用服务器需可公网访问或与悟空IM网络互通

### 构建插件

#### 1. 本地构建（当前平台）

```bash
# 克隆项目
git clone https://github.com/WuKongIM/wukong-plugins-third-callback.git
cd wukong-plugins-third-callback

# 编译插件
go build -o plugin.wkp main.go
```

**输出**: `plugin.wkp`（悟空IM插件文件）

#### 2. 跨平台构建

```bash
# 使用构建脚本（支持多平台）
bash build.sh
```

**输出目录**: `build/`

| 平台 | 文件名 |
|------|--------|
| Linux ARM64 | `wk.plugin.third.msg.callback-linux-arm64.wkp` |
| Linux AMD64 | `wk.plugin.third.msg.callback-linux-amd64.wkp` |
| macOS Intel | `wk.plugin.third.msg.callback-darwin-amd64.wkp` |
| macOS Apple Silicon | `wk.plugin.third.msg.callback-darwin-arm64.wkp` |

### 安装与启用

#### 步骤 1: 上传插件

1. 登录悟空IM后台管理系统
2. 进入 **插件管理** 或 **扩展** 菜单
3. 点击 **上传插件**
4. 选择编译好的 `.wkp` 文件

#### 步骤 2: 配置插件

1. 在插件管理页面找到 `wk.plugin.third.msg.callback`
2. 点击 **编辑配置** 或 **设置**
3. 填写以下配置项：

| 配置项 | 示例值 | 说明 |
|-------|--------|------|
| `CallbackUrl` | `https://api.example.com/callback` | 第三方接口的完整URL |
| `AppSecret` | `your-app-secret-key` | 签名密钥，与第三方应用协商 |
| `Timeout` | `5` | 请求超时时间（秒） |
| `TimeoutSend` | `false` | 超时后是否允许消息发送 |
| `Retries` | `3` | 失败重试次数 |

#### 步骤 3: 启用插件

1. 点击 **启用** 按钮
2. 查看日志确认插件加载成功
3. 发送测试消息验证功能

---

## API 规范

### 请求规范

#### 请求方法

```
POST {CallbackUrl}
```

#### 请求头

插件发送的HTTP请求包含以下请求头：

| 请求头 | 类型 | 说明 | 示例 |
|-------|------|------|------|
| `AppKey` | String | 应用Key（来自插件配置） | `wk.plugin.third.msg.callback` |
| `CurTime` | Long | 当前UTC时间戳（毫秒） | `1668084361000` |
| `MD5` | String | 请求体的MD5值（十六进制小写） | `5d41402abc4b2a76b9719d911017c592` |
| `CheckSum` | String | 校验值：SHA1(AppSecret + MD5 + CurTime) | `356a192b7913b04c54574d18c28d46e6395428ab` |
| `Content-Type` | String | 请求内容类型 | `application/json` |

#### 请求体

```json
{
  "msgBody": "base64(消息json)",
  "fromUid": "user123",
  "channelId": "channel456",
  "channelType": 1,
  "deviceId": "device789",
  "deviceFlag": 0,
  "deviceLevel": 1
}
```

**请求体字段说明**：

| 字段 | 类型 | 说明                                      | 示例             |
|------|------|-----------------------------------------|----------------|
| `msgBody` | String | 消息内容（base64编码的请求体）                      | `"xxxxxx"`     |
| `fromUid` | String | 发送者用户ID                                 | `"user123"`    |
| `channelId` | String | 频道ID（单聊时为对方ID，群聊时为群ID）                  | `"channel456"` |
| `channelType` | uint32 | 频道类型 `1=单聊` `2=群聊`                      | `1`            |
| `deviceId` | String | 发送设备ID                                  | `"device789"`  |
| `deviceFlag` | uint8 | 设备类型 `0=APP` `1=WEB` `2=PC` `99=SYSTEM` | `0`            |
| `deviceLevel` | uint8 | 设备级别 `0=从设备` `1=主设备`                    | `1`            |

### 响应规范

#### 响应状态码

| 状态码 | 说明 |
|--------|------|
| `200` | 成功（无论是否允许发送） |
| `400` | 请求格式错误 |
| `401` | 签名验证失败 |
| `500` | 服务器内部错误 |

#### 响应体

```json
{
  "allow": true,
  "msgBody": "base64(修改后的消息json)"
}
```

**响应体字段说明**：

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `allow` | Boolean | ✓ | 是否允许消息发送 |
| `msgBody` | String | ✗ | 修改后的消息内容（Base64编码，仅当allow=true且需要修改时） |

#### 响应示例

**示例1：允许发送（无修改）**
```json
{
  "allow": true
}
```

**示例2：拒绝发送**
```json
{
  "allow": false
}
```

**示例3：允许发送（修改消息）**
```json
{
  "allow": true,
  "msgBody": "TW9kaWZpZWQgTWVzc2FnZQ=="
}
```

> **Note**: `msgBody` 字段需采用 Base64 编码。如果不需要修改消息，可以省略该字段。

---

## 签名验证

### 验证原理

签名验证采用 **两步验证** 机制：

1. **第一步**：计算请求体的 MD5 值
2. **第二步**：使用 AppSecret、MD5 和 CurTime 计算 SHA1 校验值

### 验证算法

#### Go 示例

```go
package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"net/http"
)

// VerifySignature 验证请求签名
func VerifySignature(req *http.Request, appSecret string) bool {
	checksum := req.Header.Get("CheckSum")
	md5Val := req.Header.Get("MD5")
	curTime := req.Header.Get("CurTime")

	// 重新计算 CheckSum
	toSign := appSecret + md5Val + curTime
	sha1Hash := sha1.Sum([]byte(toSign))
	calculatedChecksum := hex.EncodeToString(sha1Hash[:])

	// 比对（忽略大小写）
	return strings.EqualFold(checksum, calculatedChecksum)
}

// CalculateMD5 计算请求体的MD5
func CalculateMD5(data string) string {
	decodeBase64, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return ""
	}
	hash := md5.Sum(decodeBase64)
	return hex.EncodeToString(hash[:])
}
//假设你的请求函数

func (t *WukongController) WukongMsgCallBack(c *gin.Context) {
	s := VerifySignature(c, t.config.Wukong.CallbackKey)
	if !s {
		c.String(http.StatusForbidden, "forbidden")
		return
	}
	var req ginmodel.ThirdMsgCallbackReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	md5Client := c.GetHeader("MD5")
	md5Server := CalculateMD5(req.MsgBody)
	if md5Client != md5Server {
		c.String(http.StatusBadRequest, "md5 mismatch")
		return
	}
	//真实的处理器
	allow, modifiedMsgBody := t.dao.WukongMsgCallBack(&req)
	resp := ginmodel.ThirdMsgCallbackResp{
		Allow:   allow,
		MsgBody: modifiedMsgBody,
	}
	c.JSON(http.StatusOK, resp)
}
```

### 验证清单

- [ ] 检查 `CheckSum` 是否为小写十六进制格式
- [ ] 确认 `AppSecret` 与插件配置完全一致
- [ ] 验证 `MD5` 是否基于body-JSON计算
- [ ] 检查时间差（CurTime应为当前时间，差异不应超过5秒）
- [ ] 确保字符编码为 UTF-8

---

## 配置指南

### 配置项详解

#### 1. CallbackUrl（必需）

**类型**: String
**示例**: `https://api.example.com/message/callback`
**说明**: 第三方应用接收回调的完整URL地址

**注意事项**：
- 必须是完整的 URL（包含协议 `http://` 或 `https://`）
- 第三方应用必须部署在可达的服务器上
- 建议使用 HTTPS 以确保安全性
- 支持带查询参数的URL：`https://api.example.com/callback?token=xxx`

#### 2. AppSecret（必需）

**类型**: String
**示例**: `your-secret-key-12345`
**说明**: 用于生成签名的密钥，需与第三方应用协商

**最佳实践**：
- 长度至少 16 个字符
- 混合使用大小写字母、数字和特殊字符
- 不要在日志中打印
- 定期更换密钥
- 不要在代码中硬编码，使用环境变量或配置管理系统

#### 3. Timeout（可选）

**类型**: Integer (秒)
**默认值**: `5`
**范围**: `1` - `60`
**说明**: HTTP 请求的超时时间

**建议值**：
- 快速审核逻辑: `3 - 5` 秒
- 复杂业务逻辑: `10 - 15` 秒
- 长流程处理: `20 - 30` 秒

#### 4. TimeoutSend（可选）

**类型**: Boolean
**默认值**: `false`
**说明**: 当请求超时时，是否允许消息发送

| 值 | 行为 | 适用场景 |
|---|------|---------|
| `true` | 超时后**允许发送** | 审核系统非关键，优先确保消息投递 |
| `false` | 超时后**拒绝发送** | 安全审核关键，宁可拒绝也不冒险 |

#### 5. Retries（可选）

**类型**: Integer
**默认值**: `3`
**范围**: `0` - `10`
**说明**: 请求失败时的重试次数

**重试策略**：
- `0`: 不重试，单次失败立即返回
- `1 - 3`: 推荐值，平衡可靠性和性能
- `3 - 5`: 用于不稳定的网络环境
- `> 5`: 不推荐，可能导致长时间等待

### 配置示例

#### 示例1：严格安全审核

```json
{
  "CallbackUrl": "https://security.example.com/audit",
  "AppSecret": "super-secret-key-abc123def456",
  "Timeout": 10,
  "TimeoutSend": false,
  "Retries": 5
}
```

**特点**：
- 较长的超时时间允许更复杂的审核逻辑
- 超时后拒绝发送，确保安全
- 多次重试确保可靠性

#### 示例2：性能优先

```json
{
  "CallbackUrl": "https://api.example.com/quick-check",
  "AppSecret": "app-secret-key-123",
  "Timeout": 3,
  "TimeoutSend": true,
  "Retries": 1
}
```

**特点**：
- 短超时时间，快速响应
- 超时后允许发送，优先可用性
- 少量重试，减少延迟

#### 示例3：平衡方案

```json
{
  "CallbackUrl": "https://api.example.com/callback",
  "AppSecret": "balanced-secret-key",
  "Timeout": 5,
  "TimeoutSend": false,
  "Retries": 3
}
```

**特点**：
- 中等超时时间，兼顾审核和性能
- 超时后拒绝发送，确保安全性
- 适度重试，保证可靠性

---

## 使用示例

### 场景1：内容安全审核

**业务需求**：在消息发送前进行内容安全检查（违禁词、敏感内容等）

**第三方应用实现**：

``` python
from flask import Flask, request
import hashlib
import json

app = Flask(__name__)
APP_SECRET = "your-app-secret-key"

def verify_signature(headers, body_str):
    """验证签名"""
    checksum = headers.get('CheckSum')
    md5 = headers.get('MD5')
    cur_time = headers.get('CurTime')

    to_sign = APP_SECRET + md5 + cur_time
    calculated = hashlib.sha1(to_sign.encode()).hexdigest()

    return checksum.lower() == calculated.lower()

@app.route('/callback', methods=['POST'])
def handle_callback():
    # 1. 验证签名
    body_str = request.get_data(as_text=True)
    if not verify_signature(request.headers, body_str):
        return {'error': 'Invalid signature'}, 401

    # 2. 解析请求
    msg = json.loads(body_str)
    content = msg['msgBody']

    # 3. 执行审核逻辑
    if is_sensitive_content(content):
        return {'allow': False}, 200  # 拒绝发送

    # 4. 返回响应
    return {'allow': True}, 200

def is_sensitive_content(text):
    """简单的内容检查示例"""
    banned_words = ['spam', 'abuse']
    return any(word in text.lower() for word in banned_words)
```

**插件配置**：
```json
{
  "CallbackUrl": "https://yourserver.com/callback",
  "AppSecret": "your-app-secret-key",
  "Timeout": 5,
  "TimeoutSend": false,
  "Retries": 3
}
```

### 场景2：消息内容修改

**业务需求**：自动将某些内容替换为合规的表述

**第三方应用实现**：

```javascript
const express = require('express');
const crypto = require('crypto');
const base64 = require('base64-js');

const app = express();
const APP_SECRET = 'your-app-secret-key';

function verifySignature(headers, bodyStr) {
  const checksum = headers['checksum'];
  const md5 = headers['md5'];
  const curTime = headers['curtime'];

  const toSign = APP_SECRET + md5 + curTime;
  const calculated = crypto.createHash('sha1').update(toSign).digest('hex');

  return checksum.toLowerCase() === calculated.toLowerCase();
}

app.post('/callback', express.json(), (req, res) => {
  // 1. 验证签名
  const bodyStr = JSON.stringify(req.body);
  if (!verifySignature(req.headers, bodyStr)) {
    return res.status(401).json({ error: 'Invalid signature' });
  }

  // 2. 解析请求
  const msg = req.body;
  let content = msg.msgBody;

  // 3. 执行修改逻辑
  const modified = modifyContent(content);

  // 4. 如果内容被修改，需要Base64编码返回
  if (modified !== content) {
    const base64Encoded = Buffer.from(modified).toString('base64');
    return res.json({
      allow: true,
      msgBody: base64Encoded
    });
  }

  // 5. 返回响应
  res.json({ allow: true });
});

function modifyContent(text) {
  // 替换不当词汇
  return text
    .replace(/bad_word/g, '***')
    .replace(/sensitive/g, '[已隐藏]');
}

app.listen(3000);
```

**插件配置**：
```json
{
  "CallbackUrl": "https://yourserver.com/callback",
  "AppSecret": "your-app-secret-key",
  "Timeout": 5,
  "TimeoutSend": true,
  "Retries": 2
}
```

### 场景3：消息日志记录

**业务需求**：记录所有消息用于审计追踪

**第三方应用实现**：

```go
package main

import (
    "crypto/md5"
    "crypto/sha1"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

const AppSecret = "your-app-secret-key"

type MessageLog struct {
    Timestamp   int64
    FromUID     string
    ChannelID   string
    Content     string
    DeviceFlag  uint8
    LogID       string
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. 验证签名
    body, _ := io.ReadAll(r.Body)
    if !verifySignature(r.Header, string(body)) {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "Invalid signature"})
        return
    }

    // 2. 解析请求
    var msg map[string]interface{}
    json.Unmarshal(body, &msg)

    // 3. 生成日志
    log := MessageLog{
        Timestamp:  time.Now().UnixMilli(),
        FromUID:    msg["fromUid"].(string),
        ChannelID:  msg["channelId"].(string),
        Content:    msg["msgBody"].(string),
        DeviceFlag: uint8(msg["deviceFlag"].(float64)),
        LogID:      generateLogID(),
    }

    // 4. 存储日志
    saveLog(log)

    // 5. 返回响应
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]bool{"allow": true})
}

func verifySignature(headers http.Header, body string) bool {
    checksum := headers.Get("CheckSum")
    md5Val := headers.Get("MD5")
    curTime := headers.Get("CurTime")

    toSign := AppSecret + md5Val + curTime
    sha1Hash := sha1.Sum([]byte(toSign))
    calculated := hex.EncodeToString(sha1Hash[:])

    return strings.EqualFold(checksum, calculated)
}

func calculateMD5(data string) string {
    hash := md5.Sum([]byte(data))
    return hex.EncodeToString(hash[:])
}

func generateLogID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

func saveLog(log MessageLog) {
    // 实现数据库或文件存储
    fmt.Printf("Saved log: %+v\n", log)
}

func main() {
    http.HandleFunc("/callback", handleCallback)
    http.ListenAndServe(":8080", nil)
}
```

**插件配置**：
```json
{
  "CallbackUrl": "https://yourserver.com/callback",
  "AppSecret": "your-app-secret-key",
  "Timeout": 3,
  "TimeoutSend": true,
  "Retries": 1
}
```

---

## 常见问题

### Q1: 消息会丢失吗？

**A**: 取决于超时配置：

- **TimeoutSend = true**: 超时后允许发送，消息不会丢失
- **TimeoutSend = false**: 超时后拒绝发送，消息将被拦截

建议在开发环境设置为 `true` 便于测试，生产环境根据业务需求选择。

### Q2: 如何调试签名验证失败？

**A**: 按以下步骤排查：

```python
import hashlib

# 1. 验证 MD5
request_body = '{"msgBody":"test"}'
expected_md5 = hashlib.md5(request_body.encode()).hexdigest()
actual_md5 = request_headers['MD5']
print(f"MD5 Match: {expected_md5 == actual_md5}")

# 2. 验证 CurTime
# 应该是当前UTC时间戳（毫秒），误差不应超过 5 秒

# 3. 验证 CheckSum
app_secret = 'your-secret'
to_sign = app_secret + actual_md5 + actual_curtime
expected_checksum = hashlib.sha1(to_sign.encode()).hexdigest()
actual_checksum = request_headers['CheckSum']
print(f"CheckSum Match: {expected_checksum.lower() == actual_checksum.lower()}")
```

### Q3: 可以修改哪些字段？

**A**: 根据API规范，第三方应用只能修改 `msgBody`（消息内容）。

其他字段如发送者ID、接收者、频道ID等是悟空IM内部使用，不支持修改。

### Q4: 消息修改时为什么要用Base64编码？

**A**: Base64编码有以下好处：

- **安全性**: 避免二进制数据破坏JSON结构
- **兼容性**: 支持任意编码的消息（UTF-8、GBK等）
- **完整性**: 确保特殊字符不被转义或丢失

示例：
```python
import base64

original = "This is a message with special chars: 你好 🎉"
encoded = base64.b64encode(original.encode()).decode()
print(encoded)  # "VGhpcyBpcyBhIG1lc3NhZ2Ugd2l0aCBzcGVjaWFsIGNoYXJzOiDkvaDlpb0g8J+OiQ=="
```

### Q5: 如何处理网络不稳定的情况？

**A**: 配置以下参数：

```json
{
  "Timeout": 10,           // 增加超时时间
  "TimeoutSend": true,     // 超时后允许发送
  "Retries": 5             // 增加重试次数
}
```

同时在第三方应用实现幂等性逻辑，使用消息ID去重：

```python
# 使用消息ID作为唯一键，避免重复处理
message_id = f"{msg['fromUid']}_{msg['channelId']}_{timestamp}"
if is_processed(message_id):
    return {'allow': True}  # 重复请求，直接允许

process_message(msg)
mark_as_processed(message_id)
```

### Q6: 插件的性能如何？

**A**: 影响性能的主要因素：

| 因素 | 影响 | 优化建议 |
|------|------|--------|
| Timeout | 每个消息最多等待N秒 | 根据网络状况设置，通常3-5秒 |
| Retries | 最多调用N+1次 | 3次重试通常足够 |
| 第三方应用响应速度 | 直接影响消息发送延迟 | 优化业务逻辑，使用缓存 |

**建议指标**：
- 平均响应时间: 100-500ms
- P95响应时间: < 2s
- 成功率: > 99.5%

### Q7: 如何监控插件的工作状态？

**A**: 通过以下方式：

1. **查看悟空IM日志**：
   ```bash
   tail -f /path/to/wukongim.log | grep "third.msg.callback"
   ```

2. **监控第三方应用日志**：记录所有回调请求和响应

3. **设置告警**：
   - 回调失败率超过5%
   - 平均响应时间超过2秒
   - 重试次数过多

4. **定期测试**：
   ```bash
   # 定时发送测试消息，验证回调功能
   curl -X POST http://wukongim:8000/send \
     -H "Content-Type: application/json" \
     -d '{"content":"test","to":"testuser"}'
   ```

---

## 故障排查

### 问题1: "插件加载失败"

**可能原因**：
- [ ] 插件文件损坏或平台不匹配
- [ ] 悟空IM版本不兼容
- [ ] 权限不足

**解决方案**：
```bash
# 检查插件完整性
file plugin.wkp

# 重新编译
go clean -cache
go build -o plugin.wkp main.go

# 查看详细日志
tail -f wukongim.log | grep -i error
```

### 问题2: "请求超时"

**可能原因**：
- [ ] 第三方服务器响应慢
- [ ] 网络延迟高
- [ ] 超时时间设置过短

**解决方案**：
```json
{
  "Timeout": 10,        // 增加超时时间
  "TimeoutSend": true   // 允许超时后发送
}
```

### 问题3: "签名验证失败"

**可能原因**：
- [ ] AppSecret不匹配
- [ ] MD5计算错误
- [ ] 时间差过大

**解决方案**：
```python
# 打印调试信息
print(f"AppSecret: {APP_SECRET}")
print(f"Received MD5: {headers['MD5']}")
print(f"Calculated MD5: {hashlib.md5(body).hexdigest()}")
print(f"Received CheckSum: {headers['CheckSum']}")

# 重新计算 CheckSum
to_sign = APP_SECRET + headers['MD5'] + headers['CurTime']
expected_checksum = hashlib.sha1(to_sign.encode()).hexdigest()
print(f"Expected CheckSum: {expected_checksum}")
```

### 问题4: "消息发送被拦截"

**可能原因**：
- [ ] 第三方应用返回 `allow: false`
- [ ] 超时且 `TimeoutSend: false`
- [ ] 异常错误

**解决方案**：
1. 查看第三方应用日志，确认拒绝原因
2. 修改业务逻辑或规则
3. 更新插件配置
4. 发送测试消息验证

---

## 📚 相关资源

### 官方文档

- [悟空IM官方网站](https://wukongim.github.io/)
- [悟空IM Go Plugin SDK](https://github.com/WuKongIM/go-pdk)
- [悟空IM协议定义](https://github.com/WuKongIM/WuKongIMGoProto)
- [插件安装教程](https://githubim.com/server/plugin/use.html)

 
### 技术支持
- [GitHub Issues](https://github.com/zuoliang0/wukong-plugins-third-callback)
 

---

## 📄 许可证

本项目采用 Apache License 2.0 许可证。详见 [LICENSE](LICENSE) 文件。

---

**最后更新**: 2025-11-18
**维护者**: 左良
