参考D:\code\backend\notebooklm\goclaw\.codegraph D:\code\backend\notebooklm\goclaw\cmd\gateway.go 和相关代码，详细分析下 runGateway 这个函数执行了哪些步骤，依赖了哪些外部组件，启动了哪些协程，channel之间信息是如何流动的，方便用户快速理解和使用goclaw  结果放在 D:\code\backend\notebooklm\goclaw

补充下 沙箱启动的详细流程，包括代码调用的完整路径，消息驱动调用完整路径，后端对应怎么和docker通信的，执行了哪些docker命令，不同用户，不同上下文如何隔离的，现场怎么保存和恢复的，沙箱内部执行的结果怎么传递给llm上下文的，上下文如何存储的

分析下提示词和上下文管理的完整流程，分层逻辑，组织后发起llm请求的完整结构，rag过程是怎么实现的，使用了pg向量化的完整流程使用了哪些语句，结合完整调用栈进行详细分析

详细分析下skills的注册与管理，创建，移除，还有llm发现使用的详细流程，skill存储在哪里，具体什么形式存储的，可以被修改吗？使用的详细流程，怎么控制的权限问题

详细分析下依赖的存储有哪些，分别起什么作用，有哪些表，key schema 整理下总结出来



参考D:\code\backend\notebooklm\goclaw\.codegraph D:\code\backend\notebooklm\goclaw\cmd\gateway.go 和相关代码，分析下go.mod 文件，引用了哪些包，这些包是干嘛的，goclaw使用了他的哪些功能，方便用户快速理解和使用goclaw  结果放在 D:\code\backend\notebooklm\goclaw
