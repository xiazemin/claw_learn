[](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)

https://github.com/atomicstrata/llm-wiki-compiler

https://deepwiki.com/atomicmemory/llm-wiki-compiler/1.1-getting-started


npm install -g llm-wiki-compiler
export ANTHROPIC_API_KEY=sk-...

llmwiki ingest https://en.wikipedia.org/wiki/Andrej_Karpathy

llmwiki ingest .\READEME.md

* Ingesting: .\READEME.md
(node:34164) [DEP0040] DeprecationWarning: The `punycode` module is deprecated. Please use a userland alternative instead.
(Use `node --trace-deprecation ...` to show where the warning was created)
+ Saved reademe.md → sources\reademe.md
→ Next: llmwiki compile


> tree
文件夹 PATH 列表
卷序列号为 9E05-A3CD
D:.
└─sources

> ls .\sources\
目录: D:\code\backend\study\claw_learn\llmwiki\sources


Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----         2026/6/30     21:21            499 reademe.md


> llmwiki compile

llmwiki compile
───────────────────
(node:35524) [DEP0040] DeprecationWarning: The `punycode` module is deprecated. Please use a userland alternative instead.
(Use `node --trace-deprecation ...` to show where the warning was created)
+ reademe.md [new]
* Extracting: reademe.md
⚠ API call failed (attempt 1/4):


> llmwiki compile
! No sources found. Run `llmwiki ingest <url>` first.
(node:41016) [DEP0040] DeprecationWarning: The `punycode` module is deprecated. Please use a userland alternative instead.
(Use `node --trace-deprecation ...` to show where the warning was created)



llmwiki query "What terms did Andrej coin?"

npm update -g llm-wiki-compiler



https://github.com/nashsu/llm_wiki

