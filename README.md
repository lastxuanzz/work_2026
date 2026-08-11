## 第零课：3 分钟搞懂指针

在看代码之前，先理解指针的 3 个核心操作：

### 1. 指针就是一个"门牌号"（内存地址）

```c
char c = 'A';        // 在内存的某个位置存了一个字符 'A'
char *p = &c;        // p 存的是 c 的"门牌号"（地址），不是 'A' 本身
```

```
内存示意图：
地址:  0x1000  0x1001  0x1002  ...
内容:  [ 'A' ] [  ?  ] [  ?  ]

c 在地址 0x1000，值是 'A'
p 在另一个地址，存的值是 0x1000（即 c 的地址）
```

### 2. `*p` 是"去这个门牌号看看里面有什么"（解引用）

```c
char c = 'A';
char *p = &c;    // p 指向 c
*p = 'B';        // 去 p 存的地址，把里面的值改成 'B'
// 现在 c 变成了 'B'
```

```
*p = 'B' 的意思：
p 里存的是 0x1000 → 去 0x1000 这个位置 → 把内容改成 'B'
```

### 3. `p++` 是"门牌号+1，去隔壁"

```c
char str[] = "ABC";
char *p = str;    // p 指向 str[0]，即 'A'

p++;              // p 现在指向 str[1]，即 'B'
p++;              // p 现在指向 str[2]，即 'C'
```

```
初始: p → [A][B][C][\0]     p 指向 A
p++:       p → [B][C][\0]   p 指向 B
p++:            p → [C][\0] p 指向 C
```

好，基础就这三条。现在开始走代码。

---

## 假数据

Agent 发给 Server 的 JSON 字符串（存在 `char *s` 里）：

```c
char *s = "{\"request\":\"proxy_config\",\"host\":\"server01\"}";
```

在内存中实际存储为：

```
s (指针) → 指向下面这块内存:
          ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐
字节:       { │ " │ r │ e │ q │ u │ e │ s │ t │ " │ : │ " │ p │ r │ o │ x │ y │ _ │ c │ o │ n │ f │ i │ g │ " │ , │ " │ h │ o │ s │ t │ " │ : │ " │ s │ e │ r │ v │ e │ r │ 0 │ 1 │ " │ } │ \0 │
          └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘
索引:    0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15   16   17   18   19   20   21   22   23   24   25   26   27   28   29   30   31   32   33   34   35   36   37   38   39   40   41   42   43   44
```

---

## 第一步：`zbx_json_open` 解析 JSON (trapper.c:1057)

```c
struct zbx_json_parse jp;
zbx_json_open(s, &jp);
```

这个函数做的事很简单：找到 `{` 和 `}` 的位置，记录下来。

```
解析后 jp 结构体:
  jp.start → 指向 s[0]  = '{'
  jp.end   → 指向 s[44] = '}'
```

这样后面所有操作都知道 JSON 的边界在哪里了。

---

## 第二步：调用入口 (trapper.c:1065)

```c
char value[2048] = "";  // 2048 个 \0，全部是空的
zbx_json_value_by_name(&jp, "request", value, 2048, NULL);
```

此时 `value`：

```
value → [\0][\0][\0]...[共2048个\0]
```

---

## 第三步：进入 `zbx_json_value_by_name` (json.c:1099)

```c
int zbx_json_value_by_name(const struct zbx_json_parse *jp, const char *name,
        char *string, size_t len, zbx_json_type_t *type)
{
    const char *p;   // 声明一个 char* 指针，还没赋值（垃圾值）

    // 第①小步：在 JSON 中找 "request" 这个 key
    if (NULL == (p = zbx_json_pair_by_name(jp, name)))
        return FAIL;
```

这句话做了两件事：
1. `p = zbx_json_pair_by_name(jp, "request")` — 调用函数，返回值赋给 `p`
2. `NULL == (...)` — 判断返回值是不是 NULL（没找到就失败）

---

## 第四步：进入 `zbx_json_pair_by_name` (json.c:1060)

这是最关键的一步！它要回答：**"request" 这个 key 在 JSON 的哪个位置？**

```c
const char *zbx_json_pair_by_name(const struct zbx_json_parse *jp, const char *name)
{
    char        buffer[2048];     // 临时缓冲区，存每一轮的 key 名
    const char  *p = NULL;       // 游标指针，初始为 NULL

    // 循环：每次取一个 key-value 对，比较 key 是不是 "request"
    while (NULL != (p = zbx_json_pair_next(jp, p, buffer, sizeof(buffer))))
        if (0 == strcmp(name, buffer))  // 比较：name("request") == buffer(当前key)?
            return p;                    // 匹配！返回指向 value 的指针

    return NULL;  // 没找到
}
```

### 核心循环逻辑：

```
p = NULL  (初始)
    ↓
第1次调用 zbx_json_pair_next(jp, NULL, buffer, 2048)
    → 返回指向 "request" 之后的值位置的指针
    → buffer 里被填入了 "request"
    → strcmp("request", "request") == 0 ✅ 匹配！
    → return p  ← 返回这个指针
```

---

## 第五步：进入 `zbx_json_pair_next` (json.c:1030)

这个函数负责：**从当前位置，跳到下一个 key-value 对，把 key 名拷到 buffer，返回 value 的指针。**

```c
const char *zbx_json_pair_next(const struct zbx_json_parse *jp, const char *p,
        char *name, size_t len)
{
    // ① 先跳到下一个"元素"的位置
    if (NULL == (p = zbx_json_next(jp, p)))
        return NULL;
```

### ① `zbx_json_next` 做了什么？

第一次调用时 `p = NULL`，所以它从 JSON 的开头开始：

```c
const char *zbx_json_next(const struct zbx_json_parse *jp, const char *p)
{
    if (NULL == p)
    {
        p = jp->start + 1;   // jp->start 指向 '{'，+1 后指向 '"'
        SKIP_WHITESPACE(p);  // 跳过可能的空格
        return p;            // 返回指针
    }
    // ...
}
```

```
JSON 内存:
  {  "  r  e  q  u  e  s  t  "  :  ...
  0  1  2  3  4  5  6  7  8  9 10  ...
  ↑
  jp->start (指向索引0的 '{')

  jp->start + 1 → 指向索引1的 '"'
  p 现在指向这里 ↓
  {  "  r  e  q  u  e  s  t  "  :  ...
  0  1  2  3  4  5  6  7  8  9 10  ...
     ↑
     p 指向这里
```

返回后，`p` 指向索引 1 的 `"`。

---

### ② 判断类型

```c
    if (ZBX_JSON_TYPE_STRING != __zbx_json_type(p))
        return NULL;
```

`__zbx_json_type` 函数（json.c:531）非常简单：

```c
static zbx_json_type_t __zbx_json_type(const char *p)
{
    if ('"' == *p)           // *p 就是"去 p 指向的位置看看是什么字符"
        return ZBX_JSON_TYPE_STRING;  // 是 " 开头 → 字符串类型
    if (('0' <= *p && *p <= '9') || '-' == *p)
        return ZBX_JSON_TYPE_INT;     // 是数字或 - 开头 → 整数类型
    if ('[' == *p) return ZBX_JSON_TYPE_ARRAY;
    if ('{' == *p) return ZBX_JSON_TYPE_OBJECT;
    if ('n' == p[0]...) return ZBX_JSON_TYPE_NULL;
    // ...
}
```

这里 `*p` 是 `"`，所以返回 `ZBX_JSON_TYPE_STRING`。✅ 通过。

> **新手知识点：`*p`**
> `p` 存的是门牌号（地址），`*p` 是"去这个门牌号看看里面有什么"。
> `p` 指向索引 1，索引 1 存的字符是 `"`，所以 `*p` 的值就是 `"`。

---

### ③ 拷贝 key 名到 buffer

```c
    if (NULL == (p = json_copy_string(p, name, len)))
        return NULL;
```

这里又调用了 `json_copy_string`！但这次是拷贝 **key 名**，不是拷贝 value。

`p` 指向 `"request"` 的开头引号，`name` 是 `buffer`：

```
拷贝前:
  p →  "  r  e  q  u  e  s  t  "  :  ...
  buffer → [ ? ][ ? ][ ? ][ ? ][ ? ][ ? ][ ? ][ ? ]...

拷贝后:
  p → (指向 "request" 后面的 ":")
  buffer → [ r ][ e ][ q ][ u ][ e ][ s ][ t ][ \0 ]...
           即 "request"
```

`json_copy_string` 的细节我等下专门讲，先继续。

---

### ④ 跳过冒号，定位到 value

```c
    SKIP_WHITESPACE(p);   // 跳过空格/制表符

    if (':' != *p++)       // 检查是不是冒号，然后把 p 后移一位
        return NULL;

    SKIP_WHITESPACE(p);   // 再跳过空格

    return p;              // 返回指向 value 的指针
```

```
JSON:
  ...  "  r  e  q  u  e  s  t  "  :  "  p  r  o  x  y  ...
                                    ↑
                                    p 现在指向这里（value 的开头引号）
```

---

## 第六步：回到 `zbx_json_pair_by_name`，比较 key

```c
    while (NULL != (p = zbx_json_pair_next(jp, p, buffer, sizeof(buffer))))
        if (0 == strcmp(name, buffer))  // name="request", buffer="request"
            return p;                    // 相等！返回 p
```

`strcmp("request", "request")` 返回 0，匹配成功！`return p`。

此时 `p` 指向 value 的开头引号：

```
JSON:
  ...  "  r  e  q  u  e  s  t  "  :  "  p  r  o  x  y  _  c  o  n  f  i  g  "  ...
                                    ↑
                                    p 返回这个位置（指向 "）
```

---

## 第七步：回到 `zbx_json_value_by_name`，解码 value

```c
    // p 现在指向 value 的开头引号
    if (NULL == zbx_json_decodevalue(p, string, len, type))
        return FAIL;
```

进入 `zbx_json_decodevalue`（json.c:958）：

```c
const char *zbx_json_decodevalue(const char *p, char *string, size_t size, ...)
{
    switch (type_local = __zbx_json_type(p))  // *p 是 '"' → STRING
    {
        case ZBX_JSON_TYPE_STRING:
            return json_copy_string(p, string, size);  // 走这里！
        // ...
    }
}
```

`__zbx_json_type(p)` 一看 `*p` 是 `"`，判断为 STRING 类型，调用 `json_copy_string`。

---

## 第八步：`json_copy_string` 逐字符拷贝 🔥🔥🔥

这是最核心的函数，我们逐行看（json.c:892）：

```c
const char *json_copy_string(const char *p, char *out, size_t size)
{
    char *start = out;   // 记住 out 的起始位置（用于计算已拷贝了多少）

    if (0 == size)       // 如果缓冲区大小是 0，直接失败
        return NULL;

    p++;                 // 跳过开头的引号 "
```

调用前：

```
  p →  "  p  r  o  x  y  _  c  o  n  f  i  g  "  ,  ...
       ①  ②  ③  ④  ⑤  ⑥  ⑦  ⑧  ⑨  ⑩  ⑪  ⑫  ⑬  ⑭

  out → \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0  ...  (value 缓冲区)
```

`p++` 后：

```
  p →  p  r  o  x  y  _  c  o  n  f  i  g  "  ,  ...
       ①  ②  ③  ④  ⑤  ⑥  ⑦  ⑧  ⑨  ⑩  ⑪  ⑫  ⑬

  out → \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0 \0  ...
```

---

### 进入循环：

```c
    while ('\0' != *p)    // 只要 *p 不是字符串结束符，就一直循环
    {
        switch (*p)       // 看 *p 是什么字符
        {
            case '\\':    // 转义字符分支（\n, \t, \\, \uXXXX等）
                // ... 这里跳过，我们的例子没有转义字符

            case '"':     // 遇到结束引号 → 拷贝完成！
                *out = '\0';   // 在 value 末尾写入 \0
                return ++p;    // 返回

            default:      // 普通字符 → 直接拷贝
                *out++ = *p++;  // 核心！拷贝一个字符，然后两个指针都后移
        }
    }
```

---

### 逐轮循环演示（`default` 分支）：

**关键语法：`*out++ = *p++`**

这个表达式拆开理解：
1. `*out = *p` — 把 `p` 指向的字符，拷贝到 `out` 指向的位置
2. `out++` — out 指针后移一位
3. `p++` — p 指针后移一位

```
【循环 1】
  p →  p  r  o  x  y  ...          *p = 'p'
  out → \0 \0 \0 \0 \0 \0 ...

  *out++ = *p++  执行后：
  p →  r  o  x  y  _  ...          (p 后移了)
  out → p \0 \0 \0 \0 \0 ...       (out 处写入了 'p'，然后后移)

【循环 2】
  p →  r  o  x  y  _  ...          *p = 'r'
  out → p \0 \0 \0 \0 \0 ...

  *out++ = *p++  执行后：
  p →  o  x  y  _  c  ...          (p 后移)
  out → p  r \0 \0 \0 \0 ...       (写入了 'r'，然后后移)

【循环 3】
  p →  o  x  y  _  c  ...          *p = 'o'
  out → p  r \0 \0 \0 \0 ...

  *out++ = *p++  执行后：
  out → p  r  o \0 \0 \0 ...       (写入了 'o')

【循环 4~11】 继续拷贝 x, y, _, c, o, n, f, i, g

【循环 12】
  p →  g  "  ,  ...               *p = 'g'
  out → p  r  o  x  y  _  c  o  n  f  i  g \0 ...

  *out++ = *p++  执行后：
  p →  "  ,  ...                  (p 后移)
  out → p  r  o  x  y  _  c  o  n  f  i  g \0 \0 ...

【循环 13】
  p →  "  ,  ...                  *p = '"'  ← 结束引号！
  进入 case '"' 分支:
    *out = '\0'   → 在 value 末尾写入 \0

  out → p  r  o  x  y  _  c  o  n  f  i  g \0 \0 ...
                                              ↑
                                          写入 \0 结束符

  return ++p;   → 函数返回！
```

---

## 第九步：一路返回到 trapper.c

```
json_copy_string 返回 → zbx_json_decodevalue 返回 → zbx_json_value_by_name 返回 SUCCEED
```

此时 `value` 的内容：

```
value → [p][r][o][x][y][_][c][o][n][f][i][g][\0][\0][\0]...[共2048字节]
         0  1  2  3  4  5  6  7  8  9 10 11  12  13  14
```

---

## 第十步：使用 value (trapper.c:1068)

```c
if (0 == strcmp(value, "proxy_config"))
{
    send_proxyconfig(sock, &jp);   // 值是 "proxy_config"，走这个分支
}
```

`value` 现在就是 `"proxy_config"`，可以正常使用了！

---

## 完整链路总结

```
s = "{\"request\":\"proxy_config\",\"host\":\"server01\"}"
                         │
        ┌────────────────┼────────────────┐
        │  ① zbx_json_open                │
        │     找到 { 和 } 的位置，存入 jp │
        └────────────────┼────────────────┘
                         │
        ┌────────────────┼────────────────┐
        │  ② zbx_json_pair_by_name        │
        │     └→ zbx_json_pair_next (循环)│
        │         └→ zbx_json_next        │  跳到下一个元素
        │         └→ __zbx_json_type      │  判断是不是字符串
        │         └→ json_copy_string     │  拷 key 到 buffer
        │         └→ 跳过 ':'             │
        │     └→ strcmp 比较 key 名       │
        │     找到 "request" → 返回 p     │
        └────────────────┼────────────────┘
                         │
        ┌────────────────┼────────────────┐
        │  ③ zbx_json_decodevalue         │
        │     └→ __zbx_json_type(*p)      │  *p=='"' → STRING
        │     └→ json_copy_string         │
        │         逐字符执行 *out++=*p++  │
        │         把 "proxy_config"       │
        │         拷入 value 缓冲区       │
        └────────────────┼────────────────┘
                         │
                         ▼
                  value = "proxy_config"
```

---

希望这次够详细了！如果还有哪个环节、哪个函数、哪个语法点不理解，告诉我，我可以再拆开来讲。
