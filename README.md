# IP Speed Test Automation (IP测速自动化工具)

<div align="center">

![GitHub license](https://img.shields.io/github/license/dazzlejc/IP-SpeedTest-Automation)
![GitHub stars](https://img.shields.io/github/stars/dazzlejc/IP-SpeedTest-Automation?style=social)
![GitHub forks](https://img.shields.io/github/forks/dazzlejc/IP-SpeedTest-Automation?style=social)

一个功能强大的IP测速工具，支持批量IP延迟测试、下载速度测试、地理位置识别和结果分析

[功能特性](#功能特性) • [快速开始](#快速开始) • [使用指南](#使用指南) • [API文档](#api文档) • [贡献指南](#贡献指南)

</div>

## 📋 目录

- [功能特性](#功能特性)
- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [详细使用指南](#详细使用指南)
- [配置说明](#配置说明)
- [输出格式](#输出格式)
- [API集成](#api集成)
- [故障排除](#故障排除)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

## ✨ 功能特性

### 🚀 核心功能
- **批量IP测试**: 支持大规模IP地址并发测试
- **延迟检测**: TCP连接延迟测试，支持阈值过滤
- **下载测速**: 多线程下载速度测试，可配置测速文件
- **地理位置识别**: 自动获取IP地理位置信息（支持中英文）
- **交互式界面**: 友好的命令行交互界面
- **批量处理**: 支持文件预处理和格式转换

### 🛠️ 高级特性
- **智能过滤**: 延迟阈值和速度阈值双重过滤
- **多协议支持**: HTTP/HTTPS协议切换
- **结果导出**: CSV格式详细结果导出
- **API集成**: 支持结果上传到自定义API
- **跨平台**: Windows、Linux、macOS全平台支持
- **实时监控**: 测试进度实时显示

## 🔧 环境要求

### 系统要求
- **操作系统**: Windows 10+, Linux (Ubuntu 18.04+), macOS 10.15+
- **内存**: 最少512MB，推荐1GB+
- **网络**: 稳定的互联网连接

### 运行环境依赖
```bash
# Go环境 (用于编译主程序)
go version 1.18+  # 必需

# Node.js环境 (用于辅助脚本)
node version 14+   # 可选，用于文件预处理
npm version 6+     # 可选

# 系统工具
git               # 用于代码管理
```

## 🚀 快速开始

### 1. 克隆仓库
```bash
git clone https://github.com/dazzlejc/IP-SpeedTest-Automation.git
cd IP-SpeedTest-Automation
```

### 2. 安装依赖
```bash
# 安装Go依赖
go mod tidy

# 安装Node.js依赖 (可选)
npm install
```

### 3. 编译程序
```bash
# 编译主程序
go build -o iptest iptest.go

# 或者直接运行
go run iptest.go
```

### 4. 快速测试
```bash
# 交互式运行 (推荐)
./iptest

# 或者命令行模式
./iptest -file=ip.txt -outfile=result.csv
```

## 📖 详细使用指南

### 交互式模式 (推荐)

直接运行程序进入交互式界面：

```bash
./iptest
```

交互式菜单提供以下选项：
```
=== IP测速工具 ===
1. 扫描本地文件测速
2. 从API下载测速
3. 上传测速结果
4. 测速参数设置
5. 退出
```

### 命令行模式

#### 基础命令
```bash
# 基本测试
./iptest -file=ip.txt -outfile=result.csv

# 启用下载测速
./iptest -file=ip.txt -outfile=result.csv -speedtest=10

# 设置延迟阈值
./iptest -file=ip.txt -outfile=result.csv -delay=150

# 禁用TLS
./iptest -file=ip.txt -outfile=result.csv -tls=false
```

#### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-file` | `ip.txt` | IP地址文件路径，格式为每行 `IP 端口` |
| `-outfile` | `ip.csv` | 输出CSV文件路径 |
| `-max` | `100` | 最大并发协程数 |
| `-speedtest` | `5` | 下载测速协程数量，设为`0`禁用测速 |
| `-url` | `speed.cloudflare.com/__down?bytes=500000000` | 测速文件地址 |
| `-tls` | `true` | 是否启用TLS (`true`=HTTPS, `false`=HTTP) |
| `-delay` | `300` | 延迟阈值(毫秒)，超过此值的IP将被过滤 |
| `-speedthreshold` | `3.0` | 速度阈值(MB/s)，低于此值的IP将被过滤 |
| `-upload` | `""` | 上传API地址，留空则不上传 |
| `-token` | `""` | 上传API认证令牌 |

### Node.js辅助脚本

#### 文件预处理
```bash
# 预处理文件，格式化并去重
node ip_preprocess.js input.txt output.txt

# 批量获取初始IP
node ip_init.js

# 提取有效IP
node ip_tq.js
```

#### 数据处理
```bash
# 合并CSV文件
node csv_hb.js init.csv file1.csv file2.csv

# 合并目录下所有文件
node csv_hb.js init.csv ./ip_directory
```

## ⚙️ 配置说明

### IP文件格式

支持的输入格式：
```
# 标准格式 (推荐)
1.1.1.1 443
2.2.2.2 2053

# IP:端口格式
3.3.3.3:8443

# 带注释格式
4.4.4.4 443  # 这是一个注释
```

### 配置文件

程序会自动下载地理位置数据文件 `locations.json`，包含：
- 机场代码映射
- 地理位置信息
- 多语言支持
- 国旗emoji

### 性能调优建议

```bash
# 高并发测试 (推荐配置)
./iptest -max=200 -speedtest=20 -delay=200

# 快速筛选 (仅延迟测试)
./iptest -speedtest=0 -delay=150

# 精确测速 (低并发高精度)
./iptest -max=50 -speedtest=10 -speedthreshold=5.0
```

## 📊 输出格式

### CSV输出字段

| 字段 | 说明 |
|------|------|
| IP地址 | 测试的IP地址 |
| 端口 | 测试的端口号 |
| TLS | 是否启用TLS连接 |
| 数据中心 | Cloudflare数据中心代码 |
| 源IP位置 | 源IP位置代码 |
| 地区 | 地区名称 (英文) |
| 城市 | 城市名称 (英文) |
| 地区(中文) | 地区名称 (中文) |
| 国家 | 国家名称 (英文) |
| 城市(中文) | 城市名称 (中文) |
| 国旗 | 国家国旗emoji |
| 网络延迟 | TCP连接延迟 (毫秒) |
| 下载速度(MB/s) | 下载速度 (启用测速时) |

### 示例输出
```csv
IP地址,端口,TLS,数据中心,源IP位置,地区,城市,地区(中文),国家,城市(中文),国旗,网络延迟,下载速度(MB/s)
1.1.1.1,443,true,SIN,SIN,Asia Pacific,Singapore,亚太地区,Singapore,新加坡🇸🇬,85,25.67
2.2.2.2,2053,true,HKG,HKG,Asia Pacific,Hong Kong,亚太地区,Hong Kong,香港🇭🇰,92,18.34
```

## 🔌 API集成

### 上传测速结果

程序支持将测速结果上传到自定义API：

```bash
# 配置上传API
./iptest -upload="https://your-api.com/upload" -token="your-token"
```

### 上传格式

上传数据格式为纯文本，每行一个IP：
```
IP:端口#城市(中文)
1.1.1.1:443#新加坡🇸🇬
2.2.2.2:2053#香港🇭🇰
```

### API要求

- **方法**: POST
- **Content-Type**: text/plain; charset=utf-8
- **认证**: Bearer Token (可选)
- **响应**: 2xx状态码表示成功

## 🔍 故障排除

### 常见问题

#### 1. 编译错误
```bash
# 确保Go版本正确
go version

# 清理模块缓存
go clean -modcache
go mod download
```

#### 2. 网络连接问题
```bash
# 检查防火墙设置
# 确保能访问 cloudflare.com

# 测试基础连接
ping 1.1.1.1
```

#### 3. 文件权限问题
```bash
# Linux/macOS
chmod +x iptest

# 确保有写入权限
ls -la locations.json
```

#### 4. 内存不足
```bash
# 减少并发数
./iptest -max=50 -speedtest=2

# 禁用下载测速
./iptest -speedtest=0
```

### 调试模式

启用详细输出：
```bash
# 查看当前设置
./iptest
# 选择选项4查看当前配置

# 测试单个IP
echo "1.1.1.1 443" > test.txt
./iptest -file=test.txt -max=1 -speedtest=1
```

## 🤝 贡献指南

我们欢迎所有形式的贡献！

### 贡献方式

1. **报告问题**: 在[Issues](https://github.com/dazzlejc/IP-SpeedTest-Automation/issues)中报告bug
2. **功能建议**: 提出新功能建议
3. **代码贡献**: 提交Pull Request
4. **文档改进**: 改进文档和示例

### 开发流程

1. Fork项目
2. 创建功能分支: `git checkout -b feature/AmazingFeature`
3. 提交更改: `git commit -m 'Add some AmazingFeature'`
4. 推送分支: `git push origin feature/AmazingFeature`
5. 提交Pull Request

### 代码规范

- Go代码遵循标准Go格式化规范
- JavaScript代码使用Prettier格式化
- 提交信息使用约定式提交格式
- 添加适当的注释和文档

## 📄 许可证

本项目采用 **GNU General Public License v3.0** 许可证。

<div align="center">

### 🙏 致谢

感谢以下开源项目和贡献者：

- **原项目**: [Kwisma/iptest](https://github.com/Kwisma/iptest) - 本项目基于原项目进行改进和扩展
- [Cloudflare](https://www.cloudflare.com/) - 提供优秀的CDN服务
- [XIU2](https://github.com/XIU2) - 原始项目灵感来源
- [yutian81](https://github.com/yutian81) - 延迟过滤功能参考

特别感谢原项目作者 **Kwisma** 提供的优秀代码基础，本项目在此基础上增加了交互式界面、API集成、性能优化等功能改进。

### 📞 联系方式

- 项目主页: [https://github.com/dazzlejc/IP-SpeedTest-Automation](https://github.com/dazzlejc/IP-SpeedTest-Automation)
- 问题反馈: [Issues](https://github.com/dazzlejc/IP-SpeedTest-Automation/issues)

---

**如果这个项目对您有帮助，请给我们一个 ⭐ Star！**

</div>