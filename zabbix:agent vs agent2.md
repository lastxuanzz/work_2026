思考了 26s

可以。这个问题其实很值得仔细拆开，因为 Zabbix Agent 和 Agent 2 的区别，不只是“2 代功能更多”。真正重要的是：

它们的底层架构不同、扩展方式不同、并发模型不同、平台支持不同，而且 Agent 2 虽然大多数场景可以作为 Agent 的替代品，但并不是 100% 无条件替换。

我检索了目前 Zabbix 官方文档、官方 Blog、Zabbix Forum、Reddit，并重点看了 7.0/7.4 以及正在开发的 8.0 文档。

> 先说明当前版本背景：截至 2026 年 9 月 8 日，Zabbix 官方下载页仍显示 8.0 LTS 为 beta2（2026-07-09），官方 roadmap 计划 8.0 LTS 于 2026 年 9 月发布。因此下面的正式功能对比主要以 Zabbix 7.4 官方文档为基准，同时参考 8.0 文档确认发展方向。




---

一、先说结论

如果只让我给一个非常直观的判断：

项目	Zabbix Agent	Zabbix Agent 2

定位	传统 Agent	新一代 Agent
语言	C	Go + 部分 C
普通 CPU/内存/磁盘/进程监控	✅	✅
Passive checks	✅	✅
Active checks	✅	✅
UserParameter	✅	✅
system.run[] 等常规能力	✅	✅
日志监控	✅	✅
并发能力	较传统	更强
Active check 并行执行	较弱	强
Plugin 架构	❌	✅
Docker 原生监控	❌	✅
MySQL 原生 Plugin	❌	✅
PostgreSQL 原生 Plugin	❌	✅
Redis 原生 Plugin	❌	✅
systemd 原生监控	❌	✅
自定义扩展	C Loadable Module	Go Plugin
第三方 Trap	❌	✅
Persistent Buffer	❌	✅
AIX	✅	❌
FreeBSD	✅	❌
OpenBSD	✅	❌
NetBSD	✅	❌
HP-UX	✅	❌
Solaris	✅	❌
Linux	✅	✅
Windows	✅	✅
macOS	✅	目前不属于 Agent 2 官方支持平台
兼容旧 Item key	—	基本兼容
适合新部署	可以	通常优先考虑 Agent 2
最大优势	稳定、轻量、平台广	扩展能力、并发、现代化架构
最大注意点	功能相对传统	平台和部分底层能力存在限制


这里最值得注意的是：

> “Agent 2 = Agent 1 的超集”这个说法，在常见监控 Item 的层面基本成立，但在整个软件能力层面并不严格成立。



例如官方明确写了：Agent 2 支持 Agent 的所有 item keys，但它并不支持 Agent 的所有配置参数和底层机制。


---

二、两者最大的区别，其实是“架构”

1. Agent：传统 C 架构

经典 Zabbix Agent 是 C 写的。

它的设计非常简单：

Zabbix Server / Proxy
        │
        │ TCP
        ▼
  Zabbix Agent
        │
        ├── CPU
        ├── Memory
        ├── Disk
        ├── Process
        ├── Network
        ├── Log
        ├── UserParameter
        └── ...

它最大的特点是：

轻量、简单、成熟、平台兼容性非常广。

官方目前仍然支持：

Linux

IBM AIX

FreeBSD

NetBSD

OpenBSD

HP-UX

macOS

Solaris

Windows


这一点其实非常重要。


---

三、Agent 2 到底改变了什么？

Agent 2 不是简单地把 Agent 1 重新编译了一遍。

它是以 Go 为核心重新设计的 Agent 架构，只有部分代码复用了原来的 C 代码。Zabbix 官方明确说 Agent 2 的设计目标包括：

减少 TCP connection 数量

提升 check concurrency

通过 plugin 更容易扩展

可以处理 long-running task

可以支持独立的数据采集和周期性上报

作为 Agent 的替代方案。


可以理解成：

Agent 1

        ┌─────────────────────┐
        │    Zabbix Agent     │
        │       C based       │
        ├─────────────────────┤
        │ CPU                 │
        │ Memory              │
        │ Disk                │
        │ Log                 │
        │ UserParameter       │
        │ etc...              │
        └─────────────────────┘


                  Agent 2

        ┌────────────────────────────┐
        │       Zabbix Agent 2       │
        │          Go based          │
        ├────────────────────────────┤
        │ Scheduler                  │
        │ Plugin Manager             │
        │ Task Queue                 │
        │ Connection Management      │
        ├──────────────┬─────────────┤
        │ CPU Plugin   │ MySQL Plugin│
        │ Log Plugin   │ Redis Plugin│
        │ Docker       │ PostgreSQL   │
        │ systemd      │ etc...       │
        └──────────────┴─────────────┘

这才是两者的本质区别。


---

四、功能方面：Agent 2 到底多了什么？

1. 最重要：Plugin

这是 Agent 2 最大的价值。

Agent 1 如果想扩展功能，主要通过：

UserParameter
       或
C Loadable Module

Agent 2 则可以：

Agent 2
   │
   ├── Built-in Plugin
   │
   └── Loadable Plugin

而且从 Zabbix 6.0 开始，plugin 可以作为 loadable plugin 独立加载，不必重新编译整个 Agent 2。

官方提供的 Agent 2 plugin 已经包括很多东西，例如：

Ceph

Docker

Ember+

MQTT

MSSQL

Memcached

Modbus

MongoDB

MySQL

NVIDIA GPU

Oracle

PostgreSQL

Redis

SMART

systemd

VMware 等


8.0 文档仍在继续沿着这个方向扩展。


---

五、数据库监控是 Agent 2 很典型的优势

这个差距很好理解。

传统 Agent 1：

Zabbix Agent
    │
    └── UserParameter
          │
          └── mysql command / script

Agent 2：

Zabbix Agent 2
       │
       └── MySQL Plugin
              │
              ├── Connections
              ├── Queries
              ├── InnoDB
              ├── Replication
              └── ...

Zabbix 官方 Blog 专门写过这个事情，并直接把 Agent 2 称为传统 Agent 的改进版本，强调了数据库监控场景下的优势。

现在的 Agent 2 官方 Item 中已经有例如：

mysql.ping
postgresql.ping
pgsql.wal.stat
redis.config
...

以及大量数据库专用 key。


---

六、Docker / Redis / PostgreSQL / systemd 也是类似情况

Agent 1 可以监控这些东西吗？

可以。

但是很多情况下你需要：

UserParameter
Script
External script
自定义程序

Agent 2 则可以直接通过官方 Plugin 体系完成。

官方对比表明确列出了 Agent 2 的原生监控能力，例如：

> Docker、Memcached、MySQL、PostgreSQL、Redis、systemd 等。



所以：

Agent 2 的核心优势不是“CPU 监控突然多了什么”，而是复杂目标的原生集成能力明显提高。


---

七、并发能力：这是第二个非常大的区别

这个非常重要。

Agent 1

传统 Agent 中：

Active checks
Server A
   │
   ├── Check 1
   ├── Check 2
   ├── Check 3
   ├── Check 4
   └── ...

官方说明：

> 对同一个 server 的 Active checks 是 sequential execution。



也就是说，传统架构对 Active Check 的并发能力存在明显限制。


---

Agent 2

Agent 2 使用：

Scheduler
    │
    ├── Plugin A
    │      ├── Task
    │      ├── Task
    │      └── Task
    │
    ├── Plugin B
    │      ├── Task
    │      └── Task
    │
    └── Plugin C

不同 Plugin 可以并发执行。

同一 Plugin 里面也可以并发执行多个 check，具体受到 plugin capacity 限制。

所以如果一个机器上有大量：

active items

Agent 2 的架构明显更适合。


---

八、Active Check 的调度能力也发生了变化

Agent 2 原先就支持：

Flexible interval
Scheduling

而传统 Agent 做不到这么灵活。

不过这里有一个非常容易踩坑的版本问题：

从 Zabbix 7.0 开始

Agent 1 和 Agent 2 的 Active check flexible/scheduling interval 已经统一。

也就是说：

> 到了 Zabbix 7.0，这已经不再是 Agent 2 独占能力。



官方 7.0 release notes 明确说明，这项能力从原来的 Agent 2 独有，变成 Agent 1 和 Agent 2 都支持。

所以如果你现在使用的是：

Zabbix 7.x

就不要再把：

> “支持 flexible active check”



当作 Agent 2 与 Agent 1 的主要区别。

这是老文章里面很容易出现的过时结论。


---

九、持久化 Buffer：Agent 2 明显更先进

这个是我认为实际生产环境非常值得注意的一项。

Agent 2 支持：

EnablePersistentBuffer=1

数据可以持久化到本地 SQLite。

例如：

Agent 2
   │
   ├── Server reachable
   │      ↓
   │    send data
   │
   └── Server unavailable
          ↓
      write persistent buffer
          ↓
      server recovery
          ↓
       send old data

官方文档明确提供：

EnablePersistentBuffer
PersistentBufferFile
PersistentBufferPeriod

这些都是 Agent 2 特有配置。

传统 Agent 没有这个机制。


---

十、第三方 Trap

这是 Agent 2 一个很少被普通教程提到、但其实很重要的区别。

官方 comparison：

功能	Agent	Agent 2

Third-party traps	❌	✅




这背后的设计原因，其实就是 Agent 2 的 Plugin 模型可以保持连接状态，并执行更复杂的长期运行任务。

Zabbix 官方在介绍 Agent 2 架构的时候就特别提到了：

> persistent connections、stateful monitoring、receiving SNMP traps 等能力。






---

十一、但是 Agent 2 也不是“全面碾压”

这里是非常重要的部分。

很多网上文章会写：

> Agent 2 = Agent 1 的升级版。



这句话作为入门理解没问题，但拿来做生产环境选型就不严谨。


---

十二、最大限制：操作系统支持范围

这是两者最大的硬差异之一。

Agent 1

官方支持：

Linux
IBM AIX
FreeBSD
NetBSD
OpenBSD
HP-UX
macOS
Solaris
Windows

Agent 2

官方支持范围主要是：

Linux
Windows

也就是说：

平台	Agent	Agent 2

Linux	✅	✅
Windows	✅	✅
AIX	✅	❌
HP-UX	✅	❌
Solaris	✅	❌
FreeBSD	✅	❌
OpenBSD	✅	❌
NetBSD	✅	❌
macOS	✅	❌


这是目前官方文档明确列出来的差异。

如果你公司里面有：

AIX
HP-UX
Solaris
老 BSD

那么 Agent 1 并没有因为 Agent 2 出现就失去意义。


---

十三、Agent 2 不支持 Agent 1 的所有配置参数

这个也是实际迁移的时候最容易出问题的地方。

官方直接列出了 Agent 2 不支持的一些 Agent 配置参数。

例如：

AllowRoot
User
ListenBacklog
LoadModule
LoadModulePath
StartAgents

等。

其中最值得注意的是：

Agent 1

User=zabbix
AllowRoot=0

Agent 本身有切换用户的机制。

Agent 2

这个职责交给：

systemd

所以 Agent 2 本身没有这个 daemonization / user switching 机制。

这就是为什么以前论坛里有人看到：

User
AllowRoot

在 Agent 2 中行为不同。

官方论坛开发者也专门解释过这一点。


---

十四、Loadable Module 不兼容

这是迁移老系统的时候必须检查的。

Agent 1：

C module
      ↓
LoadModule

Agent 2：

Go Plugin
      ↓
Plugins

这不是同一种机制。

所以如果你现有 Zabbix 6.0 二次开发环境里存在：

xxx.so
LoadModule=xxx.so

不能简单理解为：

> “换成 Agent 2 就自动可以。”



不行。

需要重新评估。

官方明确写：

LoadModule
LoadModulePath

**Agent 2 不支持。**

而 Agent 2 使用的是：

Go Plugin
Loadable Plugin
Plugin configuration




---

十五、UserParameter 呢？

这个倒不用担心。

很多人会误以为：

> Agent 2 既然是 Plugin architecture，是不是 UserParameter 就废了？



不是。

Agent 2 仍然支持：

UserParameter=my.key,/path/to/script.sh

官方配置文档明确列出了：

UserParameter
UserParameterDir
UnsafeUserParameters

等。

而且 Agent 2 还支持：

-R userparameter_reload

可以在运行过程中重新加载 UserParameter。

所以：

如果你的现有 Zabbix 环境大量依赖 UserParameter，不需要因为迁移 Agent 2 而全部推倒重做。


---

十六、日志监控存在一个很容易忽略的差异

官方 comparison 特别列出了：

Persistent files for log*[] metrics

Agent 1

Unix 上支持。

Agent 2

不支持这个旧的 persistent-file 机制。

另一方面，两者的：

Log data upload

行为也不完全一样。

官方明确指出：

Agent 1：

> 可以在读取日志过程中上传数据，从而释放 buffer。



Agent 2：

> 当 buffer 满的时候，log gathering 会停止。



因此 Agent 2 要特别注意：

BufferSize
MaxLinesPerSecond

之间的关系。

官方甚至明确要求 Agent 2 的：

BufferSize

应该至少达到：

MaxLinesPerSecond × 2



所以如果你的环境里：

大量日志
+
高频 log[]
+
高并发

这部分必须专门评估，而不是简单认为 Agent 2 一定更好。


---

十七、密码学 / Cipher 也有差异

官方 comparison：

项目	Agent	Agent 2

User-configurable ciphersuites	✅	❌




这属于比较底层的差异。

大多数普通 Zabbix 部署根本碰不到这个问题。

但如果你所在环境对：

TLS
Cipher
PSK
Certificate
Compliance

有严格要求，就需要单独检查。

Agent 2 使用的 OpenSSL 要求也不同，官方目前写的是：

Linux: OpenSSL 1.0.1+
Windows: OpenSSL 1.1.1+
PSK support required
LibreSSL 不支持




---

十八、性能到底谁更高？

这个问题网上很多人喜欢简单回答：

> Agent 2 性能更好。



这个回答其实不够严谨。

应该分开看。

CPU / Memory 采集

两者都很好。

你不会因为：

system.cpu.util
vm.memory.size
vfs.fs.size
proc.num

这些基本 Item 换成 Agent 2 后就突然出现数量级的性能提升。


---

大量 Active Checks

Agent 2 更有优势。

因为它：

scheduler
+
concurrency
+
plugin task management

更先进。

官方明确把：

> improved concurrency



作为 Agent 2 的设计目标。


---

大量数据库 / Docker / Redis 等

Agent 2 通常更合适。

因为可以利用：

plugin
persistent connections
state
scheduler

而不是频繁启动 shell / command。


---

大量简单 UserParameter

两者都可以。

这种场景不会因为 Agent 2 是 Go 就必然更快。


---

十九、TCP connection 数量也是 Agent 2 的设计目标

官方明确把：

> Reduce the number of TCP connections



列为 Agent 2 的设计目标。

原因也是 Plugin / scheduler / persistent connection 架构。

所以：

100个简单 Item

和：

大量复杂 Plugin Item

对两种 Agent 的差异不是一个维度。


---

二十、协议方面现在已经不是大问题了

这点如果你研究的是 Zabbix 6 → 7 → 8 的迁移，非常重要。

以前 Agent 1 / Agent 2 的协议存在一些差异。

但是：

Zabbix 7.0

Zabbix 官方已经把 Agent / Agent 2 protocol 统一。

官方 7.0 release notes：

> Zabbix agent and agent 2 protocols have been unified.



并通过：

variant

来区分：

variant = 1
Agent

和：

variant = 2
Agent 2



所以如果你目前考虑：

Zabbix 7.x / 8.x

不应该再根据早期 5.0 / 5.4 时代的一些老文章判断协议兼容问题。


---

二十一、版本兼容性

这方面官方给出的规则也很清楚。

以 Zabbix 7.4 为例：

Agent

不能比：

1.4

更老，也不能比 Server：

7.4

更新。

Agent 2

从：

4.4

开始的旧 Agent 2 都可以与 Zabbix 7.4 兼容。

同样不能比 Server：

7.4

更新。

所以：

Zabbix Server 7.4
       │
       ├── Agent 7.4
       ├── Agent 7.0
       ├── Agent2 7.4
       └── Agent2 6.x

并不是说 Agent / Agent2 必须和 Server 精确相同版本才能通信。

当然，从功能完整性和 Bug Fix 的角度，官方还是建议使用最新受支持版本。


---

二十二、Windows 上特别值得注意

你之前提过你的 Agent 环境里有 Windows Agent，所以这一点我特别给你标出来。

在 Windows 上：

Agent 1

传统：

zabbix_agentd.exe

Agent 2

zabbix_agent2.exe

而 Agent 2 在 Windows 上：

Windows Service

可以正常运行。

甚至现在 Agent 2 支持：

-m / --multiple-agents

运行多个 Agent 2 实例。

所以网上一些：

> “Agent 2 不能运行多个实例”



的老帖子，现在不能直接照搬。

Zabbix 官方当前文档已经明确支持 multiple agents。


---

二十三、Windows 上 Agent 2 曾经确实有过一些问题

这也是我为什么没有单纯看官方文档就告诉你：

> Agent 2 完胜。



我也查了论坛的实际问题。

例如有人遇到：

Windows Performance Counter

相关问题。论坛里有：

> Agent 2 无法加载某些 Windows Performance Counters



以及：

PDH_MORE_DATA

导致 Agent 2 启动失败的问题。

还有 Windows Agent 2 loadable plugin 的性能问题：

zabbix_agent2 CPU usage 17~20%

论坛中也有真实案例。

不过需要强调：

这些是具体版本 / Windows 环境 / plugin implementation 问题，不能据此得出“Agent 2 性能差”的总体结论。

更合理的结论是：

> Agent 2 的架构更复杂，因此在特殊 Plugin、Windows Performance Counter、第三方 Plugin 等场景，需要更关注具体版本和已知问题。




---

二十四、社区对于“到底应该用哪个”的看法

这一点其实很有意思。

Zabbix 官方 Forum

有一个非常经典的讨论：

Zabbix Agent vs Agent 2

里面用户直接问：

> Agent 2 看起来所有地方都更好，是不是未来会取代 Agent？



论坛管理员 / 高级用户的回答基本就是：

> Agent 2 能做 Agent 1 做的事情，并且更多。






---

另一个论坛讨论

2021 年有人询问：

> Agent 2 的 daemonization 为什么是 no？是不是不会后台运行？



Zabbix 开发者解释：

> 在 Unix 上，Agent 2 不负责 daemonization，由 systemd 负责。



也就是说这并不是“功能缺失”，而是：

职责发生了变化。




---

二十五、Reddit 的实际用户意见

我也专门看了 Reddit。

2025 年一个很典型的讨论：

“Noob question, zabbix agent and agent2”

里面多数用户的意见是：

> Agent 2 使用更新的代码基础，有更多功能。



并且有人实际管理：

> 1300+ hosts



仍然使用传统 Agent，因为传统 Agent 本身依然工作得很好。

还有用户指出：

> 如果数据库等特殊监控需求较多，优先 Agent 2。





这其实很符合现实：

并不是因为 Agent 1 不行，所以大家必须迁移 Agent 2。

而是：

> 对新项目，Agent 2 更有发展空间； 对旧项目，Agent 1 也完全可以继续稳定运行。




---

二十六、社区里一个非常重要的真实现象

论坛里还有一个很有价值的问题：

> Agent 和 Agent 2 能不能装在同一个机器？



答案：

可以。

但是要用不同端口 / 不同实例配置。

这个对于迁移特别有用。

例如你现在有：

Agent 1
Port 10050

你可以测试：

Agent 2
Port 10051

然后逐步把 Item 从：

Agent 1

切到：

Agent 2

而不是一次性全部替换。

不过生产环境是否这样部署，需要考虑：

CPU
Memory
Port
UserParameter
conflicts
service
templates


---

二十七、关于“Agent 2 会不会最终取代 Agent”

官方现在的文档措辞非常值得注意。

官方把 Agent 2 描述成：

> new generation of Zabbix agent



并明确：

> may be used in place of Zabbix agent



以及：

> Function as a replacement for Zabbix agent





但是：

截至目前，Zabbix 并没有宣布 Agent 1 已经被废弃。

相反：

Agent
Agent 2

目前依然同时维护。

因此现实中的定位更准确应该是：

Agent 1
   ↓
成熟、轻量、广平台兼容

Agent 2
   ↓
新架构、Plugin、并发、复杂监控


---

二十八、如果让我按照“实际生产环境”给它们定位

我会这么判断：

场景 A：普通 Linux Server

例如：

CPU
Memory
Disk
Network
Process
Log
UserParameter

Agent 1 和 Agent 2 都可以。

新的环境我会偏向：

> Agent 2




---

场景 B：Windows Server

例如：

CPU
Memory
Disk
Windows service
Event Log
Performance Counter
PowerShell
UserParameter

也是：

> 优先 Agent 2，但测试你实际使用的 Windows Items。



特别是大量 Performance Counter 的环境，不要盲目全量切换，最好先在几个代表性 Windows 机器上验证。


---

场景 C：MySQL / PostgreSQL / Redis / Docker

明显倾向：

> Agent 2



因为 Plugin 架构带来的优势非常明显。


---

场景 D：AIX / Solaris / HP-UX

直接：

> Agent 1



因为 Agent 2 本身的平台支持就是硬限制。


---

场景 E：你自己写了 C Loadable Module

例如：

LoadModule=/usr/lib/zabbix/xxx.so

那就：

> Agent 1



除非你准备把它重写成 Agent 2 Plugin。


---

场景 F：高量 Active Check

例如：

几百 / 几千 Active Items

Agent 2 更有优势。


---

场景 G：网络不稳定，经常断 Server

Agent 2 的：

Persistent Buffer

很有价值。


---

二十九、我认为最重要的一张表

如果你准备做实际的 Zabbix Agent 选型，可以直接看这张：

维度	Agent	Agent 2	我的判断

基本系统监控	✅	✅	基本没区别
UserParameter	✅	✅	基本没区别
Passive	✅	✅	基本没区别
Active	✅	✅	基本没区别
Active 并发	较弱	✅	Agent 2
Plugin	❌	✅	Agent 2
DB 原生监控	较弱	✅	Agent 2
Docker	❌	✅	Agent 2
Redis	❌	✅	Agent 2
systemd	❌	✅	Agent 2
第三方 Trap	❌	✅	Agent 2
Persistent Buffer	❌	✅	Agent 2
C Module	✅	❌	Agent 1
Go Plugin	❌	✅	Agent 2
AIX	✅	❌	Agent 1
Solaris	✅	❌	Agent 1
HP-UX	✅	❌	Agent 1
FreeBSD	✅	❌	Agent 1
macOS	✅	❌	Agent 1
Linux	✅	✅	都可以
Windows	✅	✅	Agent 2 倾向
复杂数据库监控	一般	很好	Agent 2
轻量传统监控	很好	很好	都可以
老系统兼容	强	相对弱	Agent 1
新项目	可以	更推荐	Agent 2



---

三十、我最终的判断

如果把 Agent 1 和 Agent 2 比喻一下：

Agent 1
=
成熟的 C 语言系统监控守护进程

Agent 2
=
重新设计的 Go 语言监控平台
+
Plugin Framework
+
Scheduler
+
Concurrency
+
Persistent Buffer

因此：

> Agent 2 真正的优势，不是“CPU/memory 采集能力比 Agent 1 高很多”，而是它已经从“一个系统监控程序”逐渐变成了“一个可扩展的数据采集框架”。



这才是你理解 Agent 2 最关键的一点。

另一方面：

> Agent 1 最大的价值也不是“老”，而是成熟、轻量以及非常广泛的平台兼容性。



所以不能简单说：

Agent 2 > Agent 1

更准确是：

新环境
      ↓
    Agent 2
      ↓
复杂监控 / DB / Docker / Plugin
      ↓
    Agent 2

特殊老平台 / 现有 C Module
      ↓
     Agent 1


---

三十一、我找到的资料 / URL

下面这些是我这次实际检索过、并认为最值得保存的资料。

① Zabbix 官方：Agent vs Agent 2 对比

这是最重要的一篇，基本可以作为你的基准资料。

[Zabbix 官方：Agent vs agent 2 comparison](https://www.zabbix.com/documentation/current/en/manual/appendix/agent_comparison?utm_source=chatgpt.com)

它直接列出了：

Programming language

Supported platforms

Process model

Metrics

Concurrency

Third-party traps

Persistent storage

Log behavior

User switching

Cipher suites


等差异。


---

② Zabbix 官方：Agent 2

[Zabbix 官方：Agent 2](https://www.zabbix.com/documentation/current/en/manual/concepts/agent2?utm_source=chatgpt.com)

主要看：

Agent 2 architecture

Passive / Active

concurrency

Windows

Linux

runtime control

configuration





---

③ Zabbix 官方：Agent 2 Plugin

[Zabbix 官方：Plugins](https://www.zabbix.com/documentation/current/en/manual/extensions/plugins?utm_source=chatgpt.com)

这是理解 Agent 2 为什么能够扩展的核心文档。


---

④ Zabbix 官方：Agent 2 独有 Item

[Zabbix 官方：Zabbix agent 2 item keys](https://www.zabbix.com/documentation/current/en/manual/config/items/itemtypes/zabbix_agent/zabbix_agent2?utm_source=chatgpt.com)

可以看到各种：

Docker
Ceph
PostgreSQL
Redis
SMART
systemd
...




---

⑤ Zabbix 官方：Agent 配置参数

[Zabbix 官方：Zabbix agent UNIX configuration](https://www.zabbix.com/documentation/current/en/manual/appendix/config/zabbix_agentd?utm_source=chatgpt.com)

用它和 Agent 2 的 configuration page 对照，非常适合研究：

User
AllowRoot
LoadModule
StartAgents

这些差异。


---

⑥ Zabbix 官方：Agent 2 configuration

[Zabbix 官方：Zabbix agent 2 UNIX configuration](https://www.zabbix.com/documentation/current/en/manual/appendix/config/zabbix_agent2?utm_source=chatgpt.com)

这里可以重点看：

EnablePersistentBuffer
UserParameter
Plugins.*
Plugin capacity
StatusPort




---

⑦ Zabbix 官方：7.0 Agent / Agent2 协议统一

[Zabbix 官方：What's new in Zabbix 7.0](https://www.zabbix.com/documentation/7.0/en/manual/introduction/whatsnew700?utm_source=chatgpt.com)

如果你正在研究 Zabbix 6 → 7 → 8，这篇特别重要，因为：

> Agent 1 / Agent 2 protocol 从 7.0 开始统一。






---

⑧ Zabbix 官方 Blog：Agent 2 的设计背景

[Zabbix Blog：Magic of New Zabbix Agent](https://blog.zabbix.com/magic-of-new-zabbix-agent/8460/?utm_source=chatgpt.com)

这是理解：

> 为什么 Zabbix 要重新搞 Agent 2



非常好的资料。


---

⑨ Zabbix 官方 Blog：Agent 2 Plugin 开发

[Zabbix Blog：Developing plugins for Zabbix Agent 2](https://blog.zabbix.com/developing-plugins-for-zabbix-agent-2/9682/?utm_source=chatgpt.com)

重点讲：

为什么 Agent 1 的扩展方式有限

为什么 Agent 2 用 Go

Plugin architecture

persistent connection

stateful monitoring





---

⑩ Zabbix 官方 Blog：数据库监控

[Zabbix Blog：Out-of-the-box database monitoring](https://blog.zabbix.com/out-of-the-box-database-monitoring/13957/?utm_source=chatgpt.com)

很适合看 Agent 2 在真实场景中为什么有优势。


---

三十二、社区 / Forum

Zabbix Forum：Agent vs Agent 2

[Zabbix Forum：Zabbix agent vs agent2](https://www.zabbix.com/forum/zabbix-help/422788-zabbix-agent-vs-agent2?utm_source=chatgpt.com)

这个帖子很适合看实际用户对：

daemonization
User
权限
兼容性

的疑问，以及 Zabbix 开发者的解释。

Zabbix Forum：Agent 和 Agent 2 是否可以同时运行

[Zabbix Forum：Zabbix Agent and Agent2](https://www.zabbix.com/forum/zabbix-help/499678-zabbix-agent-and-agent2?utm_source=chatgpt.com)

适合你做迁移验证时参考。

Zabbix Forum：Agent 2 Windows Performance Counter

[Zabbix Forum：Zabbix agent 2 Windows performance counter buffer error](https://www.zabbix.com/forum/zabbix-troubleshooting-and-problems/490251-zabbix-agent-2-windows-performance-counter-buffer-error?utm_source=chatgpt.com)

用于了解 Windows 下真实运行中的问题。

Zabbix Forum：Agent 2 Plugin performance

[Zabbix Forum：Zabbix Agent2 loadable plugin performance issue on Windows](https://www.zabbix.com/forum/zabbix-troubleshooting-and-problems/454200-zabbix-agent2-loadable-plugin-performance-issue-on-windows?utm_source=chatgpt.com)

适合了解第三方 Plugin / Windows 场景的实际问题。


---

三十三、Reddit

Zabbix Reddit：Agent vs Agent 2

[Reddit：Noob question, zabbix agent and agent2](https://www.reddit.com/r/zabbix/comments/1j08rj7/noob_question_zabbix_agent_and_agent2/?utm_source=chatgpt.com)

比较有价值的地方是：

这里不是官方观点，而是真实用户经验。

里面有人：

使用 Agent 2

使用 1300+ hosts 的 Agent 1

讨论数据库监控

讨论 Windows

讨论 zabbix-sender


所以很适合和官方文档交叉验证。


---

最后给你一个非常重要的建议

结合你之前一直在做的 Zabbix 6.0 二次开发 → 新版 Zabbix 迁移，我认为你现在不要只研究：

> “Agent 还是 Agent 2 哪个好？”



你更应该检查你现在的旧系统里到底用了哪些：

UserParameter
LoadModule
自定义 Item key
自定义脚本
log[]
logrt[]
system.run[]
Windows performance counters
zabbix_get
zabbix_sender
第三方 Agent module

因为 真正决定你能不能从 Agent 1 迁移到 Agent 2 的，不是“Agent 2 有没有这个功能”，而是你现有系统有没有使用 Agent 1 的特殊机制。

特别是你那个正在使用的 Zabbix 6.0 二次开发项目，如果 Agent 侧也有定制内容，我建议把 Agent / Agent 2 的迁移检查项单独列出来。这样能直接判断“哪些东西可以原样迁移、哪些需要改配置、哪些要改成 Plugin、哪些必须继续使用 Agent 1”。这会比单纯比较功能表更实用。
