package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ==========================================
// 1. 数据结构与 CLI 参数定义
// ==========================================

type FilterList []string

func (f *FilterList) String() string { return fmt.Sprintf("%v", *f) }
func (f *FilterList) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type ZabbixRequest struct {
	JsonRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Auth    string      `json:"auth,omitempty"`
	ID      int         `json:"id"`
}

type ZabbixError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type ZabbixResponse struct {
	JsonRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *ZabbixError    `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type ZabbixClient struct {
	URL   string
	User  string
	Pass  string
	Token string
	HTTP  *http.Client
}

// 标准 JSON 日志输出格式
type LogOutput struct {
	Timestamp int64  `json:"timestamp"`
	Target    string `json:"target"`
	Action    string `json:"action"`
	Count     int    `json:"count"`
	Status    string `json:"status"`
}

// ==========================================
// 2. 核心底层方法
// ==========================================

func (c *ZabbixClient) Call(method string, params interface{}, result interface{}) error {
	reqBody := ZabbixRequest{
		JsonRPC: "2.0",
		Method:  method,
		Params:  params,
		Auth:    c.Token,
		ID:      1,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Post(c.URL, "application/json-rpc", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var zabbixResp ZabbixResponse
	if err := json.Unmarshal(bodyBytes, &zabbixResp); err != nil {
		// 注意：此处不能把 bodyBytes 打印出来，防止泄露可能包含的敏感信息
		return fmt.Errorf("解析Zabbix响应失败")
	}

	if zabbixResp.Error != nil {
		return fmt.Errorf("Zabbix API错误 [%d]: %s - %s", zabbixResp.Error.Code, zabbixResp.Error.Message, zabbixResp.Error.Data)
	}

	if len(zabbixResp.Result) > 0 && string(zabbixResp.Result) != "null" {
		return json.Unmarshal(zabbixResp.Result, result)
	}
	return nil
}

func (c *ZabbixClient) Login() error {
	params := map[string]string{
		"username": c.User,
		"password": c.Pass,
	}
	var token string
	// 登录失败直接返回错误，不涉及注销
	if err := c.Call("user.login", params, &token); err != nil {
		return fmt.Errorf("Zabbix 登录失败: %v", err)
	}
	c.Token = token
	return nil
}

func (c *ZabbixClient) Logout() error {
	if c.Token == "" {
		return nil
	}
	// user.logout 不需要额外参数，传空数组即可
	err := c.Call("user.logout", []string{}, nil)
	c.Token = "" // 注销后清空本地 Token
	return err
}

func printJSONLog(target, action string, count int) {
	log := LogOutput{
		Timestamp: time.Now().Unix(),
		Target:    target,
		Action:    action,
		Count:     count,
		Status:    "success",
	}
	out, _ := json.Marshal(log)
	fmt.Println(string(out))
}

// 将大切片分批 (Batching)
func chunkStrings(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// 生成监控项维护名称 (含去重、排序、多字节安全截断与 SHA1)
func generateItemMaintName(hostname string, filters FilterList) string {
	var normalized []string
	seen := make(map[string]bool)
	for _, f := range filters {
		parts := strings.SplitN(f, ":", 2)
		if len(parts) == 2 {
			str := fmt.Sprintf("%s_%s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			if !seen[str] {
				seen[str] = true
				normalized = append(normalized, str)
			}
		}
	}
	sort.Strings(normalized)
	filterToken := strings.Join(normalized, "+")
	fullName := fmt.Sprintf("Kanshi_Maint_%s_item_%s", hostname, filterToken)

	// 用 []byte 计算长度，防止多字节字符导致的截断乱码问题
	if len([]byte(fullName)) > 128 {
		prefix := string([]byte(fullName)[:115])
		hash := sha1.Sum([]byte(fullName))
		hashStr := hex.EncodeToString(hash[:])[:12]
		return fmt.Sprintf("%s_%s", prefix, hashStr)
	}
	return fullName
}

// ==========================================
// 3. 业务逻辑实现 (全面改为返回 error)
// ==========================================

func getHostID(client *ZabbixClient, hostname string) (string, error) {
	var hosts []struct {
		HostID string `json:"hostid"`
	}
	err := client.Call("host.get", map[string]interface{}{
		"filter": map[string]string{"host": hostname},
		"output": []string{"hostid"},
	}, &hosts)
	if err != nil {
		return "", fmt.Errorf("获取主机信息失败: %v", err)
	}
	if len(hosts) == 0 {
		return "", fmt.Errorf("未找到主机: %s", hostname)
	}
	return hosts[0].HostID, nil
}

func runHostLogic(client *ZabbixClient, hostname, status, mode string) error {
	hostID, err := getHostID(client, hostname)
	if err != nil {
		return err
	}
	maintName := "Kanshi_HostMaint_" + hostname

	if status == "off" {
		mtype := 0
		if mode == "without_data" {
			mtype = 1
		}

		// 幂等检查
		var existing []interface{}
		client.Call("maintenance.get", map[string]interface{}{"filter": map[string]string{"name": maintName}}, &existing)
		if len(existing) > 0 {
			printJSONLog("host", "maintenance_create", 0)
			return nil
		}

		now := time.Now().Unix()
		err := client.Call("maintenance.create", map[string]interface{}{
			"name":             maintName,
			"maintenance_type": mtype,
			"hosts":            []map[string]string{{"hostid": hostID}},
			"active_since":     now,
			"active_till":      now + 315360000,
			"timeperiods": []map[string]interface{}{
				{"timeperiod_type": 0, "start_date": now, "period": 315360000},
			},
		}, nil)
		if err != nil {
			return fmt.Errorf("创建主机维护失败: %v", err)
		}
		printJSONLog("host", "maintenance_create", 1)

	} else if status == "on" {
		var existing []struct {
			MaintenanceID string `json:"maintenanceid"`
		}
		client.Call("maintenance.get", map[string]interface{}{"filter": map[string]string{"name": maintName}}, &existing)
		if len(existing) == 0 {
			printJSONLog("host", "maintenance_delete", 0)
			return nil
		}

		var delIDs []string
		for _, e := range existing {
			delIDs = append(delIDs, e.MaintenanceID)
		}
		if err := client.Call("maintenance.delete", delIDs, nil); err != nil {
			return fmt.Errorf("删除主机维护失败: %v", err)
		}
		printJSONLog("host", "maintenance_delete", len(delIDs))
	}
	return nil
}

// 并发查询目标 Item 集合
func resolveTargetItems(client *ZabbixClient, hostID string, filters FilterList) ([]string, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	itemMap := make(map[string]bool)

	for _, f := range filters {
		wg.Add(1)
		go func(filterStr string) {
			defer wg.Done()
			parts := strings.SplitN(filterStr, ":", 2)
			if len(parts) != 2 {
				return // 静默忽略不合法格式（在参数解析阶段其实已经拦截）
			}
			ftype := strings.TrimSpace(parts[0])
			fvalue := strings.TrimSpace(parts[1])

			params := map[string]interface{}{
				"hostids": hostID,
				"output":  []string{"itemid"},
			}

			if ftype == "Template" {
				var tpls []struct {
					TemplateID string `json:"templateid"`
				}
				err := client.Call("template.get", map[string]interface{}{"filter": map[string]string{"host": fvalue}}, &tpls)
				if err != nil {
					mu.Lock()
					if firstErr == nil { firstErr = err }
					mu.Unlock()
					return
				}
				if len(tpls) == 0 {
					return // 模板未找到，返回空
				}
				params["templateids"] = tpls[0].TemplateID
			} else if ftype == "Itemkey" {
				params["filter"] = map[string]string{"key_": fvalue}
			} else {
				return
			}

			var items []struct {
				ItemID string `json:"itemid"`
			}
			if err := client.Call("item.get", params, &items); err == nil {
				mu.Lock()
				for _, it := range items {
					itemMap[it.ItemID] = true
				}
				mu.Unlock()
			} else {
				mu.Lock()
				if firstErr == nil { firstErr = err }
				mu.Unlock()
			}
		}(f)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, fmt.Errorf("解析过滤条件时 API 调用失败: %v", firstErr)
	}

	var result []string
	for id := range itemMap {
		result = append(result, id)
	}
	return result, nil
}

func runItemLogic(client *ZabbixClient, hostname, status, mode string, filters FilterList) error {
	hostID, err := getHostID(client, hostname)
	if err != nil {
		return err
	}

	// 【例外】：解除维护不需要查 item
	if status == "on" && mode == "with_data" {
		maintName := generateItemMaintName(hostname, filters)
		var existing []struct {
			MaintenanceID string `json:"maintenanceid"`
		}
		client.Call("maintenance.get", map[string]interface{}{"filter": map[string]string{"name": maintName}}, &existing)
		if len(existing) == 0 {
			printJSONLog("item", "maintenance_delete", 0)
			return nil
		}
		var delIDs []string
		for _, e := range existing {
			delIDs = append(delIDs, e.MaintenanceID)
		}
		if err := client.Call("maintenance.delete", delIDs, nil); err != nil {
			return fmt.Errorf("删除监控项维护失败: %v", err)
		}
		printJSONLog("item", "maintenance_delete", len(delIDs))
		return nil
	}

	itemIDs, err := resolveTargetItems(client, hostID, filters)
	if err != nil {
		return err
	}

	if len(itemIDs) == 0 {
		action := "maintenance_create"
		if status == "on" { action = "enable" }
		if status == "off" && mode == "without_data" { action = "disable" }
		printJSONLog("item", action, 0)
		return nil
	}

	// 批量更新状态逻辑 (without_data)
	if mode == "without_data" {
		targetStatus := "1"
		action := "disable"
		if status == "on" {
			targetStatus = "0"
			action = "enable"
		}

		var validUpdateIDs []string
		// 分批查状态，防止超长
		for _, chunk := range chunkStrings(itemIDs, 500) {
			var items []struct {
				ItemID string `json:"itemid"`
				Status string `json:"status"`
			}
			if err := client.Call("item.get", map[string]interface{}{
				"itemids": chunk, "output": []string{"itemid", "status"},
			}, &items); err != nil {
				return fmt.Errorf("查询 Item 状态失败: %v", err)
			}

			for _, it := range items {
				if (status == "off" && it.Status == "0") || (status == "on" && it.Status == "1") {
					validUpdateIDs = append(validUpdateIDs, it.ItemID)
				}
			}
		}

		if len(validUpdateIDs) == 0 {
			printJSONLog("item", action, 0)
			return nil
		}

		// 分批执行更新
		for _, chunk := range chunkStrings(validUpdateIDs, 500) {
			var updates []map[string]string
			for _, id := range chunk {
				updates = append(updates, map[string]string{"itemid": id, "status": targetStatus})
			}
			if err := client.Call("item.update", updates, nil); err != nil {
				return fmt.Errorf("批量更新 Item 状态失败: %v", err)
			}
		}
		printJSONLog("item", action, len(validUpdateIDs))
		return nil
	}

	// 创建维护周期逻辑 (with_data, off)
	if mode == "with_data" && status == "off" {
		var triggers []struct {
			Description string `json:"description"`
		}
		if err := client.Call("trigger.get", map[string]interface{}{
			"itemids": itemIDs, "output": []string{"triggerid", "description"},
		}, &triggers); err != nil {
			return fmt.Errorf("查询 Trigger 失败: %v", err)
		}

		triggerMap := make(map[string]bool)
		for _, t := range triggers {
			if t.Description != "" {
				triggerMap[t.Description] = true
			}
		}

		if len(triggerMap) == 0 {
			printJSONLog("item", "maintenance_create", 0)
			return nil
		}

		var tags []map[string]string
		for desc := range triggerMap {
			tags = append(tags, map[string]string{
				"tag": "Kanshi-Maint-Key", "value": desc, "operator": "0",
			})
		}

		maintName := generateItemMaintName(hostname, filters)
		var existing []interface{}
		client.Call("maintenance.get", map[string]interface{}{"filter": map[string]string{"name": maintName}}, &existing)
		if len(existing) > 0 {
			printJSONLog("item", "maintenance_create", 0)
			return nil
		}

		now := time.Now().Unix()
		params := map[string]interface{}{
			"name":             maintName,
			"maintenance_type": 0,
			"hosts":            []map[string]string{{"hostid": hostID}},
			"active_since":     now,
			"active_till":      now + 315360000,
			"tags_evaltype":    2, // OR 逻辑
			"tags":             tags,
			"timeperiods": []map[string]interface{}{
				{"timeperiod_type": 0, "start_date": now, "period": 315360000},
			},
		}
		if err := client.Call("maintenance.create", params, nil); err != nil {
			return fmt.Errorf("创建监控项维护失败: %v", err)
		}
		printJSONLog("item", "maintenance_create", len(triggerMap))
	}
	return nil
}

// ==========================================
// 4. 初始化与主逻辑封装
// ==========================================

func run() error {
	// 1. 读取并校验环境变量
	apiURL := os.Getenv("ZABBIX_URL")
	apiUser := os.Getenv("ZABBIX_USER")
	apiPass := os.Getenv("ZABBIX_PASSWORD")

	if apiURL == "" || apiUser == "" || apiPass == "" {
		return errors.New("环境变量 ZABBIX_URL, ZABBIX_USER, ZABBIX_PASSWORD 必须提供")
	}

	// 2. 解析命令行参数
	target := flag.String("target", "", "目标类型: host 或 item")
	status := flag.String("status", "", "状态: on 或 off")
	hostname := flag.String("host", "", "主机名称")
	mode := flag.String("mode", "", "模式: with_data 或 without_data")

	var filters FilterList
	flag.Var(&filters, "filter", "过滤条件 (可多个)，格式: Type:Value")
	flag.Parse()

	// 参数合法性校验
	if *target != "host" && *target != "item" {
		return errors.New("参数错误: -target 必须是 host 或 item")
	}
	if *status != "on" && *status != "off" {
		return errors.New("参数错误: -status 必须是 on 或 off")
	}
	if *hostname == "" {
		return errors.New("参数错误: 必须提供 -host")
	}
	if *mode != "with_data" && *mode != "without_data" {
		return errors.New("参数错误: -mode 必须是 with_data 或 without_data")
	}
	if *target == "item" && len(filters) == 0 {
		return errors.New("参数错误: 当 -target 为 item 时，-filter 是必填项")
	}
	if *target == "host" && len(filters) > 0 {
		return errors.New("参数错误: 当 -target 为 host 时，不允许传入 -filter")
	}

	client := &ZabbixClient{
		URL:   apiURL,
		User:  apiUser,
		Pass:  apiPass,
		HTTP:  &http.Client{Timeout: 30 * time.Second},
	}

	// 3. 认证：必须先登录成功，再挂载 defer 注销
	if err := client.Login(); err != nil {
		return err
	}

	defer func() {
		// 无论业务逻辑成功或失败，都会执行注销
		if err := client.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 注销临时 Token 失败: %v\n", err)
		}
	}()

	// 4. 执行核心业务
	if *target == "host" {
		return runHostLogic(client, *hostname, *status, *mode)
	}
	return runItemLogic(client, *hostname, *status, *mode, filters)
}

func main() {
	// 通过 run() 包装确保 defer 能够被正常执行，避免 os.Exit 直接中断
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}