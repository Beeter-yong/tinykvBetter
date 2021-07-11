# TinyKv Project1
目的是实现一个 KV 存储系统。该 project 只是针对单机实现，不涉及通信、不涉及分布式系统。其实现是基于已有的 badger 数据库作为底层存储，并且 tinykv 已经为 badger 实现了操作 API，我们只是在这些 API 基础上再封装成上层可操作的 Put/Delete/Get/Scan 函数。

编码位置：`kv/storage/standalone_storage/standalone_storage.go` 和 `kv/server/server.go`

测试函数位置：`kv/server/server_test.go`

## 方法
1. 找到测试函数位置，从测试函数 debug 运行代码反推所要 code 函数功能
2. 对于结构体，更是需要从测试函数中获取 struct 需要什么字段
3. 需要先对 `kv/util/engine_util` 文件夹里的文件有所了解，这里给出了怎么去操作 badger

## Code
1. 确定结构体。我们从 `kv/util/engine_util/doc.go` 知道，要想对 storage 操作需要一个 engine，而 `kv/util/engine_util/engines.go` 给出它提供了 KVDB 和 RaftDB，以及它们的存储位置
2. 所以，storage 结构体最重要的是有一个 engine 来驱动；那么 初始化 storage 功能就是创建 engine
3. 对于 start 函数，貌似没有给出明确作用，对于 stop 函数就是关闭 engine，有对应 API（Close、Destroy） 可调用
4. 下一步是对 serve 中 Put/Delete/Get/Scan 进行封装，这是给上层应用调用的接口，它们要操作具体 Data 需要通过 storage 中的 Reader 和 Write 函数来操作，所以需要先对 Reader 和 Write 进行 code
5. 具体到 Reader 中，应该容易知道 Get 和 Scan 功能需要涉及 Reader。而 Reader 给定返回类型 `storage.StorageReader` 包含 GetCF 和 IterCF，应该对应的是 Get 和 Scan 操作。进一步发现 `storage.StorageReader` 没有对其实现，表明有我们对其进行具体实现。
   1. 定义 struct；初始化 struct；定义 GetCF、IterCF、Close API
    ![](https://cdn.jsdelivr.net/gh/Beeter-yong/pictures/imgTwo/20210711162014.png)
   2. 而后可以看出对 badger 的具体操作在 `kv/util/engine_util/util.go` 中，从名字易得 GetCF 应该对应 GetCF 和 GetCFFromTxn，其差别是有无 Txn，根据 `doc/project1-StandaloneKV.md` Hints 给出使用带有 Txn 的函数。那么我们传入的参数就多了一个，并且无论 Get 还是 Scan 都有这个参数，我们把它当作结构体字段设计。
   3. 对于 Scan 函数，我们发现其 Data 操作封装在 `kv/util/engine_util/cf_iterator.go`, IterCF 返回类型是 DBIterator，刚好由 BadgerIterator 继承。
   4. Scan 函数查找需传入起始 key，以及查找个数 limit，我们查找时需要先找到起始 key 位置，然后遍历 limit 次数即可
6. 具体到 Write 中，易得 Put 和 Delete 功能涉及 Write。我们看到 Write 传参有一个 Modify，表明对于 write 操作我们需要传入类型（Put or Delete）。
   1. 需要注意，此处 Put 和 Delter 请求都封装成 Modify 类型才能被 Write 识别，在 Write 中就需要再解析出是 Put 还是 Delete

## 测试
make project1