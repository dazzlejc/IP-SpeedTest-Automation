// 增强版IP格式化脚本 - 支持多种格式，自动去重
import fs from "node:fs";
import path from "node:path";
import url from "node:url";

// 获取当前脚本路径
const __filename = url.fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// 处理函数 - 支持命令行参数
async function processFile(inputFile, outputFile = "processed_ips.txt") {
    try {
        const inputPath = path.resolve(__dirname, inputFile);
        const outputPath = path.resolve(__dirname, outputFile);

        // 检查输入文件是否存在
        if (!fs.existsSync(inputPath)) {
            throw new Error(`输入文件不存在: ${inputFile}`);
        }

        // 读取文件内容
        const data = await fs.promises.readFile(inputPath, "utf8");

        // 按行分割并清理
        const lines = data
            .split("\n")
            .map(line => line.trim())
            .filter(line => line && !line.startsWith("#")); // 去掉空行和注释行

        if (lines.length === 0) {
            throw new Error("文件内容为空");
        }

        console.log(`开始处理文件: ${inputFile}`);
        console.log(`总行数: ${lines.length}`);

        // 使用Set进行去重
        const uniqueIPs = new Set();
        let processedCount = 0;
        let skippedCount = 0;

        for (const line of lines) {
            const result = parseIPAndPort(line);
            if (result) {
                uniqueIPs.add(result);
                processedCount++;
            } else {
                skippedCount++;
            }
        }

        // 转换为数组并排序（可选）
        const sortedIPs = Array.from(uniqueIPs).sort((a, b) => {
            const [ipA, portA] = a.split(" ");
            const [ipB, portB] = b.split(" ");

            // 按IP地址排序
            const ipPartsA = ipA.split(".").map(Number);
            const ipPartsB = ipB.split(".").map(Number);

            for (let i = 0; i < 4; i++) {
                if (ipPartsA[i] !== ipPartsB[i]) {
                    return ipPartsA[i] - ipPartsB[i];
                }
            }

            // IP相同则按端口排序
            return parseInt(portA) - parseInt(portB);
        });

        // 写入文件
        const result = sortedIPs.join("\n");
        await fs.promises.writeFile(outputPath, result, "utf8");

        console.log(`✅ 处理完成!`);
        console.log(`📊 统计信息:`);
        console.log(`   - 原始行数: ${lines.length}`);
        console.log(`   - 成功处理: ${processedCount}`);
        console.log(`   - 跳过无效: ${skippedCount}`);
        console.log(`   - 去重后数量: ${uniqueIPs.size}`);
        console.log(`📁 输出文件: ${outputPath}`);

        return outputPath;

    } catch (error) {
        console.error("❌ 处理文件时发生错误:", error.message);
        throw error;
    }
}

// 解析IP和端口 - 支持多种格式
function parseIPAndPort(line) {
    // 清理行内容
    line = line.trim();

    // 跳过注释行和空行
    if (!line || line.startsWith("#") || line.startsWith("//")) {
        return null;
    }

    let ip = "";
    let port = "";

    try {
        // 格式1: IP:端口#国家描述 (如: 103.20.199.122:443#🇯🇵日本24)
        if (line.includes(":") && line.includes("#")) {
            const beforeHash = line.split("#")[0].trim();
            const parts = beforeHash.split(":");
            if (parts.length === 2) {
                ip = parts[0].trim();
                port = parts[1].trim();
            }
        }
        // 格式2: IP:端口 (如: 103.20.199.122:443)
        else if (line.includes(":") && !line.includes("#")) {
            const parts = line.split(":");
            if (parts.length === 2) {
                ip = parts[0].trim();
                port = parts[1].trim();
            }
        }
        // 格式3: IP 端口 (空格分隔)
        else if (line.includes(" ") && !line.includes(":")) {
            const parts = line.split(/\s+/);
            if (parts.length >= 2) {
                ip = parts[0].trim();
                port = parts[1].trim();
            }
        }
        // 格式4: CSV格式 - 处理可能的引号
        else if (line.includes(",")) {
            const parts = line.split(",").map(p => p.replace(/['"]/g, '').trim());
            if (parts.length >= 2) {
                ip = parts[0].trim();
                port = parts[1].trim();
            }
        }

        // 验证IP地址格式
        if (!isValidIP(ip)) {
            console.log(`⚠️  无效IP地址: ${ip}`);
            return null;
        }

        // 验证端口格式
        const portNum = parseInt(port);
        if (!portNum || portNum < 1 || portNum > 65535) {
            console.log(`⚠️  无效端口号: ${port}`);
            return null;
        }

        return `${ip} ${portNum}`;

    } catch (error) {
        console.log(`⚠️  解析失败: ${line} - ${error.message}`);
        return null;
    }
}

// 验证IP地址格式
function isValidIP(ip) {
    if (!ip) return false;

    const parts = ip.split(".");
    if (parts.length !== 4) return false;

    for (const part of parts) {
        const num = parseInt(part);
        if (isNaN(num) || num < 0 || num > 255) {
            return false;
        }
    }

    return true;
}

// 主函数
async function main() {
    const args = process.argv.slice(2);

    if (args.length === 0) {
        console.log(`
🔧 增强版IP格式化工具

用法:
  node ip_init_enhanced.js <输入文件> [输出文件]

示例:
  node ip_init_enhanced.js merged_proxies.txt
  node ip_init_enhanced.js ip_list.csv processed.txt
  node ip_init_enhanced.js mixed_format.txt final_ips.txt

支持的格式:
  • IP:端口#国家描述    (如: 103.20.199.122:443#🇯🇵日本24)
  • IP:端口            (如: 103.20.199.122:443)
  • IP 端口            (如: 103.20.199.122 443)
  • CSV格式           (如: "103.20.199.122","443")

功能特点:
  • 自动识别多种格式
  • 智能去重
  • IP地址验证
  • 端口范围检查
  • 结果排序
  • 详细统计信息
        `);
        return;
    }

    const inputFile = args[0];
    const outputFile = args[1] || "processed_ips.txt";

    try {
        await processFile(inputFile, outputFile);
    } catch (error) {
        process.exit(1);
    }
}

// 如果直接运行此脚本
if (import.meta.url === `file://${process.argv[1]}`) {
    main();
}

// 导出函数供其他模块使用
export { processFile, parseIPAndPort, isValidIP };