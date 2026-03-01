### 整合数据
```python
a=[
    {"col1": "11", "col2": "12", "col3": ""},
    {"col1": "21", "col2": "other", "col3": "23"},
    {"col1": "31", "col2": "32", "col3": ""},
]

for r in a:
    if r.get("col2") == "other":
        r["col2"] = r.get("col3") # type: ignore
    r.pop("col3", None)

print(a)
```
~~~
```python
A = [{"syscode": "AAA", "env": "dev", "uuid": "111"},
     {"syscode": "BBB", "env": "dev", "uuid": "222"},
     {"syscode": "CCC", "env": "dev", "uuid": "333"}]
B = [{"SystemCde": "AAA", "setting_key": "111", "Env": "dev"},
     {"SystemCde": "BBB", "setting_key": "222", "Env": "dev"},
     {"SystemCde": "CCC", "setting_key": "333", "Env": "prod"},
     {"SystemCde": "DDD", "setting_key": "444", "Env": "dev"}]

# 字段映射（A字段名 -> B字段名）
field_map = {
    "syscode": "SystemCde",
    "uuid": "setting_key",
    "env": "Env"          # 用于比较的额外字段
}

key_fields = ["syscode", "uuid"]   # 作为唯一键的字段

def make_key(item, source='A'):
    """生成标准化键（元组）"""
    if source == 'A':
        return tuple(item[f] for f in key_fields)
    else:  # B
        return tuple(item[field_map[f]] for f in key_fields)

# 构建键到原始字典的映射
dictA = {make_key(d, 'A'): d for d in A}
dictB = {make_key(d, 'B'): d for d in B}


keysA = set(dictA.keys())
keysB = set(dictB.keys())
print("dictA:", keysA)
print("dictB:", keysB)

common_keys = keysA & keysB
only_A_keys = keysA - keysB
only_B_keys = keysB - keysA

# 需要比较的非键字段（排除键字段后的映射）
compare_fields = [(fA, fB) for fA, fB in field_map.items() if fA not in key_fields]

inconsistent = []
only_in_A = []
only_in_B = []

# 处理键相同的记录，检查一致性
for key in common_keys:
    a_dict = dictA[key]
    b_dict = dictB[key]
    is_consistent = True
    for fA, fB in compare_fields:
        if a_dict.get(fA) != b_dict.get(fB):
            is_consistent = False
            break
    if not is_consistent:
        syscode, uuid = key
        inconsistent.append({"syscode": syscode, "uuid": uuid})

# 处理仅存在于A的记录
for key in only_A_keys:
    syscode, uuid = key
    only_in_A.append({"syscode": syscode, "uuid": uuid})

# 处理仅存在于B的记录
for key in only_B_keys:
    syscode, uuid = key
    only_in_B.append({"syscode": syscode, "uuid": uuid})

print("not equal:", inconsistent)
print("only in A:", only_in_A)
print("only in B:", only_in_B)
```
### 发送请求
```python
import requests

BASE_URL = "https://dev292235.service-now.com/api/now/table/u_table"
AUTH = ("admin", "q!5TEwDl1Cy$")
HEADERS = {"Content-Type": "application/json", "Accept": "application/json"}

def fetch_all_rows(query, fields, page_size):
    """
    不断请求下一页，直到没有数据为止。
    返回一个包含所有记录的列表。
    """
    offset = 0
    all_results = []

    while True:
        params = {
            "sysparm_query": query,
            "sysparm_limit": page_size,
            "sysparm_offset": offset,
            "sysparm_fields": fields
        }

        resp = requests.get(BASE_URL,
                            auth=AUTH,
                            headers=HEADERS,
                            params=params)
        if resp.status_code != 200: 
            print('Status:', resp.status_code, 'Headers:', resp.headers, 'Error Response:',resp.json())
            exit()

        data = resp.json()

        rows = data.get("result", [])
        if not rows:
            break                       # 没有更多了就退出

        all_results.extend(rows)
        offset += len(rows)             # 移动到下一页

    return all_results

records = fetch_all_rows(query="u_syscodeSTARTSWITHAAA^u_envSTARTSWITHdev",
                         fields="u_syscode,u_env,u_col1,u_col2,u_col3,u_col4,u_col5",
                         page_size=10000)
col1_values = []
for item in records:
    col1_values.append(item['u_col1'])
print(col1_values)
```


### ddb
```python
import boto3

def lambda_handler(event, context):
    # 1. 创建 DynamoDB 客户端
    table_name = 'YourTableName' # 替换为你的真实表名
    dynamodb = boto3.client('dynamodb') # 使用 client 而非 resource

    all_items = []

    # 2. 创建分页器
    paginator = dynamodb.get_paginator('scan')

    # 3. 设置分页参数并迭代每一页
    page_iterator = paginator.paginate(TableName=table_name)

    for page in page_iterator:
        # 处理每一页中的 Items
        # 注意：使用 client 时，返回的 Item 属性值是字典格式，如 {'S': 'value'}
        for item in page.get('Items', []):
            # 这里可以对每个 item 进行处理，或直接添加到列表
            all_items.append(item)

    # 4. 返回所有数据
    print(f"成功获取 {len(all_items)} 条记录")
    return {
        'statusCode': 200,
        'body': all_items
    }
```
