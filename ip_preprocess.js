// 简化版IP格式化脚本
import fs from "node:fs";
import path from "node:path";
import url from "node:url";

const __filename = url.fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function parseIPAndPort(line) {
    line = line.trim();
    if (!line || line.startsWith("#") || line.startsWith("//")) return null;

    let ip = "", port = "";

    // IP:端口#国家描述
    if (line.includes(":") && line.includes("#")) {
        const beforeHash = line.split("#")[0].trim();
        const parts = beforeHash.split(":");
        if (parts.length === 2) {
            ip = parts[0].trim();
            port = parts[1].trim();
        }
    }
    // IP:端口
    else if (line.includes(":")) {
        const parts = line.split(":");
        if (parts.length === 2) {
            ip = parts[0].trim();
            port = parts[1].trim();
        }
    }
    // IP 端口
    else {
        const parts = line.split(/\s+/);
        if (parts.length >= 2) {
            ip = parts[0].trim();
            port = parts[1].trim();
        }
    }

    // 验证IP
    const ipParts = ip.split(".");
    if (ipParts.length !== 4) return null;

    for (const part of ipParts) {
        const num = parseInt(part);
        if (isNaN(num) || num < 0 || num > 255) return null;
    }

    // 验证端口
    const portNum = parseInt(port);
    if (isNaN(portNum) || portNum < 1 || portNum > 65535) return null;

    return `${ip} ${portNum}`;
}

async function processFile(inputFile, outputFile) {
    try {
        const data = await fs.promises.readFile(inputFile, "utf8");
        const lines = data.split("\n").map(line => line.trim()).filter(line => line);

        console.log(`开始处理: ${inputFile}`);
        console.log(`总行数: ${lines.length}`);

        const uniqueIPs = new Set();
        let processed = 0, skipped = 0;

        for (const line of lines) {
            const result = parseIPAndPort(line);
            if (result) {
                uniqueIPs.add(result);
                processed++;
            } else {
                skipped++;
            }
        }

        const result = Array.from(uniqueIPs).sort().join("\n");
        await fs.promises.writeFile(outputFile, result, "utf8");

        console.log(`✅ 处理完成!`);
        console.log(`成功: ${processed}, 跳过: ${skipped}, 去重后: ${uniqueIPs.size}`);
        console.log(`输出: ${outputFile}`);

        return outputFile;
    } catch (error) {
        console.error("❌ 错误:", error.message);
        throw error;
    }
}

// 命令行处理
const args = process.argv.slice(2);
if (args.length >= 2) {
    await processFile(args[0], args[1]);
} else {
    console.log("用法: node ip_preprocess.js <输入文件> <输出文件>");
}