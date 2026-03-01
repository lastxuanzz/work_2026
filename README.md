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

```python

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
