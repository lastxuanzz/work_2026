# ServiceNow SP：为指定 Catalog Item 增加独立 Tab

## 目的

在 ServiceNow Service Portal（SP）的 `My Request` / `Standard Ticket` 页面中：

- 仅针对指定的 Catalog Item（例如 `AAA`）显示新的 Tab。
- 其他 Catalog Item 提交的 RITM 不显示该 Tab。
- 不修改 OOB 的 Standard Ticket 页面或 Widget。
- Tab 内部通过 Custom Widget 显示自定义内容。

---

## 1. 创建 Custom Widget

进入：

```text
Service Portal
→ Widgets
→ New
```

例如：

```text
Name:
AAA Request Details

ID:
aaa_request_details
```

> **注意：Widget 的 `ID` 必须填写。**

先创建一个最简单的 Widget 进行测试。

### HTML Template

```html
<div style="padding:20px;">
    <h3>AAA Details</h3>
    <p>AAA Custom Widget is working.</p>
</div>
```

### Client Script

```javascript
function() {
    var c = this;
}
```

### Server Script

```javascript
(function() {

})();
```

保存 Widget。

---

## 2. 找到 Standard Ticket Configuration

进入：

```text
Standard Ticket
→ Standard Ticket Configuration
```

找到：

```text
Table = Requested Item [sc_req_item]
```

打开该配置。

---

## 3. 新增 Tab Configuration

在：

```text
Tab Configurations
```

中点击：

```text
New
```

设置：

```text
Type:
Custom

Tab name:
AAA Details

Order:
例如 4
```

然后在 `Widget` 中选择刚刚创建的：

```text
AAA Request Details
```

---

## 4. 设置只有 AAA Catalog Item 才显示

在 Tab Configuration 中设置：  
打开 Advanced  
在Visible(Script)中使用以下的js：  
```javascript
 answer = false;

if (current.cat_item == '628013167c86408ea7e26950c36133d2') {  // catalog item的sysid
    answer = true;
}
````


---

## 5. 测试 Tab

进入：

```text
Home
→ My Request
→ RITM0010xxx
```

确认页面下方出现：

```text
Activity | Attachments | Additional Details | AAA Details
```

点击：

```text
AAA Details
```

应该可以看到 Custom Widget 中的内容。

---

## 6. 将 Widget 改为读取当前 RITM

测试静态内容正常后，再修改 Server Script。

推荐使用：

```javascript
(function() {

    var gr = $sp.getRecord();

    if (!gr || !gr.isValidRecord()) {
        data.error = "Current record not found.";
        return;
    }

    data.number = gr.getValue("number");
    data.short_description = gr.getValue("short_description");
    data.cat_item = gr.cat_item.getDisplayValue();

})();
```

对应 HTML：

```html
<div style="padding:20px;">

    <h3>AAA Details</h3>

    <p>
        <strong>RITM Number:</strong>
        {{::c.data.number}}
    </p>

    <p>
        <strong>Catalog Item:</strong>
        {{::c.data.cat_item}}
    </p>

    <p>
        <strong>Short Description:</strong>
        {{::c.data.short_description}}
    </p>

    <p ng-if="c.data.error">
        <strong>Error:</strong>
        {{::c.data.error}}
    </p>

</div>
```

---

## 7. 最终结构

```text
Standard Ticket Configuration
└── Table: sc_req_item
    └── Tab Configurations
        ├── Activity
        ├── Attachments
        ├── Additional Details
        └── AAA Details
            ├── Type = Custom
            ├── Visible = Item is AAA
            └── Widget = AAA Request Details
```

---

## 8. 实现结果

### AAA Catalog Item

```text
Activity
Attachments
Additional Details
AAA Details
```

### 其他 Catalog Item

```text
Activity
Attachments
Additional Details
```

因此：

* 不需要修改 Standard Ticket OOB Widget。
* 不需要修改 `standard_ticket` 页面。
* 不会影响其他 Catalog Item。
* AAA 专用的内容全部放在自己的 Custom Widget 中。
* 后续可以在 Custom Widget 中查询 RITM、Catalog Variables、自定义表、审批信息等。
