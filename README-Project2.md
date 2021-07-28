# TinyKv project2
该 project 中分为三个部分：
1. PartA：实现 Raft 算法
2. PartB：基于 Raft 建立 KV server
3. PartC：添加日志垃圾回收和快照

该 Project 很明显借鉴了 etcd 的源码，当然 TiDB 本身涉及就是借鉴了 etcd。所以实现 Project2 最有力的 code 参考就是 [etcd](https://github.com/etcd-io/etcd/tree/main/raft) 中 raft 实现。
## PartA
TinyKv 的 Raft 实现几乎按照了 [Raft](https://willzhuang.github.io/2018/03/04/Raft%E8%AE%BA%E6%96%87%E7%BF%BB%E8%AF%91/) 论文自上而下一步步的实现，所以将论文读懂是很有必要的

编码位置：`raft/raft.go` 

测试位置：`raft/raft_paper_test.go`（主要是对论文相关功能测试）、`raft/raft_test.go`（）

在 PratA 中又分为三个功能点实现：

1.  Leader 选举，涉及论文中的心跳、投票、 状态转换等
2.  日志复制，即通过 Leader 将 client 的请求广播给其他 servers
3.  Raw node 接口，即封装 Raft 基本操作以及提供其他功能接口给上层应用调用

### 序言
1. 这里使用的是逻辑时钟 tick 代替物理时钟，在 Tick() 函数中每调用一次即时间增加一个值。
2. TinyKv 在 Raft 涉及上最大区别是将 Heartbeat 和 AppendEntries 分成两个消息处理
3. 了解消息类型很重要，tinyKv 在 Raft 通信这使用了很多消息进行通信，了解 `MessageTypes` 所对应类型作用是什么，进而为每一个 Type 功能进行相应涉及

### Code - Leader 选举
1.首先根据第一个测试 `TestFollowerUpdateTermFromMessage2AA()` 进行调试，我们找到所对应的第一测试功能位置
  - ![](https://cdn.jsdelivr.net/gh/Beeter-yong/pictures/imgTwo/20210717202348.png)
  - 根据注释，我们知道测试功能是 Term 变更，即根据论文 5.1 部分，无论什么状态的 server 接收的消息带有比自己大的 Term，则自身 Term 变更为较大 Term，同时将自己状态改为 Follwer。
  - 不过因为刚开始，我们需要先注意到第一个语句，也就是 `newTestRaft`，即先实例化一个 Raft，其传入参数包括当前 Raft 集群都有谁（1，2，3），心跳和选举时间，以及 Raft 存储引擎。
  - 这个 Raft 存储引擎区别于 KV 存储引擎，Raft 存储引擎主要存储的是 Raft 当前状态和日志信息，KV 存储的是数据
  - 接下来我们的重点就应该在 newRaft 上，也就是对 `raft/raft.go` code
2. 对 tinyKv 的 raft code，就可以对比 [etcd 的 raft](https://github.com/etcd-io/etcd/blob/main/raft/raft.go#L318:6) 进行考虑。
  - 这其中就涉及到 raft struct 的 涉及，还好 tinykv 已经给了我们设定了很多字段，我们可以先把每一个字段含义理解清楚，这里字段已经有一个 electionTimeout 表示选举超时时间，但我们知道 Raft 为了减少不同 server 同时开始竞争选举导致长时间不能选出 leader，而选用随机选举时间减少这种情况发生。所以我们添加一个 randomizedElectionTimeout 表示当前 raft 随机选举超时时间。
  - 接下来就是考虑 newRaft，我们根据参数 Config 实例化一个 raft 返回。需要注意新创建的 raft 一定是 follower，且 term = 0，以及初始化选举时间间隔
  - 这里我们也应该考虑 raft 宕机后重启的可能，以及初始化日志等情况，但在 leader 选举阶段可以先不考虑
3. 处理 become 状态转变函数，要注意 send() 函数。它是 Raft 运转的核心枢纽，基本分为：接收消息 -> 根据当前 State 识别对应的消息 -> 调用处理函数 -> 返回消息
  - 当前调用函数类型是 `MessageType_MsgAppend`，这是对当前 Raft 请求追加日志的消息
