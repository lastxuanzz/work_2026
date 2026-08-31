# AWS Lambda 与 Azure Blob Storage 交互实施说明

## 1. 场景说明

当前场景：

- Azure Storage 已经由其他团队/人员创建
- 需要在 AWS 侧创建 Lambda
- Lambda 需要读取 Azure Blob Storage 中存储的文件
- 目前不清楚 Azure Storage 的具体认证和网络配置

整体逻辑通常如下：

```text
┌──────────────────────┐
│     AWS Lambda       │
│                      │
│  Python / Node.js    │
└──────────┬───────────┘
           │ HTTPS 443
           │
           ▼
      Internet / VPN
           │
           ▼
┌────────────────────────────┐
│ Azure Storage Account      │
│                            │
│  Blob Container            │
│    ├── file1.csv           │
│    ├── file2.json          │
│    └── xxx.zip             │
└────────────────────────────┘
```

本质上：

```text
AWS Lambda
    ↓ HTTPS
Azure Blob Storage API
```

真正需要重点确认的是：

1. Azure Blob Storage 如何认证
2. AWS Lambda 是否能够通过网络访问 Azure Storage
3. Azure 凭证如何安全保存
4. 文件大小、文件数量以及后续处理方式

---

# 2. 首先向 Azure 管理员确认的信息

由于 Azure Storage 不是自己创建的，建议在创建 Lambda 前，先向 Azure 管理员确认相关信息。

## 2.1 Storage Account 名称

例如：

```text
mystorageaccount
```

Azure Blob Storage 的地址通常类似：

```text
https://mystorageaccount.blob.core.windows.net
```

需要确认：

```text
Storage Account Name
```

---

## 2.2 Container 名称

Azure Blob Storage 的结构通常如下：

```text
Storage Account
    │
    ├── Container A
    │      ├── file1.csv
    │      └── file2.csv
    │
    └── Container B
           └── report.json
```

例如：

```text
Storage Account:
mystorageaccount

Container:
reports
```

文件可能是：

```text
report.csv

2026/report.csv

logs/app/xxx.json
```

因此需要确认：

- Container 名称
- Blob 文件完整路径
- 文件名规则

---

## 2.3 文件是固定的还是动态变化的

这一点会直接影响 Lambda 的代码设计。

### 情况 A：固定文件名

例如：

```text
reports/daily.csv
```

Lambda 可以直接下载指定文件。

---

### 情况 B：文件名按照日期变化

例如：

```text
reports/report_20260831.csv
reports/report_20260901.csv
```

Lambda 需要根据日期自动生成 Blob 名称。

例如：

```text
report_YYYYMMDD.csv
```

---

### 情况 C：文件名随机生成

例如：

```text
reports/abc123.csv
reports/xyz456.csv
```

这种情况下 Lambda 可能需要：

```text
先 List Blob
    ↓
找到符合条件的文件
    ↓
下载文件
```

因此需要确认：

> Lambda 是否能够直接知道完整 Blob 路径，还是需要先查询 Container 中有哪些文件？

---

# 3. 最关键的问题：Azure 使用什么认证方式

AWS Lambda 访问 Azure Blob Storage 时，Azure 必须提供认证方式。

常见方式包括：

1. SAS Token
2. Storage Account Key
3. Connection String
4. Microsoft Entra ID / Service Principal

---

# 4. 方案一：SAS Token（推荐用于简单跨云访问）

对于以下场景：

> AWS Lambda 访问其他团队管理的 Azure Blob Storage

SAS Token 通常是比较容易实施的方案。

Azure 管理员可能提供完整 URL，例如：

```text
https://mystorageaccount.blob.core.windows.net/reports/file.csv?sv=xxxx&sp=r&se=xxxx&sig=xxxx
```

也可能分别提供：

```text
Storage Account URL
+
SAS Token
```

例如：

```text
Account URL:
https://mystorageaccount.blob.core.windows.net

SAS Token:
?sv=2025-xx-xx&sp=rl&se=xxxx&sig=xxxx
```

## 4.1 权限建议

如果 Lambda 只需要读取文件：

```text
Read
```

如果 Lambda 需要先查询文件列表再读取：

```text
List + Read
```

不建议提供：

```text
Write
Delete
Create
```

推荐遵循最小权限原则。

---

## 4.2 SAS Token 过期问题

必须确认：

```text
SAS Token 有效期是多久？
```

例如：

```text
2026-09-01
~
2027-09-01
```

SAS Token 过期后：

```text
Lambda
   ↓
认证失败
   ↓
无法访问 Azure Blob Storage
```

因此需要提前明确：

- 谁负责更新 SAS Token
- Token 到期前如何通知
- 更新后如何更新 AWS Secrets Manager 中的凭证

---

# 5. 方案二：Storage Account Key

Azure 管理员可能提供：

```text
Account Name:
mystorageaccount

Account Key:
xxxxxxxxxxxxxxxxxxxxx
```

Python Azure SDK 可以使用 Account Key 认证。

但是这种方式通常权限较大。

如果 Lambda 只需要：

```text
读取某个 Container 中的文件
```

直接提供整个 Storage Account 的 Key 可能权限过大。

因此一般情况下：

```text
SAS Token
```

比：

```text
Storage Account Key
```

更适合最小权限控制。

---

# 6. 方案三：Connection String

Azure 管理员也可能直接提供 Connection String，例如：

```text
DefaultEndpointsProtocol=https;
AccountName=mystorageaccount;
AccountKey=xxxxx;
EndpointSuffix=core.windows.net
```

Python Azure SDK 可以直接使用 Connection String。

但是需要注意：

> Connection String 通常包含 Storage Account Key。

因此安全性问题与直接使用 Account Key 类似。

---

# 7. 方案四：Microsoft Entra ID / Service Principal

这是企业环境中更加规范的认证方式。

整体流程可能如下：

```text
AWS Lambda
     │
     │ Client ID
     │ Client Secret / Certificate
     ▼
Microsoft Entra ID
     │
     │ 获取 Access Token
     ▼
Azure Blob Storage
```

Azure 管理员需要配置身份，并提供类似：

```text
Tenant ID
Client ID
Client Secret
```

同时需要给该身份配置 Azure Blob Storage 的相应 RBAC 权限。

例如只允许读取 Blob 数据。

---

## 7.1 什么时候推荐使用 Entra ID

适合：

```text
生产环境
长期运行
企业安全要求较高
Azure 身份管理比较规范
```

建议：

### 简单实施方案

```text
SAS Token
```

### 企业长期生产方案

```text
Microsoft Entra ID / Service Principal
```

---

# 8. 必须确认 Azure Storage 的网络配置

即使认证信息正确，也不代表 Lambda 一定能够访问 Azure Storage。

必须确认 Azure Storage 的网络限制。

---

# 9. 网络情况 A：Azure Storage 允许公网访问

架构：

```text
AWS Lambda
     │
     │ HTTPS 443
     ▼
Internet
     │
     ▼
Azure Blob Storage
```

这种情况最简单。

Lambda 只需要能够访问：

```text
https://<storage-account>.blob.core.windows.net
```

通常不需要复杂的 VPC 网络配置。

---

# 10. 网络情况 B：Azure Storage 配置了 IP 白名单

Azure Storage 可能只允许指定 IP：

```text
Allow:
1.2.3.4
5.6.7.8
```

此时需要让 Azure 管理员允许 AWS Lambda 的出口 IP。

但是：

> 普通 Lambda 的公网出口 IP 通常不是固定的。

因此不能简单地把 Lambda 的 IP 加入 Azure 白名单。

---

## 推荐架构：NAT Gateway + Elastic IP

```text
Lambda
   │
   ▼
Private Subnet
   │
   ▼
NAT Gateway
   │
   │ 固定 Elastic IP
   ▼
Internet
   │
   ▼
Azure Blob Storage
```

Azure Firewall 中允许：

```text
AWS NAT Gateway 的 Elastic IP
```

这样：

```text
Lambda 出网
    ↓
NAT Gateway
    ↓
固定 Elastic IP
    ↓
Azure Blob Storage
```

这是跨云访问中非常常见的设计。

---

# 11. Lambda 是否需要放到 VPC

取决于 Azure 的网络要求。

## 情况 A：Azure Storage 允许公网访问，且没有 IP 白名单要求

可以：

```text
Lambda
   ↓
Internet
   ↓
Azure Blob Storage
```

这种情况下通常可以不把 Lambda 放进 VPC。

---

## 情况 B：Azure Storage 要求固定 IP 白名单

建议：

```text
Lambda
   ↓
Private Subnet
   ↓
NAT Gateway
   ↓
Elastic IP
   ↓
Azure Blob Storage
```

Azure 管理员将 Elastic IP 加入允许列表。

---

## 情况 C：Azure Storage 禁止公网访问，只允许 Private Endpoint

例如：

```text
Public Network Access:
Disabled
```

这种情况下就不能简单通过公网访问。

可能需要：

```text
AWS VPC
   │
   │ VPN / Direct Connect / 私网连接
   │
   ▼
Azure VNet
   │
   ▼
Private Endpoint
   │
   ▼
Azure Blob Storage
```

需要进一步确认：

- Azure 是否配置 Private Endpoint
- Azure VNet 是否已经与 AWS 建立网络连接
- 是否有 Site-to-Site VPN
- 是否有 ExpressRoute / Direct Connect
- DNS 如何解析 Private Endpoint

这种情况需要跨云网络架构设计，不是单纯创建 Lambda 就能解决。

---

# 12. 建议向 Azure 管理员确认的完整问题清单

可以直接把下面的内容发给 Azure 管理员。

## A. Storage 基本信息

```text
1. Storage Account Name 是什么？

2. Blob Container Name 是什么？

3. Lambda 需要访问哪些文件？

4. 文件完整路径是什么？

5. 文件名是固定的还是每天变化？

6. Lambda 是否需要列出 Container 中的文件？
```

---

## B. 认证信息

```text
1. 推荐使用哪种认证方式？

   - SAS Token
   - Service Principal / Entra ID
   - Storage Account Key
   - Connection String

2. Lambda 需要什么权限？

   - Read
   - List + Read

3. 凭证有效期多久？

4. 凭证过期后的更新流程是什么？
```

---

## C. 网络信息

```text
1. Storage Account 是否允许 Public Network Access？

2. 是否限制 IP 白名单？

3. 如果限制 IP：
   是否可以将 AWS Lambda 的固定出口 IP 加入白名单？

4. 是否使用 Azure Private Endpoint？

5. Azure Storage 是否只能通过 Private Endpoint 访问？
```

---

# 13. 推荐的简单 POC 方案

如果 Azure 环境满足：

```text
Storage Account
   │
   ├── Public Network Access：Enabled
   │
   ├── 指定 Container
   │
   └── SAS Token：Read / List
```

AWS 侧可以：

```text
AWS Lambda
   │
   ├── Python
   ├── azure-storage-blob
   ├── AWS Secrets Manager
   └── CloudWatch Logs
```

架构：

```text
Lambda
   │ HTTPS 443
   ▼
Azure Blob Storage
```

这是最容易实现的 POC 方案。

---

# 14. AWS 侧实施步骤

## Step 1：创建 AWS Secrets Manager

不要把 SAS Token 直接写进 Lambda 代码。

建议保存到：

```text
AWS Secrets Manager
```

例如 Secret：

```json
{
  "azure_storage_account": "mystorageaccount",
  "azure_container": "reports",
  "azure_sas_token": "?sv=xxxx&sig=xxxx"
}
```

---

# 15. Step 2：创建 Lambda IAM Role

Lambda 至少需要：

## CloudWatch Logs 权限

例如：

```text
logs:CreateLogGroup
logs:CreateLogStream
logs:PutLogEvents
```

## Secrets Manager 权限

例如：

```text
secretsmanager:GetSecretValue
```

建议只允许访问指定 Secret ARN，而不是：

```text
Resource: *
```

遵循最小权限原则。

---

# 16. Step 3：准备 Azure Python SDK

Python Lambda 环境中需要 Azure Blob SDK。

例如：

```bash
pip install azure-storage-blob -t package/
```

目录可能如下：

```text
package/
    │
    ├── azure/
    ├── azure_storage_blob...
    └── lambda_function.py
```

然后打包成 ZIP 并部署到 Lambda。

另一种方式是使用：

```text
Lambda Layer
```

如果以后多个 Lambda 都需要访问 Azure Blob Storage，可以考虑把：

```text
azure-storage-blob
```

放到 Lambda Layer 中。

---

# 17. Python 示例：使用 SAS Token 下载 Blob

以下是一个简单示例：

```python
from azure.storage.blob import BlobServiceClient


def lambda_handler(event, context):

    account_name = "mystorageaccount"
    container_name = "reports"
    sas_token = "YOUR_SAS_TOKEN"

    account_url = (
        f"https://{account_name}.blob.core.windows.net"
    )

    blob_service_client = BlobServiceClient(
        account_url=account_url,
        credential=sas_token
    )

    blob_client = blob_service_client.get_blob_client(
        container=container_name,
        blob="test/test.csv"
    )

    download_stream = blob_client.download_blob()

    data = download_stream.readall()

    print(data.decode("utf-8"))

    return {
        "statusCode": 200
    }
```

生产环境中不应该把：

```text
SAS Token
```

直接写在代码中。

应该从：

```text
AWS Secrets Manager
```

读取。

---

# 18. 大文件需要特别注意

如果 Azure Blob 文件较大，例如：

```text
100 MB
500 MB
2 GB
```

不建议直接：

```python
data = download_stream.readall()
```

因为会把整个文件读取到 Lambda 内存中。

可能导致：

```text
Memory exhausted
```

或者：

```text
Lambda timeout
```

---

## 如果需要下载后保存到 S3

架构可能是：

```text
Azure Blob
    │
    │ 下载
    ▼
Lambda
    │
    │ 上传
    ▼
Amazon S3
```

需要根据文件大小设计流式处理，而不是全部读入内存。

---

## 如果文件特别大

例如：

```text
几十 GB
```

可能不适合使用 Lambda。

可以考虑：

```text
Azure Blob
     │
     ▼
AWS ECS / Fargate
```

具体取决于：

- 文件大小
- 文件数量
- 处理时间
- Lambda 超时时间
- 内存需求

---

# 19. 建议的测试顺序

创建 Lambda 后，不要立即实现全部业务逻辑。

建议按以下顺序测试。

## 第一步：DNS

测试是否能够解析：

```text
<storage-account>.blob.core.windows.net
```

---

## 第二步：HTTPS 网络连接

测试 Lambda 是否能够访问 Azure Blob Storage 的：

```text
HTTPS 443
```

---

## 第三步：Azure 认证

使用 SAS Token 或其他凭证测试：

```text
List Blob
```

或者读取一个测试文件。

---

## 第四步：下载小文件

先测试：

```text
test.txt
```

确认：

```text
网络正常
认证正常
权限正常
```

之后再处理正式文件。

---

# 20. 常见问题

## 1. Lambda Timeout

常见原因：

```text
Lambda 位于 Private Subnet
        ↓
没有 NAT Gateway
        ↓
无法访问 Internet
        ↓
Azure 连接 Timeout
```

---

## 2. Azure 返回 403 Forbidden

可能原因：

```text
SAS Token 权限不足
SAS Token 已过期
Azure Storage Firewall 拒绝访问
IP 白名单未配置
```

---

## 3. DNS 解析失败

可能原因：

```text
DNS 配置问题
Private Endpoint DNS 配置问题
```

---

## 4. 可以连接 Storage Account，但无法访问指定 Container

可能原因：

```text
SAS Token 只授权了其他 Container
```

或者：

```text
RBAC 权限不足
```

---

# 21. 推荐实施路线

## 第一步：先确认 Azure 信息

至少获取：

```text
① Storage Account Name

② Container Name

③ Blob 文件路径规则

④ 认证方式

⑤ Public Network Access 是否允许

⑥ 是否存在 IP 白名单

⑦ 是否使用 Private Endpoint
```

---

## 第二步：根据 Azure 网络配置决定 AWS 架构

### 公网开放

```text
Lambda
   ↓
Internet
   ↓
Azure Blob Storage
```

---

### Azure 要求固定 IP

```text
Lambda
   ↓
Private Subnet
   ↓
NAT Gateway
   ↓
Elastic IP
   ↓
Azure Blob Storage
```

然后让 Azure 管理员将：

```text
Elastic IP
```

加入白名单。

---

### Azure 只能通过 Private Endpoint 访问

需要进一步设计：

```text
AWS VPC
   ↕
VPN / Direct Connect / 其他私网连接
   ↕
Azure VNet
   ↓
Private Endpoint
   ↓
Azure Blob Storage
```

---

# 22. 推荐架构

如果需要生产环境长期运行，并且 Azure Storage 配置 IP 白名单，我推荐：

```text
Azure
│
├── Blob Storage
│
├── 指定 Container
│
├── SAS Token
│      └── 只提供 Read / List 权限
│
└── IP Firewall
       └── 允许 AWS NAT Gateway Elastic IP

                ▲
                │
         NAT Gateway
                ▲
                │
        AWS Lambda
```

AWS 侧：

```text
Lambda
├── Python
├── azure-storage-blob
├── AWS Secrets Manager
├── CloudWatch Logs
└── IAM Least Privilege
```

---

# 23. 最终建议

对于当前项目，建议不要立即创建正式 Lambda。

首先确认 Azure 侧：

```text
认证方式
+
网络访问方式
```

然后再决定 AWS 架构。

优先推荐：

## 简单 POC

```text
Public Network Access
+
SAS Token（Read / List）
+
Lambda
```

## 生产环境（需要固定出口 IP）

```text
Lambda
+
Private Subnet
+
NAT Gateway
+
Elastic IP
+
Azure Storage IP Whitelist
+
SAS Token
```

## 企业级长期方案

```text
AWS Lambda
+
Microsoft Entra ID / Service Principal
+
Azure RBAC
+
跨云网络访问控制
```

实际最终方案需要根据 Azure Storage 当前的认证方式和网络配置决定。
