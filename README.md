go-cqhttp leveldb v3 迁移工具 
===============================

## 安装

你可以下载 [已经编译好的二进制文件](https://github.com/RomiChan/gocq-leveldb-migrate/releases).

从源码安装:
```bash
$ go install github.com/RomiChan/gocq-leveldb-migrate@latest
```

## 使用方法

```bash
./gocq-leveldb-migrate -from xxx -to yyy
```
默认值：
 * from: `data/leveldb-v2`
 * to: `data/leveldb-v3`