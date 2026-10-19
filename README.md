# Zabbix 微观监控管理工具 (zabbix_kanshi) 技术设计与开发规范文档

## 1. 命令行接口 (CLI) 规范

程序必须严格支持以下两种命令格式。入参大小写敏感。

### 格式一：主机级别控制
```bash
zabbix_kanshi -target host -status <on|off> -host <hostname> -mode <with_data|without_data>
```

### 格式二：监控项级别控制
```bash
zabbix_kanshi -target item -status <on|off> -host <hostname> -filter "<type>:<name>" [-filter "<type>:<name>" ...] -mode <with_data|without_data>
```

### 参数值约束列表：
*   `-target`：必须是 `host` 或 `item` 之一。
*   `-status`：
   *   `off`：**关闭监视**（即进入维护/抑制告警状态）。
   *   `on`：**开启监视**（即解除维护/恢复正常监控）。
*   `-host`：目标主机的唯一名称（对应 Zabbix 的 `host` 字段）。
*   `-mode`：必须是 `with_data` 或 `without_data` 之一。
*   `-filter`：当 `target` 为 `item` 时为**必填项**，且**可以重复传入多个**（至少一个）。格式为固定字符串 `"Type:Value"`，按**第一个冒号**分割，类型部分大小写敏感。
   *   **情况 A（按模板过滤）**：`"Template:<模板技术名称>"`（例如 `"Template:Linux by Zabbix agent"`）
   *   **情况 B（按监控项键值过滤）**：`"Itemkey:<精确的键值>"`（例如 `"Itemkey:vfs.fs.size[/,free]"`）
   *   多个 `-filter` 允许混用 `Template` 与 `Itemkey`，最终结果取各 filter 命中 item 的**并集**并去重。
*   `-filter` 仅适用于 `target=item`；`target=host` 时不允许传入。

---

## 2. 命名与硬编码规范（解决无状态记忆问题）

为了使脚本不依赖任何外部数据库即可实现“精准解除维护”，程序内必须严格硬编码并遵守以下规范：

1.  **主机维护周期（Maintenance）名称规则**：
   *   由脚本生成的主机级原生维护周期名称固定为：
       `"Kanshi_HostMaint_" + hostname`
2.  **监控项维护周期（Maintenance）名称规则**：
   *   先对每个 `-filter` 归一化为 `"<filter_type>_<filter_value>"`（去除首尾空白）；
   *   对归一化后的结果去重，并按字典序升序排序；
   *   使用 `+` 拼接所有结果，得到 `filter_token`；
   *   监控项维护周期名称固定为：
       `"Kanshi_Maint_" + hostname + "_item_" + filter_token`
   *   若名称长度超过 Zabbix 限制（128 字符），则取完整名称的前 115 个字符，并追加 `"_" + 完整名称的 SHA-1 前 12 位十六进制字符`（总计 128 字符），保证名称确定且可重复生成。
   *   `filter` 的传入顺序不影响名称，`on` 与 `off` 必须能够生成完全一致的名称。
3.  **维护过滤标签（Tag）匹配规则**：
   *   维护周期用于匹配触发器的标签 Key 固定为：`Kanshi-Maint-Key`
   *   标签 Value 取触发器名称（即 `trigger.get` 返回的 `description` 字段），每个去重后的触发器名称生成一个标签项。
   *   标签项的 `operator` 固定为 `"0"`（等于匹配）。
   *   多个标签项之间使用 `tags_evaltype = 2`（Or）语义：任一触发器名称命中即抑制对应问题。
   *   当目标触发器名称为空（无匹配）时，**禁止创建 tags 为空的维护周期**（空 tags 会抑制主机上的全部问题）。
4.  **时间段硬编码（Time Period）**：
   *   调用 `maintenance.create` 时，硬编码 `active_since` 为当前时间戳，`active_till` 为当前时间戳 + 315360000 秒（10年）。
   *   内部 `timeperiods` 数组中：`timeperiod_type = 0`（仅一次），`start_date` 为当前时间戳，`period` 为 315360000 秒。

---

## 3. 详细核心业务逻辑与 API 调用序列

在执行任何 `-target` 分支逻辑前，程序必须首先执行**前置通用动作**：
*   **API 动作**：调用 `host.get`，传入 `filter={"host": hostname}`。
*   **逻辑**：获取对应的 `hostid`。如果找不到对应的物理主机，程序必须向 `STDERR` 打印错误并以非 0 状态码退出。

### 3.1 逻辑一：`-target host`

#### 【场景 1】 `-status off`（关闭主机监视）
1.  根据 `-mode` 参数确定 Zabbix 的 `maintenance_type` 字段值：
   *   `with_data` → `maintenance_type = 0`（收集数据）
   *   `without_data` → `maintenance_type = 1`（不收集数据）
2.  **API 动作**：首先调用 `maintenance.get` 检查名称为 `"Kanshi_HostMaint_" + hostname` 的维护周期是否已存在。如果已存在，则跳过创建，确保**幂等性**。
3.  **API 动作**：若不存在，则调用 `maintenance.create`。
   *   `name` 设定为 `"Kanshi_HostMaint_" + hostname`
   *   `hosts` 传入 `[{"hostid": hostid}]`
   *   传入硬编码的 10 年时间段参数。

#### 【场景 2】 `-status on`（开启主机监视）
1.  **API 动作**：调用 `maintenance.get`，查询 `name = "Kanshi_HostMaint_" + hostname`，获取其 `maintenanceid`。
2.  **API 动作**：如果存在，调用 `maintenance.delete`，传入 `[maintenanceid]` 进行删除。如果不存在，则视为已解除，静默成功。

---

### 3.2 逻辑二：`-target item`

#### 🔍 对象定位查询（按场景按需执行）
1.  **解析 Filter 列表**：
   *   收集所有 `-filter` 参数，逐个解析为 `(filter_type, filter_value)`。
   *   每个 filter 的归一化字符串为 `"<filter_type>_<filter_value>"`，供维护周期名称计算使用。
2.  **解析目标 Item（`off + with_data`、`off/on + without_data` 需要）**：
   *   遍历每个 filter，分别查询后合并结果：
      *   若为 `Template:<name>`：先调用 `template.get`，传入 `filter={"host": name}` 查到对应的 `templateid`；再调用 `item.get`，传入 `hostids=hostid` 和 `templateids=templateid`。
      *   若为 `Itemkey:<name>`：直接调用 `item.get`，传入 `hostids=hostid` 和 `filter={"key_": name}`（精确匹配键值）。
   *   对所有查询结果按 `itemid` 去重，得到目标 `itemid` 列表。
3.  **联动查询 Trigger（仅 `off + with_data` 需要）**：
   *   调用 `trigger.get`，传入 `itemids=itemid列表`，`output=["triggerid","description"]`。
   *   按 `description`（触发器名称）去重，得到 `trigger_names` 列表；维护标签的 Value 直接使用这些名称，无需再查询触发器标签。
4.  **空结果处理**：
   *   若目标 `itemid` 列表为空，或 `off + with_data` 场景下 `trigger_names` 为空，则视为无匹配对象：输出 `count=0` 的成功日志，不创建维护、不修改对象，且不得将空列表传入批量更新接口。
5.  **`on + with_data` 例外**：
   *   解除维护只需根据 filter 计算出维护周期名称，不要求目标 item/trigger 仍然存在，因此无需执行上述 item/trigger 定位查询。

#### 【场景 1】 `-status off`（关闭监控项监视）
*   **分支 A：`-mode with_data`**
   1.  根据字符串拼接公式生成维护周期名称与 `trigger_names` 标签列表。
   2.  若 `trigger_names` 为空，输出 `count=0` 成功日志并结束。
   3.  **API 动作**：调用 `maintenance.get`，传入 `filter={"name": 维护周期名称}` 检查是否已存在。如果已存在，则跳过创建，确保**幂等性**。
   4.  **API 动作**：若不存在，则调用 `maintenance.create`。
       *   `name` 为按规则生成的维护周期名称
       *   `maintenance_type = 0`
       *   `hosts = [{"hostid": hostid}]`
       *   `tags = [{"tag": "Kanshi-Maint-Key", "value": "<trigger_name>", "operator": "0"}, ...]`（每个去重后的 `trigger_name` 一项）
       *   `tags_evaltype = 2`（Or）
       *   传入硬编码的 10 年时间段参数
*   **分支 B：`-mode without_data`**
   1.  **API 动作**：调用 `item.get`，传入前面获取到的 `itemid` 列表，获取各 item 当前的 `status`，并过滤出其中 **`status == 0`（即当前处于正常启用状态）** 的 Item。
   2.  **API 动作**：调用 `item.update`，将这些过滤出来的 Item 的 `status` 批量修改为 `1`（禁用）。*(跳过原本就已经禁用的监控项，实现状态记忆与恢复保护)*
   3.  **说明**：该分支直接禁用 Item，不查询、不修改关联 Trigger。

#### 【场景 2】 `-status on`（开启监控项监视）
*   **分支 A：`-mode with_data`**
   1.  根据字符串拼接公式生成维护周期名称。
   2.  **API 动作**：调用 `maintenance.get`，传入 `filter={"name": 维护周期名称}` 获取 `maintenanceid` 列表。
   3.  **API 动作**：若存在，调用 `maintenance.delete` 删除查到的维护周期（若存在多个同名项则全部删除）；若不存在，则视为已解除，静默成功。
   4.  **说明**：该分支只删除维护周期，不修改任何 Trigger 标签。
*   **分支 B：`-mode without_data`**
   1.  **API 动作**：在目标 `itemid` 列表中查询各 item 当前的 `status`，过滤出当前 **`status == 1`（当前处于禁用状态）** 的目标 Item。
   2.  **API 动作**：调用 `item.update`，将这些 Item 的 `status` 批量修改为 `0`（重新启用），恢复数据采集。
   3.  **说明**：该分支不查询、不修改关联 Trigger。

> **配对约束**：`-status on -mode with_data` 必须使用与 `-status off` 相同的 `-filter` 集合（顺序可以不同）才能匹配到同一维护周期。filter 集合不同时，视为不同的维护周期，不会互相删除。

---

## 4. 异常处理与非功能性要求

1.  **严格的幂等性（Idempotence）**：
   *   脚本必须支持连续多次运行相同的 `-status off` 或 `-status on` 命令。
   *   在执行 `create` 类接口前必须先通过 `get` 检查；在执行 `delete` 前也必须先通过 `get` 定位。若已处于目标状态，程序应当静默成功或打印提示日志，绝不能因 Zabbix API 抛出对象已存在（如 Maintenance conflict）或对象不存在等异常而直接崩溃。
2.  **空匹配安全约束**：
   *   当 filter 未匹配到任何 Item 或 Trigger 时，程序视为成功（`count=0`），不得创建维护周期。
   *   严禁调用 `maintenance.create` 时传入空的 `tags` 数组，避免误抑制主机上的全部问题。
3.  **认证机制（Authentication，临时 Token 模式）**：
   *   **环境变量**：脚本所需以下参数必须从环境变量中获取：
       *   `ZABBIX_URL`：Zabbix API 地址。
       *   `ZABBIX_USER`：登录用户名。
       *   `ZABBIX_PASSWORD`：登录密码。
       *   上述任一环境变量缺失或为空时，必须向 `STDERR` 输出错误并以非 0 状态码退出。
   *   **获取临时 Token**：脚本在初始化阶段必须先调用 `user.login`，使用环境变量中的 `ZABBIX_USER` 与 `ZABBIX_PASSWORD` 获取临时 `auth_token`。
   *   **使用临时 Token**：后续所有的 HTTP 请求必须在请求头（Header）或 JSON Body 中携带该临时认证 Token。
   *   **注销临时 Token**：业务逻辑执行完毕（无论成功或异常）或进程退出前，必须调用 `user.logout` 注销当前临时 Token，使其立即失效。
   *   **登录失败处理**：若 `user.login` 失败，则视为认证失败，不得调用 `user.logout`，必须向 `STDERR` 输出明确的错误信息并以非 0 状态码退出。
   *   **注销失败处理**：若 `user.logout` 失败，仅向 `STDERR` 输出警告信息，不得改变业务逻辑本身的退出码。
   *   **Token 安全**：Token 禁止打印到日志、写入文件或出现在任何错误信息中。
4.  **日志与退出码规范**：
   *   **成功变更**：程序执行成功且发生实质状态变更后，必须向标准输出（`STDOUT`）打印标准的 JSON 格式日志，如：
       `{"timestamp": 1698123456, "target": "item", "action": "maintenance_create", "count": 5, "status": "success"}`
   *   `action` 取值约定：
      *   `with_data` + `off` → `maintenance_create`
      *   `with_data` + `on` → `maintenance_delete`
      *   `without_data` + `off` → `disable`
      *   `without_data` + `on` → `enable`
   *   `count` 取值约定：
      *   `maintenance_create`：本次维护周期中包含的唯一触发器名称数量
      *   `maintenance_delete`：本次删除的维护周期数量
      *   `disable` / `enable`：本次实际发生状态变更的 Item 数量
      *   无匹配对象或无需变更时为 `0`
   *   **异常失败**：若发生网络错误、API 凭证失效、参数非法或主机/对象定位失败等异常，必须向标准错误（`STDERR`）输出明确的错误信息，并以非 0 状态码（`Exit Code > 0`）终止程序。filter 无匹配不属于异常，按上述空匹配规则输出 `count=0` 的成功日志。
