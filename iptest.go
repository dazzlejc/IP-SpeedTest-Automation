package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	requestURL  = "speed.cloudflare.com/cdn-cgi/trace" // 请求trace URL
	timeout     = 1 * time.Second                      // 超时时间
	maxDuration = 2 * time.Second                      // 最大持续时间
)

var (
	File         = flag.String("file", "ip.txt", "IP地址文件名称,格式为 ip port ,就是IP和端口之间用空格隔开")       // IP地址文件名称
	outFile      = flag.String("outfile", "ip.csv", "输出文件名称")                                  // 输出文件名称
	maxThreads   = flag.Int("max", 100, "并发请求最大协程数")                                           // 最大协程数
	speedTest    = flag.Int("speedtest", 5, "下载测速协程数量,设为0禁用测速")                                // 下载测速协程数量
	speedTestURL = flag.String("url", "speed.cloudflare.com/__down?bytes=500000000", "测速文件地址") // 测速文件地址
	enableTLS    = flag.Bool("tls", true, "是否启用TLS")                                           // TLS是否启用
	delay        = flag.Int("delay", 300, "延迟阈值(ms)，默认300ms，设为0禁用延迟过滤")                   // 延迟阈值
	speedThreshold = flag.Float64("speedthreshold", 3.0, "速度阈值(MB/s)，默认3.0MB/s，设为0禁用速度过滤") // 速度阈值
	uploadURL    = flag.String("upload", "", "上传API地址，留空则不上传")                              // 上传API地址
	uploadToken  = flag.String("token", "", "上传API认证令牌")                                      // 上传API令牌
)

type result struct {
	ip          string        // IP地址
	port        int           // 端口
	dataCenter  string        // 数据中心
	locCode    string        // 源IP位置
	region      string        // 地区
	city        string        // 城市
	region_zh      string        // 地区
	country        string        // 国家
	city_zh      string        // 城市
	emoji      string        // 国旗
	latency     string        // 延迟
	tcpDuration time.Duration // TCP请求延迟
}

type speedtestresult struct {
	result
	downloadSpeed float64 // 下载速度
}

type location struct {
	Iata   string  `json:"iata"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Cca2   string  `json:"cca2"`
	Region string  `json:"region"`
	City   string  `json:"city"`
	Region_zh string  `json:"region_zh"`
	Country   string  `json:"country"`
	City_zh string  `json:"city_zh"`
	Emoji   string  `json:"emoji"`
}

// 尝试提升文件描述符的上限
func increaseMaxOpenFiles() {
	fmt.Println("正在尝试提升文件描述符的上限...")
	cmd := exec.Command("bash", "-c", "ulimit -n 10000")
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("提升文件描述符上限时出现错误: %v\n", err)
	} else {
		fmt.Printf("文件描述符上限已提升!\n")
	}
}

func main() {
	// 检查是否有命令行参数
	if len(os.Args) > 1 {
		// 有参数，使用命令行模式
		runOriginalMain()
		return
	}

	// 无参数，进入交互模式
	for {
		showMenu()
		choice := readInput()

		switch choice {
		case "1":
			scanLocalFiles()
		case "2":
			downloadFromAPI()
		case "3":
			uploadResultsMenu()
		case "4":
			showSettingsMenu()
		case "5":
			fmt.Println("再见!")
			return
		default:
			fmt.Println("无效选择，请重新输入")
		}
	}
}

// 显示主菜单
func showMenu() {
	fmt.Println("\n=== IP测速工具 ===")
	fmt.Println("1. 扫描本地文件测速")
	fmt.Println("2. 从API下载测速")
	fmt.Println("3. 上传测速结果")
	fmt.Println("4. 测速参数设置")
	fmt.Println("5. 退出")
	fmt.Print("请选择模式 (1-5): ")
}

// 读取用户输入
func readInput() string {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// 扫描本地文件
func scanLocalFiles() {
	fmt.Println("\n=== 扫描本地文件 ===")

	// 查找当前目录下的.txt和.csv文件
	files, err := filepath.Glob("*.txt")
	if err != nil {
		fmt.Printf("查找文件失败: %v\n", err)
		return
	}

	csvFiles, err := filepath.Glob("*.csv")
	if err != nil {
		fmt.Printf("查找CSV文件失败: %v\n", err)
		return
	}

	files = append(files, csvFiles...)

	if len(files) == 0 {
		fmt.Println("当前目录下未找到.txt或.csv文件")
		return
	}

	fmt.Println("找到以下文件:")
	for i, file := range files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("请选择文件序号: ")
	choice := readInput()

	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(files) {
		fmt.Println("无效的选择")
		return
	}

	selectedFile := files[index-1]
	fmt.Printf("已选择文件: %s\n", selectedFile)

	// 预处理文件
	processedFile, err := preprocessFile(selectedFile)
	if err != nil {
		fmt.Printf("文件预处理失败: %v\n", err)
		return
	}

	if processedFile != selectedFile {
		fmt.Printf("文件已预处理完成，输出到: %s\n", processedFile)
	}

	// 使用预处理后的文件运行测速
	runTestWithFile(processedFile)
}

// 从API下载
func downloadFromAPI() {
	fmt.Println("\n=== 从API下载 ===")
	fmt.Print("请输入API地址 (直接回车使用默认地址): ")
	apiURL := readInput()

	if apiURL == "" {
		apiURL = "https://zip.cm.edu.kg/all.txt"
	}

	fmt.Printf("正在从 %s 下载IP列表...\n", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Printf("下载失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("下载失败，状态码: %d\n", resp.StatusCode)
		return
	}

	// 保存到临时文件
	tempFile := "temp_downloaded_ips.txt"
	file, err := os.Create(tempFile)
	if err != nil {
		fmt.Printf("创建临时文件失败: %v\n", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		fmt.Printf("保存文件失败: %v\n", err)
		return
	}

	fmt.Printf("下载完成，保存到: %s\n", tempFile)

	// 预处理下载的文件
	processedFile, err := preprocessFile(tempFile)
	if err != nil {
		fmt.Printf("文件预处理失败: %v\n", err)
		os.Remove(tempFile)
		return
	}

	if processedFile != tempFile {
		fmt.Printf("文件已预处理完成，输出到: %s\n", processedFile)
		// 删除原始下载文件
		os.Remove(tempFile)
	}

	// 使用预处理后的文件运行测速
	runTestWithFile(processedFile)

	// 清理临时文件（如果是预处理文件）
	if strings.Contains(processedFile, "temp_") {
		os.Remove(processedFile)
	}
}

// 上传结果
func uploadResultsMenu() {
	fmt.Println("\n=== 上传测速结果 ===")
	fmt.Println("1. 上传测速结果文件")
	fmt.Println("2. 选择IP列表文件上传")
	fmt.Println("3. 返回主菜单")
	fmt.Print("请选择上传模式 (1-3): ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1":
		uploadSpeedTestResults()
	case "2":
		uploadIPListFile()
	case "3":
		return
	default:
		fmt.Println("无效选择，返回主菜单")
	}
}

// 上传测速结果文件
func uploadSpeedTestResults() {
	fmt.Println("\n=== 上传测速结果文件 ===")

	// 查找最新的结果文件
	files, err := filepath.Glob("ip*.csv")
	if err != nil {
		fmt.Printf("查找文件失败: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Println("未找到测速结果文件")
		return
	}

	// 按修改时间排序，获取最新的文件
	type fileInfo struct {
		name    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{file, stat.ModTime()})
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.After(fileInfos[j].modTime)
	})

	latestFile := fileInfos[0].name
	fmt.Printf("找到最新结果文件: %s\n", latestFile)

	fmt.Print("请输入上传API地址: ")
	uploadURL := readInput()

	if uploadURL == "" {
		fmt.Println("未输入API地址，取消上传")
		return
	}

	fmt.Print("请输入认证令牌 (可选): ")
	uploadToken := readInput()

	// 读取结果文件
	results, err := readResultsFromCSV(latestFile)
	if err != nil {
		fmt.Printf("读取结果文件失败: %v\n", err)
		return
	}

	// 上传结果
	fmt.Println("正在上传结果...")
	if err := uploadResults(results, uploadURL, uploadToken); err != nil {
		fmt.Printf("上传失败: %v\n", err)
	}
}

// 选择IP列表文件上传
func uploadIPListFile() {
	fmt.Println("\n=== 选择IP列表文件上传 ===")

	// 查找当前目录下的.txt和.csv文件
	txtFiles, err := filepath.Glob("*.txt")
	if err != nil {
		fmt.Printf("查找TXT文件失败: %v\n", err)
		return
	}

	csvFiles, err := filepath.Glob("*.csv")
	if err != nil {
		fmt.Printf("查找CSV文件失败: %v\n", err)
		return
	}

	var allFiles []string
	allFiles = append(allFiles, txtFiles...)
	allFiles = append(allFiles, csvFiles...)

	if len(allFiles) == 0 {
		fmt.Println("当前目录下未找到.txt或.csv文件")
		return
	}

	fmt.Println("请选择要上传的文件:")
	for i, file := range allFiles {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("请选择文件编号: ")
	var choiceStr string
	fmt.Scanln(&choiceStr)

	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > len(allFiles) {
		fmt.Println("无效选择")
		return
	}

	selectedFile := allFiles[choice-1]
	fmt.Printf("已选择文件: %s\n", selectedFile)

	fmt.Print("请输入上传API地址: ")
	uploadURL := readInput()

	if uploadURL == "" {
		fmt.Println("未输入API地址，取消上传")
		return
	}

	fmt.Print("请输入认证令牌 (可选): ")
	uploadToken := readInput()

	// 上传IP列表
	fmt.Println("正在上传IP列表...")
	if err := uploadIPListFromFile(selectedFile, uploadURL, uploadToken); err != nil {
		fmt.Printf("上传失败: %v\n", err)
	}
}

// 从CSV文件读取结果
func readResultsFromCSV(filename string) ([]speedtestresult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) <= 1 {
		return nil, fmt.Errorf("文件中没有数据")
	}

	var results []speedtestresult
	// 跳过标题行
	for _, record := range records[1:] {
		if len(record) < 12 {
			continue
		}

		port, _ := strconv.Atoi(record[1])
		tcpDuration, _ := time.ParseDuration(record[11] + "ms")

		var downloadSpeed float64
		if len(record) > 12 && record[12] != "" {
			speedStr := record[12]
			// CSV中存储的是MB/s，转换为KB/s用于内部处理
			if speed, err := strconv.ParseFloat(speedStr, 64); err == nil {
				downloadSpeed = speed * 1024
			}
		}

		res := speedtestresult{
			result: result{
				ip:          record[0],
				port:        port,
				dataCenter:  record[3],
				locCode:    record[3], // 机场代码在record[3] (如SIN)
				region:      record[5],
				city:        record[6],
				region_zh:    record[7],
				country:      record[8],
				city_zh:      record[9],
				emoji:      record[10],
				latency:     record[11],
				tcpDuration: tcpDuration,
			},
			downloadSpeed: downloadSpeed,
		}
		results = append(results, res)
	}

	return results, nil
}

// 预处理文件 - 调用JavaScript脚本进行格式化和去重
func preprocessFile(inputFile string) (string, error) {
	// 检查文件是否已经是标准格式
	if isStandardFormat(inputFile) {
		fmt.Println("文件已是标准格式，无需预处理")
		return inputFile, nil
	}

	// 生成输出文件名
	baseName := strings.TrimSuffix(inputFile, filepath.Ext(inputFile))
	outputFile := baseName + "_processed.txt"

	// 构建JavaScript命令
	jsPath := "ip_preprocess.js"
	if !fileExists(jsPath) {
		return "", fmt.Errorf("JavaScript处理脚本不存在: %s", jsPath)
	}

	cmd := exec.Command("node", jsPath, inputFile, outputFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("正在预处理文件...")
	fmt.Println("  - 格式化: 转换为 IP 端口 格式")
	fmt.Println("  - 去重: 移除重复的IP:端口组合")
	fmt.Println("  - 验证: 检查IP地址和端口有效性")

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("预处理失败: %v", err)
	}

	// 检查输出文件是否生成
	if !fileExists(outputFile) {
		return "", fmt.Errorf("预处理输出文件未生成: %s", outputFile)
	}

	return outputFile, nil
}

// 检查文件是否为标准格式
func isStandardFormat(filename string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	standardFormatCount := 0

	for scanner.Scan() {
		if lineCount >= 10 { // 只检查前10行
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 2 {
			// 检查第一部分是否为IP地址，第二部分是否为端口号
			if isIP(parts[0]) && isPort(parts[1]) {
				standardFormatCount++
			}
		}
		lineCount++
	}

	// 如果80%以上的行是标准格式，则认为是标准格式文件
	return lineCount > 0 && float64(standardFormatCount)/float64(lineCount) >= 0.8
}

// 检查是否为IP地址
func isIP(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// 检查是否为端口号
func isPort(s string) bool {
	port, err := strconv.Atoi(s)
	return err == nil && port > 0 && port <= 65535
}

// 检查文件是否存在
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// 显示设置菜单
func showSettingsMenu() {
	for {
		fmt.Println("\n=== 测速参数设置 ===")
		fmt.Println("1. 查看当前设置")
		fmt.Println("2. 修改延迟阈值")
		fmt.Println("3. 修改速度阈值")
		fmt.Println("4. 修改测速协程数")
		fmt.Println("5. 修改并发协程数")
		fmt.Println("6. 重置为默认值")
		fmt.Println("7. 返回主菜单")
		fmt.Print("请选择 (1-7): ")

		choice := readInput()

		switch choice {
		case "1":
			showCurrentSettings()
		case "2":
			modifyDelaySetting()
		case "3":
			modifySpeedSetting()
		case "4":
			modifySpeedTestSetting()
		case "5":
			modifyMaxThreadsSetting()
		case "6":
			resetToDefaults()
		case "7":
			return
		default:
			fmt.Println("无效选择，请重新输入")
		}
	}
}

// 显示当前设置
func showCurrentSettings() {
	fmt.Println("\n=== 当前设置 ===")
	fmt.Printf("  延迟阈值: %d ms %s\n", *delay, func() string {
		if *delay == 0 {
			return "(禁用过滤)"
		}
		return ""
	}())
	fmt.Printf("  速度阈值: %.1f MB/s %s\n", *speedThreshold, func() string {
		if *speedThreshold == 0 {
			return "(禁用过滤)"
		}
		return ""
	}())
	fmt.Printf("  测速协程数: %d %s\n", *speedTest, func() string {
		if *speedTest == 0 {
			return "(禁用测速)"
		}
		return ""
	}())
	fmt.Printf("  并发协程数: %d\n", *maxThreads)
	fmt.Printf("  TLS启用: %t\n", *enableTLS)
	fmt.Printf("  输出文件: %s\n", *outFile)
	if *uploadURL != "" {
		fmt.Printf("  上传API: %s\n", *uploadURL)
	}
}

// 修改延迟设置
func modifyDelaySetting() {
	fmt.Printf("\n当前延迟阈值: %d ms\n", *delay)
	fmt.Printf("输入新的延迟阈值 (0=禁用过滤): ")
	input := readInput()

	if input == "" {
		fmt.Println("保持原设置")
		return
	}

	if newDelay, err := strconv.Atoi(input); err == nil && newDelay >= 0 {
		*delay = newDelay
		fmt.Printf("延迟阈值已更新为: %d ms\n", *delay)
	} else {
		fmt.Println("输入无效，请输入非负整数")
	}
}

// 修改速度设置
func modifySpeedSetting() {
	fmt.Printf("\n当前速度阈值: %.1f MB/s\n", *speedThreshold)
	fmt.Printf("输入新的速度阈值 (0=禁用过滤): ")
	input := readInput()

	if input == "" {
		fmt.Println("保持原设置")
		return
	}

	if newSpeed, err := strconv.ParseFloat(input, 64); err == nil && newSpeed >= 0 {
		*speedThreshold = newSpeed
		fmt.Printf("速度阈值已更新为: %.1f MB/s\n", *speedThreshold)
	} else {
		fmt.Println("输入无效，请输入非负数")
	}
}

// 修改测速协程数设置
func modifySpeedTestSetting() {
	fmt.Printf("\n当前测速协程数: %d\n", *speedTest)
	fmt.Printf("输入新的测速协程数 (0=禁用测速): ")
	input := readInput()

	if input == "" {
		fmt.Println("保持原设置")
		return
	}

	if newTest, err := strconv.Atoi(input); err == nil && newTest >= 0 {
		*speedTest = newTest
		fmt.Printf("测速协程数已更新为: %d\n", *speedTest)
	} else {
		fmt.Println("输入无效，请输入非负整数")
	}
}

// 修改并发协程数设置
func modifyMaxThreadsSetting() {
	fmt.Printf("\n当前并发协程数: %d\n", *maxThreads)
	fmt.Printf("输入新的并发协程数: ")
	input := readInput()

	if input == "" {
		fmt.Println("保持原设置")
		return
	}

	if newThread, err := strconv.Atoi(input); err == nil && newThread > 0 {
		*maxThreads = newThread
		fmt.Printf("并发协程数已更新为: %d\n", *maxThreads)
	} else {
		fmt.Println("输入无效，请输入正整数")
	}
}

// 重置为默认值
func resetToDefaults() {
	fmt.Print("\n确认重置所有设置为默认值? (y/N): ")
	choice := strings.ToLower(readInput())

	if choice == "y" || choice == "yes" {
		*delay = 300
		*speedThreshold = 3.0
		*speedTest = 5
		*maxThreads = 100
		*enableTLS = true
		*outFile = "ip.csv"
		*uploadURL = ""
		*uploadToken = ""

		fmt.Println("所有设置已重置为默认值")
		showCurrentSettings()
	} else {
		fmt.Println("取消重置")
	}
}

// 使用指定文件运行测速
func runTestWithFile(filename string) {
	// 设置全局参数
	*File = filename

	// 显示当前设置信息
	fmt.Println("\n=== 使用当前设置进行测速 ===")
	showCurrentSettings()

	// 运行原有的main逻辑
	runOriginalMain()
}

// 原有的main逻辑
func runOriginalMain() {
	flag.Parse()
	var validCount int32 // 有效IP计数器

	startTime := time.Now()
	osType := runtime.GOOS
	if osType == "linux" {
		increaseMaxOpenFiles()
	}

	var locations []location
	if _, err := os.Stat("locations.json"); os.IsNotExist(err) {
		fmt.Println("本地 locations.json 不存在\n正在从 https://locations-adw.pages.dev/ 下载 locations.json")
		resp, err := http.Get("https://locations-adw.pages.dev/")
		if err != nil {
			fmt.Printf("无法从URL中获取JSON: %v\n", err)
			return
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("无法读取响应体: %v\n", err)
			return
		}

		err = json.Unmarshal(body, &locations)
		if err != nil {
			fmt.Printf("无法解析JSON: %v\n", err)
			return
		}
		file, err := os.Create("locations.json")
		if err != nil {
			fmt.Printf("无法创建文件: %v\n", err)
			return
		}
		defer file.Close()

		_, err = file.Write(body)
		if err != nil {
			fmt.Printf("无法写入文件: %v\n", err)
			return
		}
	} else {
		fmt.Println("本地 locations.json 已存在,无需重新下载")
		file, err := os.Open("locations.json")
		if err != nil {
			fmt.Printf("无法打开文件: %v\n", err)
			return
		}
		defer file.Close()

		body, err := ioutil.ReadAll(file)
		if err != nil {
			fmt.Printf("无法读取文件: %v\n", err)
			return
		}

		err = json.Unmarshal(body, &locations)
		if err != nil {
			fmt.Printf("无法解析JSON: %v\n", err)
			return
		}
	}

	locationMap := make(map[string]location)
	for _, loc := range locations {
		locationMap[loc.Iata] = loc
	}

	ips, err := readIPs(*File)
	if err != nil {
		fmt.Printf("无法从文件中读取 IP: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(ips))

	resultChan := make(chan result, len(ips))

	thread := make(chan struct{}, *maxThreads)

	var count int
	total := len(ips)

	for _, ip := range ips {
		thread <- struct{}{}
		go func(ip string) {
			defer func() {
				<-thread
				wg.Done()
				count++
				percentage := float64(count) / float64(total) * 100
				fmt.Printf("已完成: %d 总数: %d 已完成: %.2f%%\r", count, total, percentage)
				if count == total {
					fmt.Printf("已完成: %d 总数: %d 已完成: %.2f%%\n", count, total, percentage)
				}
			}()

			parts := strings.Fields(ip)
			if len(parts) != 2 {
				fmt.Printf("IP地址格式错误: %s\n", ip)
				return
			}
			ipAddr := parts[0]
			portStr := parts[1]

			port, err := strconv.Atoi(portStr)
			if err != nil {
				fmt.Printf("端口格式错误: %s\n", portStr)
				return
			}

			dialer := &net.Dialer{
				Timeout:   timeout,
				KeepAlive: 0,
			}
			start := time.Now()
			conn, err := dialer.Dial("tcp", net.JoinHostPort(ipAddr, strconv.Itoa(port)))
			if err != nil {
				return
			}
			defer conn.Close()

			tcpDuration := time.Since(start)
			if *delay > 0 && tcpDuration.Milliseconds() > int64(*delay) {
				return // 超过延迟阈值直接返回（仅在delay>0时生效）
			}

			start = time.Now()

			client := http.Client{
				Transport: &http.Transport{
					Dial: func(network, addr string) (net.Conn, error) {
						return conn, nil
					},
				},
				Timeout: timeout,
			}

			var protocol string
			if *enableTLS {
				protocol = "https://"
			} else {
				protocol = "http://"
			}
			requestURL := protocol + requestURL

			req, _ := http.NewRequest("GET", requestURL, nil)

			// 添加用户代理
			req.Header.Set("User-Agent", "Mozilla/5.0")
			req.Close = true
			resp, err := client.Do(req)
			if err != nil {
				return
			}

			duration := time.Since(start)
			if duration > maxDuration {
				return
			}

			defer resp.Body.Close()
			buf := &bytes.Buffer{}
			// 创建一个读取操作的超时
			timeout := time.After(maxDuration)
			// 使用一个 goroutine 来读取响应体
			done := make(chan bool)
			errChan := make(chan error)
			go func() {
				_, err := io.Copy(buf, resp.Body)
				done <- true
				errChan <- err
				if err != nil {
					return
				}
			}()
			// 等待读取操作完成或者超时
			select {
			case <-done:
				// 读取操作完成
			case <-timeout:
				// 读取操作超时
				return
			}

			body := buf
			err = <-errChan
			if err != nil {
				return
			}
			if strings.Contains(body.String(), "uag=Mozilla/5.0") {
				if matches := regexp.MustCompile(`colo=([A-Z]+)[\s\S]*?loc=([A-Z]+)`).FindStringSubmatch(body.String()); len(matches) > 2 {
					dataCenter := matches[1]
					locCode := matches[2]
					loc, ok := locationMap[dataCenter]
					// 记录通过延迟检查的有效IP
					atomic.AddInt32(&validCount, 1)
					if ok {
						fmt.Printf("发现有效IP %s 端口 %d 位置信息 %s 延迟 %d 毫秒\n", ipAddr, port, loc.City_zh, tcpDuration.Milliseconds())
						resultChan <- result{ipAddr, port, dataCenter, locCode, loc.Region, loc.City, loc.Region_zh, loc.Country, loc.City_zh, loc.Emoji, fmt.Sprintf("%d ms", tcpDuration.Milliseconds()), tcpDuration}
					} else {
						fmt.Printf("发现有效IP %s 端口 %d 位置信息未知 延迟 %d 毫秒\n", ipAddr, port, tcpDuration.Milliseconds())
						resultChan <- result{ipAddr, port, dataCenter, locCode, "", "", "", "", "", "", fmt.Sprintf("%d ms", tcpDuration.Milliseconds()), tcpDuration}
					}
				}
			}
		}(ip)
	}

	wg.Wait()
	close(resultChan)

	if len(resultChan) == 0 {
		// 清除输出内容
		fmt.Print("\033[2J")
		fmt.Println("没有发现有效的IP")
		return
	}
	var results []speedtestresult
	if *speedTest > 0 {
		fmt.Printf("找到符合条件的ip 共%d个\n", atomic.LoadInt32(&validCount))
		fmt.Printf("开始测速\n")
		var wg2 sync.WaitGroup
		wg2.Add(*speedTest)
		count = 0
		total := len(resultChan)
		results = []speedtestresult{}
		for i := 0; i < *speedTest; i++ {
			thread <- struct{}{}
			go func() {
				defer func() {
					<-thread
					wg2.Done()
				}()
				for res := range resultChan {

					downloadSpeed := getDownloadSpeed(res.ip, res.port)
					// 速度阈值过滤：只添加满足条件的IP到结果中
					if downloadSpeed > 0 {
						results = append(results, speedtestresult{result: res, downloadSpeed: downloadSpeed})
					}

					count++
					percentage := float64(count) / float64(total) * 100
					fmt.Printf("已完成: %.2f%%\r", percentage)
					if count == total {
						fmt.Printf("已完成: %.2f%%\033[0\n", percentage)
					}
				}
			}()
		}
		wg2.Wait()
	} else {
		for res := range resultChan {
			results = append(results, speedtestresult{result: res})
		}
	}

	if *speedTest > 0 {
		sort.Slice(results, func(i, j int) bool {
			return results[i].downloadSpeed > results[j].downloadSpeed
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			return results[i].result.tcpDuration < results[j].result.tcpDuration
		})
	}

	file, err := os.Create(*outFile)
	if err != nil {
		fmt.Printf("无法创建文件: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if *speedTest > 0 {
		writer.Write([]string{"IP地址", "端口", "TLS", "数据中心", "源IP位置", "地区", "城市", "地区(中文)", "国家", "城市(中文)", "国旗", "网络延迟", "下载速度(MB/s)"})
	} else {
		writer.Write([]string{"IP地址", "端口", "TLS", "数据中心", "源IP位置", "地区", "城市", "地区(中文)", "国家", "城市(中文)", "国旗", "网络延迟"})
	}
	for _, res := range results {
		if *speedTest > 0 {
			speedMBs := res.downloadSpeed / 1024
			if speedMBs >= 1 {
				writer.Write([]string{res.result.ip, strconv.Itoa(res.result.port), strconv.FormatBool(*enableTLS), res.result.dataCenter, res.result.locCode, res.result.region, res.result.city, res.result.region_zh, res.result.country, res.result.city_zh, res.result.emoji, res.result.latency, fmt.Sprintf("%.2f", speedMBs)})
			} else {
				writer.Write([]string{res.result.ip, strconv.Itoa(res.result.port), strconv.FormatBool(*enableTLS), res.result.dataCenter, res.result.locCode, res.result.region, res.result.city, res.result.region_zh, res.result.country, res.result.city_zh, res.result.emoji, res.result.latency, fmt.Sprintf("%.3f", speedMBs)})
			}
		} else {
			writer.Write([]string{res.result.ip, strconv.Itoa(res.result.port), strconv.FormatBool(*enableTLS), res.result.dataCenter, res.result.locCode, res.result.region, res.result.city, res.result.region_zh, res.result.country, res.result.city_zh, res.result.emoji, res.result.latency})
		}
	}

	writer.Flush()
	// 清除输出内容
	fmt.Print("\033[2J")
	fmt.Printf("有效IP数量: %d | 成功将结果写入文件 %s，耗时 %d秒\n", atomic.LoadInt32(&validCount), *outFile, time.Since(startTime)/time.Second)

	// 上传结果到API（如果配置了）
	if *uploadURL != "" {
		fmt.Println("正在上传结果到API...")
		if err := uploadResults(results, *uploadURL, *uploadToken); err != nil {
			fmt.Printf("上传失败: %v\n", err)
		}
	}
}

// 从文件中读取IP地址和端口
func readIPs(File string) ([]string, error) {
	file, err := os.Open(File)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var ips []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			fmt.Printf("行格式错误: %s\n", line)
			continue
		}
		ipAddr := parts[0]
		portStr := parts[1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			fmt.Printf("端口格式错误: %s\n", portStr)
			continue
		}

		ip := fmt.Sprintf("%s %d", ipAddr, port)
		ips = append(ips, ip)
	}
	return ips, scanner.Err()
}

// inc函数实现ip地址自增
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// 测速函数
func getDownloadSpeed(ip string, port int) float64 {
	var protocol string
	if *enableTLS {
		protocol = "https://"
	} else {
		protocol = "http://"
	}
	speedTestURL := protocol + *speedTestURL
	// 创建请求
	req, _ := http.NewRequest("GET", speedTestURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	// 创建TCP连接
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 0,
	}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return 0
	}
	defer conn.Close()

	fmt.Printf("正在测试IP %s 端口 %d\n", ip, port)
	startTime := time.Now()
	// 创建HTTP客户端
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return conn, nil
			},
		},
		//设置单个IP测速最长时间为5秒
		Timeout: 5 * time.Second,
	}
	// 发送请求
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("IP %s 端口 %d 测速无效\n", ip, port)
		return 0
	}
	defer resp.Body.Close()

	// 复制响应体到/dev/null，并计算下载速度
	written, _ := io.Copy(io.Discard, resp.Body)
	duration := time.Since(startTime)
	speedKBs := float64(written) / duration.Seconds() / 1024
	speedMBs := speedKBs / 1024

	// 速度阈值过滤
	if *speedThreshold > 0 && speedMBs < *speedThreshold {
		fmt.Printf("IP %s 端口 %d 速度 %.2f MB/s 低于阈值 %.2f MB/s，已过滤\n", ip, port, speedMBs, *speedThreshold)
		return 0
	}

	// 输出结果 - 使用MB/s单位显示
	if speedMBs >= 1 {
		fmt.Printf("IP %s 端口 %d 下载速度 %.2f MB/s\n", ip, port, speedMBs)
	} else {
		fmt.Printf("IP %s 端口 %d 下载速度 %.0f kB/s\n", ip, port, speedKBs)
	}
	return speedKBs // 仍返回KB/s保持精度
}

// 上传结果到API
func uploadResults(results []speedtestresult, uploadURL, token string) error {
	if uploadURL == "" {
		fmt.Println("未配置上传API地址，跳过上传")
		return nil
	}

	// 格式化为 IP:端口#城市(中文)国旗
	var ipList []string
	for _, res := range results {
		// 尝试获取城市信息（中文名+国旗），处理编码问题
		cityInfo := getValidCityInfo(res.result.city_zh, res.result.city, res.result.locCode)
		ipList = append(ipList, fmt.Sprintf("%s:%d#%s", res.result.ip, res.result.port, cityInfo))
	}

	if len(ipList) == 0 {
		fmt.Println("没有有效的IP数据可上传")
		return nil
	}

	// 构建上传文本 - 每行一个IP
	uploadText := strings.Join(ipList, "\n")

	// 创建HTTP请求
	req, err := http.NewRequest("POST", uploadURL, strings.NewReader(uploadText))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头 - 使用纯文本格式
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "IPTest-Tool/1.0")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("成功上传 %d 个IP到API (格式: IP:端口#城市(中文))\n", len(ipList))
		return nil
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("上传失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

// 从文件上传IP列表
func uploadIPListFromFile(filename, uploadURL, token string) error {
	if uploadURL == "" {
		fmt.Println("未配置上传API地址，跳过上传")
		return nil
	}

	// 读取文件内容
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	var ipList []string
	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// 尝试解析不同格式的IP行
		ip, port, city := parseIPLineForUpload(line)
		if ip != "" && port > 0 {
			// 如果没有城市信息，使用默认值
			if city == "" {
				city = "Unknown"
			}
			ipList = append(ipList, fmt.Sprintf("%s:%d#%s", ip, port, city))
			lineCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	if len(ipList) == 0 {
		fmt.Println("没有找到有效的IP数据可上传")
		return nil
	}

	fmt.Printf("从文件中解析出 %d 个有效IP (总行数: %d)\n", len(ipList), lineCount)

	// 构建上传文本 - 每行一个IP
	uploadText := strings.Join(ipList, "\n")

	// 创建HTTP请求
	req, err := http.NewRequest("POST", uploadURL, strings.NewReader(uploadText))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头 - 使用纯文本格式
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", "IPTest-Tool/1.0")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("成功上传 %d 个IP到API (格式: IP:端口#城市(中文))\n", len(ipList))
		return nil
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("上传失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}
}

// 解析IP行用于上传 - 支持多种格式
func parseIPLineForUpload(line string) (ip string, port int, city string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", 0, ""
	}

	// 格式1: IP 端口 (标准格式)
	if strings.Contains(line, " ") && !strings.Contains(line, ":") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			ip = parts[0]
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
				// 如果有更多部分，作为城市名
				if len(parts) >= 3 {
					city = strings.Join(parts[2:], " ")
				}
			}
		}
	} else if strings.Contains(line, ":") && !strings.Contains(line, "#") {
		// 格式2: IP:端口
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			ip = parts[0]
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			}
		}
	} else if strings.Contains(line, ":") && strings.Contains(line, "#") {
		// 格式3: IP:端口#国家描述
		beforeHash := strings.Split(line, "#")[0]
		parts := strings.SplitN(beforeHash, ":", 2)
		if len(parts) == 2 {
			ip = parts[0]
			if p, err := strconv.Atoi(parts[1]); err == nil {
				port = p
			}
		}
		// 提取国家描述作为城市
		if hashParts := strings.SplitN(line, "#", 2); len(hashParts) == 2 {
			city = hashParts[1]
		}
	}

	// 验证IP地址格式
	if !isValidIPForUpload(ip) {
		return "", 0, ""
	}

	// 验证端口范围
	if port < 1 || port > 65535 {
		return "", 0, ""
	}

	return ip, port, city
}

// 验证IP地址格式
func isValidIPForUpload(ip string) bool {
	if ip == "" {
		return false
	}

	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// 获取有效的城市信息（中文名+国旗），处理编码问题
func getValidCityInfo(cityZh, city, locCode string) string {
	// 优先尝试中文城市名（如果有且有效）
	if cityZh != "" && isValidUTF8(cityZh) {
		return cityZh
	}

	// 使用位置代码映射（优先于英文城市名，因为我们想要中文城市名）
	if locCode != "" {
		if cityName, exists := getCityNameByCode(locCode); exists {
			return cityName
		}
		// 如果映射中没有，但代码本身是有效的ASCII，就使用代码
		if isValidUTF8(locCode) {
			return locCode
		}
	}

	// 尝试英文城市名（作为最后的回退）
	if city != "" && isValidUTF8(city) {
		return city
	}

	// 最后的回退选项
	return "Unknown"
}

// 根据机场代码获取城市中文名
func getCityNameByCode(locCode string) (string, bool) {
	cityMap := map[string]string{
		"SIN": "新加坡",
		"HKG": "香港",
		"NRT": "东京",
		"ICN": "首尔",
		"BOM": "孟买",
		"DEL": "新德里",
		"SYD": "悉尼",
		"MEL": "墨尔本",
		"LHR": "伦敦",
		"CDG": "巴黎",
		"FRA": "法兰克福",
		"AMS": "阿姆斯特丹",
		"JFK": "纽约",
		"LAX": "洛杉矶",
		"SFO": "旧金山",
		"ORD": "芝加哥",
		"DFW": "达拉斯",
		"DXB": "迪拜",
		"DOH": "多哈",
		"BKK": "曼谷",
		"KUL": "吉隆坡",
		"CGK": "雅加达",
		"MNL": "马尼拉",
		"SGN": "胡志明市",
		"HAN": "河内",
		"TAI": "台北",
		"PVG": "上海",
		"PEK": "北京",
		"CAN": "广州",
		"SZX": "深圳",
		"CTU": "成都",
		"XIY": "西安",
		"KMG": "昆明",
		"KIX": "大阪",
		"NGO": "名古屋",
		"FCO": "罗马",
		"BCN": "巴塞罗那",
		"MAD": "马德里",
		"IST": "伊斯坦布尔",
		"CAI": "开罗",
		"JNB": "约翰内斯堡",
	}

	if cityName, exists := cityMap[locCode]; exists {
		return cityName, true
	}
	return "", false
}

// 检查字符串是否为有效的UTF-8编码
func isValidUTF8(s string) bool {
	if s == "" {
		return false
	}

	// 检查是否包含乱码字符
	invalidChars := []string{"�", "\uFFFD", "�", "锟斤拷"}
	for _, invalid := range invalidChars {
		if strings.Contains(s, invalid) {
			return false
		}
	}

	// 检查是否全是ASCII字符（安全）
	if isASCII(s) {
		return true
	}

	// 对于非ASCII字符，检查是否能正确转换为UTF-8
	return utf8.ValidString(s)
}

// 检查字符串是否只包含ASCII字符
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}