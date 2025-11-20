# 0g 共学营第二次作业

本项目演示如何使用 0g-storage-client 生成、切分、上传和查询大文件。

## 参考资源

- **0g-storage-client**: https://github.com/0gfoundation/0g-storage-client
- **0g-storage-go-starter-kit**: https://github.com/0gfoundation/0g-storage-go-starter-kit
- **0G Storage SDK 文档**: https://docs.0g.ai/developer-hub/building-on-0g/storage/sdk

## 环境准备

### 1. 安装 0g-storage-client

请参考官方文档安装 0g-storage-client 工具。

### 2. 配置私钥（可选）

> **注意**：`.env` 文件仅在需要代码集成（如使用 Go SDK 开发应用）时才需要配置。如果只是使用 CLI 命令行工具，可以直接通过 `--key` 参数传递私钥，无需配置 `.env` 文件。

创建 `.env` 文件并配置私钥（注意：请勿泄露您的私钥）：

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```
PRIVATE_KEY=your_private_key_here
```

## 操作流程

### 步骤 1：生成 4GB 测试文件

使用 0g-storage-client 生成一个 4GB 的测试文件：

```bash
0g-storage-client gen --file test-4gb.dat --size 4294967296
```

### 步骤 2：切分文件

运行 Go 程序将 4GB 文件切分为 10 个 400MB 的文件：

```bash
go run main.go
```

执行后会在 `chunks/` 目录下生成以下文件：
- `test-4gb-part-01.dat` (400MB)
- `test-4gb-part-02.dat` (400MB)
- ...
- `test-4gb-part-10.dat` (400MB)

### 步骤 3：批量上传文件

使用 0g-storage-client 将切分后的文件批量上传到 0G Storage 网络。

**注意：请确保在项目根目录下运行命令，或使用绝对路径。**

```bash
# 上传单个文件
0g-storage-client upload \
  --url https://evmrpc-testnet.0g.ai \
  --key <YOUR_PRIVATE_KEY> \
  --indexer https://indexer-storage-testnet-turbo.0g.ai \
  --file chunks/test-4gb-part-01.dat

# 批量上传所有切分文件（Linux/Mac）
for i in $(seq -w 1 10); do
  0g-storage-client upload \
    --url https://evmrpc-testnet.0g.ai \
    --key <YOUR_PRIVATE_KEY> \
    --indexer https://indexer-storage-testnet-turbo.0g.ai \
    --file chunks/test-4gb-part-$i.dat
done

# 批量上传所有切分文件（Windows CMD）
for %i in (01 02 03 04 05 06 07 08 09 10) do 0g-storage-client upload --url https://evmrpc-testnet.0g.ai --key <YOUR_PRIVATE_KEY> --indexer https://indexer-storage-testnet-turbo.0g.ai --file chunks\test-4gb-part-%i.dat

# 批量上传所有切分文件（Windows PowerShell）
1..10 | ForEach-Object {
  $num = "{0:D2}" -f $_
  0g-storage-client upload `
    --url https://evmrpc-testnet.0g.ai `
    --key <YOUR_PRIVATE_KEY> `
    --indexer https://indexer-storage-testnet-turbo.0g.ai `
    --file "chunks\test-4gb-part-$num.dat"
}
```

上传成功后会返回每个文件的 `root_hash`，请记录下来以便后续查询。

### 步骤 4：查询和下载验证文件

#### 查询文件信息

通过 Indexer 的 HTTP API 查询文件信息：

```bash
# 通过 root hash 查询文件信息
curl "https://indexer-storage-testnet-turbo.0g.ai/file/info/<ROOT_HASH>"

# 示例：
curl "https://indexer-storage-testnet-turbo.0g.ai/file/info/<ROOT_HASH>"

# 批量查询多个文件
curl "https://indexer-storage-testnet-turbo.0g.ai/files/info?cid=<ROOT_HASH_1>&cid=<ROOT_HASH_2>"
```

#### 下载验证文件

使用 0g-storage-client 下载已上传的文件：

```bash
# 下载单个文件
0g-storage-client download \
  --indexer https://indexer-storage-testnet-turbo.0g.ai \
  --root <ROOT_HASH> \
  --file downloaded-part-01.dat

# 示例：
0g-storage-client download \
  --indexer https://indexer-storage-testnet-turbo.0g.ai \
  --root <ROOT_HASH> \
  --file downloaded-part-01.dat
```

也可以通过 HTTP 直接下载：

```bash
# 通过 root hash 下载
curl -O "https://indexer-storage-testnet-turbo.0g.ai/file?root=<ROOT_HASH>"

# 指定下载文件名
curl -o downloaded.dat "https://indexer-storage-testnet-turbo.0g.ai/file?root=<ROOT_HASH>&name=downloaded.dat"
```

下载完成后，可以对比原文件和下载文件的哈希值来验证完整性。

## 网络配置

- **EVM RPC**: `https://evmrpc-testnet.0g.ai`
- **Indexer RPC**: `https://indexer-storage-testnet-turbo.0g.ai`

## 踩坑历程

### 1. Go 版本过高导致编译失败

在编译 0g-storage-client 时，如果遇到以下错误：

```
link: github.com/fjl/memsize: invalid reference to runtime.stopTheWorld
```

这是因为 Go 版本过高导致的兼容性问题。需要使用以下命令进行编译：

```bash
go build -ldflags=-checklinkname=0
```

### 2. 其他操作

其他操作正常按照官方文档进行即可。

## 注意事项

1. **私钥安全**：请妥善保管您的私钥，切勿上传到公开仓库
2. **Gas 费用**：上传文件需要消耗 Gas，请确保账户有足够的测试币
3. **文件大小**：每个切分文件为 400MB，总计 4GB

## 项目结构

```
.
├── main.go          # 文件切分程序
├── .env             # 环境变量配置（私钥）
├── .env.example     # 环境变量示例
├── test-4gb.dat     # 生成的 4GB 测试文件
├── chunks/          # 切分后的文件目录
│   ├── test-4gb-part-01.dat
│   ├── test-4gb-part-02.dat
│   └── ...
└── README.md        # 本文档
```
