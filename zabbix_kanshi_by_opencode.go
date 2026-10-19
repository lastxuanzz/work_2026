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
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	tagKey          = "Kanshi-Maint-Key"
	tagOperator     = "0"
	tagsEvalType    = 2
	maintenanceSpan = 315360000
	maxNameLen      = 128
	truncateLen     = 115
	hashLen         = 12
	chunkSize       = 500
	requestTimeout  = 30 * time.Second
)

const (
	envURL      = "ZABBIX_URL"
	envUser     = "ZABBIX_USER"
	envPassword = "ZABBIX_PASSWORD"
)

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (e *apiError) Error() string {
	if e.Data != "" {
		return fmt.Sprintf("[%d] %s - %s", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Auth    string      `json:"auth,omitempty"`
	ID      int         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *apiError       `json:"error"`
	ID      int             `json:"id"`
}

type client struct {
	url   string
	token string
	http  *http.Client
	id    int
}

func newClient(apiURL string) *client {
	return &client{
		url:  apiURL,
		http: &http.Client{Timeout: requestTimeout},
	}
}

func (c *client) call(method string, params interface{}, result interface{}) error {
	c.id++
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		Auth:    c.token,
		ID:      c.id,
	})
	if err != nil {
		return fmt.Errorf("序列化 %s 请求失败: %w", method, err)
	}

	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("构造 %s 请求失败: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s 请求失败: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("读取 %s 响应失败: %w", method, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s 返回 HTTP %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("解析 %s 响应失败: %w", method, err)
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if result == nil || len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(rpcResp.Result, result); err != nil {
		return fmt.Errorf("解析 %s 结果失败: %w", method, err)
	}
	return nil
}

func (c *client) login(user, password string) error {
	attempts := []map[string]string{
		{"username": user, "password": password},
		{"user": user, "password": password},
	}
	var lastErr error
	for _, params := range attempts {
		var token string
		if err := c.call("user.login", params, &token); err != nil {
			lastErr = err
			if isUnexpectedParameter(err) {
				continue
			}
			return err
		}
		if token == "" {
			return errors.New("user.login 返回空 Token")
		}
		c.token = token
		return nil
	}
	return lastErr
}

func (c *client) logout() error {
	if c.token == "" {
		return nil
	}
	err := c.call("user.logout", []interface{}{}, nil)
	c.token = ""
	return err
}

func isUnexpectedParameter(err error) bool {
	var ae *apiError
	if !errors.As(err, &ae) {
		return false
	}
	msg := strings.ToLower(ae.Message + " " + ae.Data)
	return strings.Contains(msg, "unexpected parameter")
}

type parsedFilter struct {
	kind       string
	value      string
	normalized string
}

type config struct {
	target  string
	status  string
	host    string
	mode    string
	filters []parsedFilter
}

type filterFlag []string

func (f *filterFlag) String() string { return strings.Join(*f, ",") }

func (f *filterFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type envConfig struct {
	url      string
	user     string
	password string
}

type hostEntity struct {
	HostID string `json:"hostid"`
}

type templateEntity struct {
	TemplateID string `json:"templateid"`
}

type itemEntity struct {
	ItemID string `json:"itemid"`
	Status string `json:"status"`
}

type triggerEntity struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
}

type maintenanceEntity struct {
	MaintenanceID string `json:"maintenanceid"`
}

type logRecord struct {
	Timestamp int64  `json:"timestamp"`
	Target    string `json:"target"`
	Action    string `json:"action"`
	Count     int    `json:"count"`
	Status    string `json:"status"`
}

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "参数错误: "+err.Error())
		return 2
	}

	env, err := loadEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "环境变量错误: "+err.Error())
		return 2
	}

	c := newClient(env.url)
	if err := c.login(env.user, env.password); err != nil {
		fmt.Fprintln(os.Stderr, "登录失败: "+err.Error())
		return 1
	}
	defer func() {
		if err := c.logout(); err != nil {
			fmt.Fprintln(os.Stderr, "警告: 注销临时 Token 失败: "+err.Error())
		}
	}()

	if err := execute(c, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "执行失败: "+err.Error())
		return 1
	}
	return 0
}

func parseArgs(args []string) (*config, error) {
	fs := flag.NewFlagSet("zabbix_kanshi", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var target, status, host, mode string
	var rawFilters filterFlag

	fs.StringVar(&target, "target", "", "host 或 item")
	fs.StringVar(&status, "status", "", "on 或 off")
	fs.StringVar(&host, "host", "", "目标主机名称")
	fs.StringVar(&mode, "mode", "", "with_data 或 without_data")
	fs.Var(&rawFilters, "filter", "过滤条件，格式 Type:Value，可重复")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("不支持的位置参数: %s", strings.Join(fs.Args(), " "))
	}

	if target != "host" && target != "item" {
		return nil, errors.New("-target 必须是 host 或 item")
	}
	if status != "on" && status != "off" {
		return nil, errors.New("-status 必须是 on 或 off")
	}
	if host == "" {
		return nil, errors.New("-host 不能为空")
	}
	if mode != "with_data" && mode != "without_data" {
		return nil, errors.New("-mode 必须是 with_data 或 without_data")
	}

	cfg := &config{target: target, status: status, host: host, mode: mode}

	if target == "host" {
		if len(rawFilters) > 0 {
			return nil, errors.New("当 -target 为 host 时不允许传入 -filter")
		}
		return cfg, nil
	}

	filters, err := parseFilters(rawFilters)
	if err != nil {
		return nil, err
	}
	cfg.filters = filters
	return cfg, nil
}

func parseFilters(raw []string) ([]parsedFilter, error) {
	if len(raw) == 0 {
		return nil, errors.New("当 -target 为 item 时 -filter 为必填项")
	}
	filters := make([]parsedFilter, 0, len(raw))
	for _, f := range raw {
		idx := strings.Index(f, ":")
		if idx < 0 {
			return nil, fmt.Errorf("filter 缺少冒号: %q", f)
		}
		kind := strings.TrimSpace(f[:idx])
		value := strings.TrimSpace(f[idx+1:])
		if kind != "Template" && kind != "Itemkey" {
			return nil, fmt.Errorf("filter 类型非法（仅支持 Template 或 Itemkey）: %q", f)
		}
		if value == "" {
			return nil, fmt.Errorf("filter 值不能为空: %q", f)
		}
		filters = append(filters, parsedFilter{
			kind:       kind,
			value:      value,
			normalized: kind + "_" + value,
		})
	}
	return filters, nil
}

func loadEnv() (*envConfig, error) {
	apiURL := strings.TrimSpace(os.Getenv(envURL))
	user := os.Getenv(envUser)
	password := os.Getenv(envPassword)

	if apiURL == "" {
		return nil, fmt.Errorf("环境变量 %s 未设置或为空", envURL)
	}
	if user == "" {
		return nil, fmt.Errorf("环境变量 %s 未设置或为空", envUser)
	}
	if password == "" {
		return nil, fmt.Errorf("环境变量 %s 未设置或为空", envPassword)
	}

	parsed, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("环境变量 %s 不是合法 URL: %w", envURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("环境变量 %s 必须使用 http 或 https 协议", envURL)
	}

	return &envConfig{url: apiURL, user: user, password: password}, nil
}

func hostMaintenanceName(host string) string {
	return "Kanshi_HostMaint_" + host
}

func itemMaintenanceName(host string, filters []parsedFilter) string {
	seen := make(map[string]struct{}, len(filters))
	for _, f := range filters {
		seen[f.normalized] = struct{}{}
	}
	tokens := make([]string, 0, len(seen))
	for token := range seen {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)

	full := "Kanshi_Maint_" + host + "_item_" + strings.Join(tokens, "+")
	if utf8.RuneCountInString(full) <= maxNameLen {
		return full
	}
	runes := []rune(full)
	sum := sha1.Sum([]byte(full))
	return string(runes[:truncateLen]) + "_" + hex.EncodeToString(sum[:])[:hashLen]
}

func chunkStrings(slice []string, size int) [][]string {
	if size <= 0 || len(slice) == 0 {
		return nil
	}
	var chunks [][]string
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func sortedStrings(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func execute(c *client, cfg *config) error {
	hostID, err := getHostID(c, cfg.host)
	if err != nil {
		return err
	}
	if cfg.target == "host" {
		return executeHost(c, cfg, hostID)
	}
	return executeItem(c, cfg, hostID)
}

func executeHost(c *client, cfg *config, hostID string) error {
	name := hostMaintenanceName(cfg.host)

	if cfg.status == "off" {
		created, err := createHostMaintenance(c, hostID, name, cfg.mode)
		if err != nil {
			return err
		}
		count := 0
		if created {
			count = 1
		}
		return emitLog("host", "maintenance_create", count)
	}

	deleted, err := removeMaintenancesByName(c, name)
	if err != nil {
		return err
	}
	return emitLog("host", "maintenance_delete", deleted)
}

func executeItem(c *client, cfg *config, hostID string) error {
	if cfg.mode == "with_data" && cfg.status == "on" {
		name := itemMaintenanceName(cfg.host, cfg.filters)
		deleted, err := removeMaintenancesByName(c, name)
		if err != nil {
			return err
		}
		return emitLog("item", "maintenance_delete", deleted)
	}

	if cfg.mode == "without_data" {
		from, to, action := "0", "1", "disable"
		if cfg.status == "on" {
			from, to, action = "1", "0", "enable"
		}
		count, err := changeItemStatus(c, hostID, cfg.filters, from, to)
		if err != nil {
			return err
		}
		return emitLog("item", action, count)
	}

	return createItemMaintenance(c, cfg, hostID)
}

func createItemMaintenance(c *client, cfg *config, hostID string) error {
	itemIDs, err := resolveItemIDs(c, hostID, cfg.filters)
	if err != nil {
		return err
	}
	if len(itemIDs) == 0 {
		return emitLog("item", "maintenance_create", 0)
	}

	triggerNames, err := fetchTriggerNames(c, itemIDs)
	if err != nil {
		return err
	}
	if len(triggerNames) == 0 {
		return emitLog("item", "maintenance_create", 0)
	}

	name := itemMaintenanceName(cfg.host, cfg.filters)
	existing, err := findMaintenances(c, name)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return emitLog("item", "maintenance_create", 0)
	}

	tags := make([]map[string]string, 0, len(triggerNames))
	for _, triggerName := range triggerNames {
		tags = append(tags, map[string]string{
			"tag":      tagKey,
			"value":    triggerName,
			"operator": tagOperator,
		})
	}
	if len(tags) == 0 {
		return emitLog("item", "maintenance_create", 0)
	}

	if err := c.call("maintenance.create", buildMaintenanceParams(name, hostID, 0, tags), nil); err != nil {
		if again, getErr := findMaintenances(c, name); getErr == nil && len(again) > 0 {
			return emitLog("item", "maintenance_create", 0)
		}
		return fmt.Errorf("maintenance.create 失败: %w", err)
	}
	return emitLog("item", "maintenance_create", len(triggerNames))
}

func getHostID(c *client, host string) (string, error) {
	var hosts []hostEntity
	err := c.call("host.get", map[string]interface{}{
		"filter": map[string]interface{}{"host": host},
		"output": []string{"hostid"},
	}, &hosts)
	if err != nil {
		return "", fmt.Errorf("host.get 失败: %w", err)
	}
	if len(hosts) == 0 {
		return "", fmt.Errorf("未找到主机: %s", host)
	}
	return hosts[0].HostID, nil
}

func resolveItemIDs(c *client, hostID string, filters []parsedFilter) ([]string, error) {
	seen := make(map[string]struct{})
	for _, f := range filters {
		var items []itemEntity
		switch f.kind {
		case "Template":
			var templates []templateEntity
			if err := c.call("template.get", map[string]interface{}{
				"filter": map[string]interface{}{"host": f.value},
				"output": []string{"templateid"},
			}, &templates); err != nil {
				return nil, fmt.Errorf("template.get 失败（%s）: %w", f.value, err)
			}
			if len(templates) == 0 {
				continue
			}
			templateIDs := make([]string, 0, len(templates))
			for _, t := range templates {
				templateIDs = append(templateIDs, t.TemplateID)
			}
			if err := c.call("item.get", map[string]interface{}{
				"hostids":     hostID,
				"templateids": templateIDs,
				"output":      []string{"itemid"},
			}, &items); err != nil {
				return nil, fmt.Errorf("item.get 失败（Template:%s）: %w", f.value, err)
			}
		case "Itemkey":
			if err := c.call("item.get", map[string]interface{}{
				"hostids": hostID,
				"filter":  map[string]interface{}{"key_": f.value},
				"output":  []string{"itemid"},
			}, &items); err != nil {
				return nil, fmt.Errorf("item.get 失败（Itemkey:%s）: %w", f.value, err)
			}
		}
		for _, item := range items {
			if item.ItemID != "" {
				seen[item.ItemID] = struct{}{}
			}
		}
	}
	return sortedStrings(seen), nil
}

func fetchTriggerNames(c *client, itemIDs []string) ([]string, error) {
	seen := make(map[string]struct{})
	for _, chunk := range chunkStrings(itemIDs, chunkSize) {
		var triggers []triggerEntity
		if err := c.call("trigger.get", map[string]interface{}{
			"itemids": chunk,
			"output":  []string{"triggerid", "description"},
		}, &triggers); err != nil {
			return nil, fmt.Errorf("trigger.get 失败: %w", err)
		}
		for _, trigger := range triggers {
			if trigger.Description != "" {
				seen[trigger.Description] = struct{}{}
			}
		}
	}
	return sortedStrings(seen), nil
}

func changeItemStatus(c *client, hostID string, filters []parsedFilter, fromStatus, toStatus string) (int, error) {
	itemIDs, err := resolveItemIDs(c, hostID, filters)
	if err != nil {
		return 0, err
	}
	if len(itemIDs) == 0 {
		return 0, nil
	}

	var targetIDs []string
	for _, chunk := range chunkStrings(itemIDs, chunkSize) {
		var items []itemEntity
		if err := c.call("item.get", map[string]interface{}{
			"itemids": chunk,
			"output":  []string{"itemid", "status"},
		}, &items); err != nil {
			return 0, fmt.Errorf("item.get 状态失败: %w", err)
		}
		for _, item := range items {
			if item.Status == fromStatus {
				targetIDs = append(targetIDs, item.ItemID)
			}
		}
	}
	if len(targetIDs) == 0 {
		return 0, nil
	}

	for _, chunk := range chunkStrings(targetIDs, chunkSize) {
		updates := make([]map[string]string, 0, len(chunk))
		for _, id := range chunk {
			updates = append(updates, map[string]string{"itemid": id, "status": toStatus})
		}
		if err := c.call("item.update", updates, nil); err != nil {
			return 0, fmt.Errorf("item.update 失败: %w", err)
		}
	}
	return len(targetIDs), nil
}

func findMaintenances(c *client, name string) ([]maintenanceEntity, error) {
	var maintenances []maintenanceEntity
	if err := c.call("maintenance.get", map[string]interface{}{
		"filter": map[string]interface{}{"name": name},
		"output": []string{"maintenanceid"},
	}, &maintenances); err != nil {
		return nil, fmt.Errorf("maintenance.get 失败: %w", err)
	}
	return maintenances, nil
}

func createHostMaintenance(c *client, hostID, name, mode string) (bool, error) {
	existing, err := findMaintenances(c, name)
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}

	maintenanceType := 0
	if mode == "without_data" {
		maintenanceType = 1
	}

	if err := c.call("maintenance.create", buildMaintenanceParams(name, hostID, maintenanceType, nil), nil); err != nil {
		if again, getErr := findMaintenances(c, name); getErr == nil && len(again) > 0 {
			return false, nil
		}
		return false, fmt.Errorf("maintenance.create 失败: %w", err)
	}
	return true, nil
}

func removeMaintenancesByName(c *client, name string) (int, error) {
	existing, err := findMaintenances(c, name)
	if err != nil {
		return 0, err
	}
	if len(existing) == 0 {
		return 0, nil
	}

	ids := make([]string, 0, len(existing))
	for _, m := range existing {
		ids = append(ids, m.MaintenanceID)
	}

	if err := c.call("maintenance.delete", ids, nil); err != nil {
		if remaining, getErr := findMaintenances(c, name); getErr == nil && len(remaining) == 0 {
			return len(ids), nil
		}
		return 0, fmt.Errorf("maintenance.delete 失败: %w", err)
	}
	return len(ids), nil
}

func buildMaintenanceParams(name, hostID string, maintenanceType int, tags []map[string]string) map[string]interface{} {
	now := time.Now().Unix()
	params := map[string]interface{}{
		"name":             name,
		"maintenance_type": maintenanceType,
		"hosts":            []map[string]string{{"hostid": hostID}},
		"active_since":     now,
		"active_till":      now + maintenanceSpan,
		"timeperiods": []map[string]interface{}{
			{
				"timeperiod_type": 0,
				"start_date":      now,
				"period":          maintenanceSpan,
			},
		},
	}
	if len(tags) > 0 {
		params["tags"] = tags
		params["tags_evaltype"] = tagsEvalType
	}
	return params
}

func emitLog(target, action string, count int) error {
	record := logRecord{
		Timestamp: time.Now().Unix(),
		Target:    target,
		Action:    action,
		Count:     count,
		Status:    "success",
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化日志失败: %w", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return fmt.Errorf("写入日志失败: %w", err)
	}
	return nil
}
