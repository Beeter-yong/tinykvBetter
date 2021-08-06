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
  - 当前调用函数类型是 `MessageType_MsgAppend`，这是对当前 Leader 广播请求求追加日志的消息，则 Follower 和 Candidate 都需要处理，尤其 Candidate 此时转为 Follower。
4. `TestLeaderBcastBeat2AA` 测试 Leader 的广播能力，这就设计 Leader 管理其他 Follower 的日志序号能力，我们需要把日志部分完善。
  - 日志在 `raft/log.go` 中，对于日志中重要的几个节点：
    - first：为什么要有 first 的记录呢，因为 first 虽说位置在第一位，但其日志编号不一定是第一个，因为前面可能还有快照。**于是我们要在 Log struct 中创建一个字段标记第一个日志编号是多少**
    - applied：是已经应用的日志，不可改变，待称为快照
    - committed：是 leader 已经经过半数节点同意的日志
    - stabled：是已经接收的日志，但还没有决定是否采用，有可能会被覆盖
  - 那么需要在 `becomeLeader()` 处补充 leader 对日志管理的记录，并且 leader 要广播一个空消息给其他 Follower 来宣布权威，所以记得将这条日志即时加上去
5. `testNonleaderStartElection()` 比较简单，只需要考虑 tick 超时后处理相应的消息，把自己变成 candidate 和广播选举请求
  - 【注】这里的消息 RPC 都发到自己的 `msgs []pb.Message` 中即可，后续有类 pd 程序将其提取和转发相应的节点
6. `TestLeaderElectionInOneRoundRPC2AA()` 主要测试 candidate 接收其他节点对选举的响应设计，进而决定是称为 leader 还是转成 follower
7. `TestFollowerVote2AA()` 测试节点接收请求投票 RPC 后是否对其投票
  - 根据论文考虑 Term、是否已经投过票以及对方的日志比自己更多更新
8. 对于超时的测试，只要在 tick 考虑到了超时归零以及随机 Timeout 就通过测试

#### 问题
1. 在测试 `TestLeaderCycle2AA()`  不通过：后发现其他测试都没有对接收日志作测试，导致这一步 follower 对广播的日志没有处理使得 raftlog 的 term 和 index 改变不正确，就导致第三轮 candidate 不能得到一半以上的投票而不能称为 leader。
   1. 解决：只要对 leader 的广播日志追加到自己的日志后面即可解决问题，等下一步日志复制时这一步会很重要，会再细化。

### Code - Log replication
raft 另一重要部分。这其中有几个重要点：1）client 都是将请求发给 Leader 的，由 Leader 广播日志；2）Leader 决定一个日志是否提交（提交条件是**本任期内**过半节点响应 RPC）；3）Leader 发送消息携带自己已经提交的索引号，Follower 根据此决定自身的提交索引位置

1. client 的提议请求消息类型是 `pb.MessageType_MsgPropose`，则 Leader 自己追加日志并广播出去。
2. 记得完成 Follower 追加了 Leader 广播的日志后会给出响应，需要对此处理，重点是 Leader 需要决定自己的 commit index 是否需要更新
3. `TestFollowerCommitEntry2AB()` 测试考察 follower 根据 leader 发来的消息中的 committed 来更新自己的 committed 位置。这里就需要进一步补充 project2aa 部分最后待完善的 follower 日志追加函数。即更新 committed，但还有如果日志不合适需要拒绝响应没有完善
   ```
   if m.Commit > r.RaftLog.committed {
		r.RaftLog.committed = min(m.Commit, m.Index+uint64(len(m.Entries)))
	}
   ```
4. `TestFollowerCheckMessageType_MsgAppend2AB()` 同上个测试都是对 folower 接收日志功能测试，这部分完善 leader 发来的日志起始位置与自己不匹配而拒绝
5. `TestLeaderElectionOverwriteNewerLogs2AB()` 这个测试考虑情况复杂，涉及到 Raft 论文 5.4.2 Figure8 的情况
   - 【注】：`entsWithConfig()` 和 `votedWithConfig()` 都是在创建 Raft 时设定了一定的配置条件，前者对日志进行配置，后者对状态进行了设置，尤其后者，有 `storage.SetHardState(pb.HardState{Vote: vote, Term: term})`。所以需要在 `NewRaft()` 处考虑初始化时得到这些信息
   - 如下代码模拟了 figure8 的 a、b 两种情形，而后考虑 c 图情况
    ```go
    n := newNetworkWithConfig(cfg,
		entsWithConfig(cfg, 1),     // Node 1: Won first election
		entsWithConfig(cfg, 1),     // Node 2: Got logs from node 1
		entsWithConfig(cfg, 2),     // Node 3: Won second election
		votedWithConfig(cfg, 3, 2), // Node 4: Voted but didn't get logs
		votedWithConfig(cfg, 3, 2)) // Node 5: Voted but didn't get logs
    ```

#### 问题
1. 虽然测试了新 leader 覆盖之前任期的日志，但仍没有考虑 5.4.2 的 d、e 情况
2. `raft_test.go` 中很多测试没研究就可以通过 
