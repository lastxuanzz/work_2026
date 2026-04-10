# work_2026
对于EC2中的安全组字段，<br>
要创建两个<br>
1.安全组，类型：List Collector 👉 这是实际要使用的变量，plan或apply时要用<br>
2.安全组_temp，类型：Select Box 👉 这是辅助用的变量，仅仅是为了给上述的变量赋值用<br>
<br>
对于1.安全组，在Type Specifications，[List table]选择Question Choice，<br>
Reference qualifier中写： question=*sys_id*<br>
上述*sys_id*的值为2.安全组_temp的sysid,这保证页面中1.安全组的下拉框是空的，不能出现无关的值<br>

<br>
2.安全组_temp 绑定onChange事件（对于变量集）<br>

```javascript
function onChange(control, oldValue, newValue, isLoading) {
    // 1. 如果正在加载，或者当前改变的值为空，则不执行逻辑
    if (isLoading || newValue === '') {
        return;
    }

    // 2. 获取目标变量 sg 的当前值
    var targetField = 'sg'; // 这是实际要用的安全组字段名
    var currentStr = g_form.getValue(targetField);

    // 3. 将字符串转为数组，并过滤掉可能存在的空项
    // .filter(Boolean) 能确保如果原字符串为空，我们得到的是一个干净的空数组 [] 而不是 [""]
    var arr = currentStr ? currentStr.split(',').filter(Boolean) : [];

    // 4. 检查要追加的 newValue 是否已经在 sg 里面了
    if (arr.indexOf(newValue) === -1) {
        // 确保使用 newValue 而不是未定义的 newItem
        arr.push(newValue);

        // 5. 重新拼接并赋值
        var updatedStr = arr.join(',');
        g_form.setValue(targetField, updatedStr);
    }
}

```

<br>

可选：提交变量集的表单时，清空2.安全组_temp的值<br>

```javascript
function onSubmit() {
   //Type appropriate comment here, and begin script below
   g_form.clearValue('sg_temp');
}

```
