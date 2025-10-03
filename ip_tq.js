import fs from "node:fs/promises";
import path from "node:path";
import url from "node:url";

// 配置参数
const CONFIG = {
  perCountryCount: 5,      // 每个国家提取数量，0表示不限制
  filterBySpeed: false,    // 是否过滤下载速度
  minSpeed: 0,             // 过滤下载速度下限，单位kb/s
  targetFile: "ip_tq.csv"  // 指定要处理的CSV文件名
};

// CSV 列名
const COLUMNS = {
  ip: "IP地址",
  port: "端口",
  speed: "下载速度",
  datacenter: "数据中心",
  bronIpLocatie: "源IP位置"
};

class CSVProcessor {
  constructor(config = CONFIG) {
    this.config = config;
    this.locations = null;
    this.scriptDir = path.dirname(url.fileURLToPath(import.meta.url));
  }

  async process() {
    try {
      const csvFilePath = this.getFilePath(this.config.targetFile);
      const txtFilePath = this.getFilePath(this.config.targetFile.replace(".csv", ".txt"));

      await this.validateFileExists(csvFilePath);
      console.log(`开始处理文件: ${this.config.targetFile}`);
      
      await this.loadLocations();
      await this.processCSV(csvFilePath, txtFilePath);
      
    } catch (error) {
      this.handleError("处理文件时发生错误", error);
    }
  }

  getFilePath(filename) {
    return path.resolve(this.scriptDir, filename);
  }

  async validateFileExists(filePath) {
    try {
      await fs.access(filePath);
    } catch {
      throw new Error(`未找到指定的文件: ${path.basename(filePath)}`);
    }
  }

  async loadLocations() {
    try {
      const jsonFilePath = this.getFilePath("locations.json");
      const jsonData = await fs.readFile(jsonFilePath, "utf8");
      this.locations = JSON.parse(jsonData);
      console.log("位置数据加载成功。");
    } catch (error) {
      throw new Error(`加载位置数据失败: ${error.message}`);
    }
  }

  isIPv6(ip) {
    return ip.includes(":");
  }

  formatIPv6(ip) {
    return this.isIPv6(ip) && !ip.startsWith("[") ? `[${ip}]` : ip;
  }

  getCountryFromLocationData(datacenterCode, bronIpLocatie) {
    if (!this.locations) return "Unknown";

    // 优先查找同时匹配 datacenterCode 和 bronIpLocatie 的 location
    const exactMatch = this.locations.find(loc => 
      loc.iata === datacenterCode && loc.cca2 === bronIpLocatie
    );
    
    if (exactMatch) {
      return `${exactMatch.emoji}${exactMatch.country}`;
    }
    
    // 如果没有精确匹配，则只匹配 bronIpLocatie
    const countryMatch = this.locations.find(loc => 
      loc.cca2 === bronIpLocatie
    );
    
    return countryMatch ? `${countryMatch.emoji}${countryMatch.country}` : "Unknown";
  }

  parseCSVLine(line) {
    // 简单的CSV解析，处理可能包含逗号的字段
    const result = [];
    let inQuotes = false;
    let currentField = "";

    for (let i = 0; i < line.length; i++) {
      const char = line[i];
      
      if (char === '"' && (i === 0 || line[i - 1] !== '\\')) {
        inQuotes = !inQuotes;
      } else if (char === ',' && !inQuotes) {
        result.push(currentField.trim());
        currentField = "";
      } else {
        currentField += char;
      }
    }
    
    result.push(currentField.trim());
    return result;
  }

  shouldIncludeBySpeed(speedField) {
    if (!this.config.filterBySpeed || !speedField) return true;
    
    const speedValue = parseFloat(speedField.replace(" kB/s", ""));
    return !isNaN(speedValue) && speedValue >= this.config.minSpeed;
  }

  async processCSV(csvFilePath, txtFilePath) {
    console.log(`开始读取 CSV 文件: ${path.basename(csvFilePath)}`);
    
    const data = await fs.readFile(csvFilePath, "utf8");
    const lines = data.split("\n").map(line => line.trim()).filter(Boolean);
    
    if (lines.length < 2) {
      throw new Error("CSV 文件内容不足或格式不正确");
    }

    // 解析表头
    const headers = this.parseCSVLine(lines[0]);
    const indices = this.getColumnIndices(headers);

    // 处理数据行
    const ipEntries = this.processDataLines(lines.slice(1), indices);
    
    if (ipEntries.length === 0) {
      console.log("没有符合条件的IP记录");
      return;
    }

    // 分组并应用数量限制
    const groupedEntries = this.groupAndLimitEntries(ipEntries);
    
    if (Object.keys(groupedEntries).length === 0) {
      console.log("没有国家满足数量要求");
      return;
    }

    // 生成并保存结果
    await this.generateAndSaveResult(groupedEntries, txtFilePath);
  }

  getColumnIndices(headers) {
    const indices = {};
    const requiredColumns = [COLUMNS.ip, COLUMNS.port, COLUMNS.datacenter, COLUMNS.bronIpLocatie];
    
    for (const col of requiredColumns) {
      indices[col] = headers.indexOf(col);
      if (indices[col] === -1) {
        throw new Error(`CSV 文件缺少 ${col} 列`);
      }
    }
    
    // 速度列是可选的
    indices[COLUMNS.speed] = headers.indexOf(COLUMNS.speed);
    
    return indices;
  }

  processDataLines(lines, indices) {
    const ipEntries = [];
    
    for (const line of lines) {
      if (!line) continue;
      
      const fields = this.parseCSVLine(line);
      if (fields.length <= Math.max(...Object.values(indices).filter(i => i !== -1))) {
        continue; // 跳过列数不足的行
      }

      // 速度过滤
      if (indices[COLUMNS.speed] !== -1 && !this.shouldIncludeBySpeed(fields[indices[COLUMNS.speed]])) {
        continue;
      }

      // 提取和格式化数据
      const ip = this.formatIPv6(fields[indices[COLUMNS.ip]]);
      const port = fields[indices[COLUMNS.port]];
      const datacenter = fields[indices[COLUMNS.datacenter]];
      const bronIpLocatie = fields[indices[COLUMNS.bronIpLocatie]];
      
      const country = this.getCountryFromLocationData(datacenter, bronIpLocatie);
      
      ipEntries.push({
        entry: `${ip}:${port}#${country}`,
        country
      });
    }
    
    console.log(`IP 和端口提取完成。共 ${ipEntries.length} 条记录`);
    return ipEntries;
  }

  groupAndLimitEntries(ipEntries) {
    const grouped = {};
    const countryCounts = {};
    
    for (const { entry, country } of ipEntries) {
      if (!grouped[country]) {
        grouped[country] = [];
        countryCounts[country] = 0;
      }

      if (this.config.perCountryCount === 0 || countryCounts[country] < this.config.perCountryCount) {
        grouped[country].push(entry);
        countryCounts[country]++;
      }
    }
    
    // 过滤掉数量不足的国家
    return Object.fromEntries(
      Object.entries(grouped).filter(([country, entries]) => 
        this.config.perCountryCount === 0 || entries.length >= this.config.perCountryCount
      )
    );
  }

  async generateAndSaveResult(groupedEntries, txtFilePath) {
    const validCountries = Object.keys(groupedEntries).sort();
    
    const result = validCountries
      .map(country => 
        groupedEntries[country]
          .map((entry, index) => `${entry}${index + 1}`)
          .join("\n")
      )
      .join("\n");

    console.log(`提取国家: ${validCountries.join("、")} (共 ${validCountries.length} 个国家)`);
    
    await fs.writeFile(txtFilePath, result, "utf8");
    console.log(`已成功保存到 ${path.basename(txtFilePath)}`);
  }

  handleError(context, error) {
    console.error(`${context}: ${error.message}`);
    if (error.stack) {
      console.error(error.stack);
    }
  }
}

// 执行处理
async function main() {
  const processor = new CSVProcessor();
  await processor.process();
}

// 启动应用
main().catch(error => {
  console.error("应用程序执行失败:", error.message);
  process.exit(1);
});