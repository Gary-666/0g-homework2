package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/0gfoundation/0g-storage-client/common/blockchain"
	"github.com/0gfoundation/0g-storage-client/indexer"
	"github.com/0gfoundation/0g-storage-client/transfer"
	"github.com/joho/godotenv"
	"github.com/openweb3/web3go"
)

// Network configuration for 0G Testnet
const (
	EvmRPC             = "https://evmrpc-testnet.0g.ai"
	IndexerRPCStandard = "https://indexer-storage-testnet-turbo.0g.ai"
	IndexerRPCTurbo    = "https://indexer-storage-testnet-turbo.0g.ai"
	DefaultReplicas    = 1
)

// File split configuration
const (
	InputFile     = "test-4gb.dat"
	ChunkSize     = 400 * 1024 * 1024 // 400MB
	TotalChunks   = 10
	OutputPattern = "test-4gb-part-%02d.dat"
	ChunksDir     = "chunks"
	DownloadDir   = "downloads"
)

type StorageClient struct {
	web3Client    *web3go.Client
	indexerClient *indexer.Client
	ctx           context.Context
}

func NewStorageClient(ctx context.Context, privateKey string, useTurbo bool) (*StorageClient, error) {
	web3Client := blockchain.MustNewWeb3(EvmRPC, privateKey)

	indexerRPC := IndexerRPCStandard
	if useTurbo {
		indexerRPC = IndexerRPCTurbo
	}

	indexerClient, err := indexer.NewClient(indexerRPC)
	if err != nil {
		web3Client.Close()
		return nil, fmt.Errorf("failed to create indexer client: %v", err)
	}

	return &StorageClient{
		web3Client:    web3Client,
		indexerClient: indexerClient,
		ctx:           ctx,
	}, nil
}

func (c *StorageClient) Close() {
	if c.web3Client != nil {
		c.web3Client.Close()
	}
}

func (c *StorageClient) UploadFile(filePath string) (string, string, error) {
	nodes, err := c.indexerClient.SelectNodes(c.ctx, uint(DefaultReplicas), nil, "random", false)
	if err != nil {
		return "", "", fmt.Errorf("failed to select storage nodes: %v", err)
	}

	uploader, err := transfer.NewUploader(c.ctx, c.web3Client, nodes)
	if err != nil {
		return "", "", fmt.Errorf("failed to create uploader: %v", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
	defer cancel()

	txHash, rootHash, err := uploader.UploadFile(ctx, filePath)
	if err != nil {
		return "", "", fmt.Errorf("upload failed: %v", err)
	}

	return txHash.String(), rootHash.String(), nil
}

func (c *StorageClient) DownloadFile(rootHash, outputPath string) error {
	nodes, err := c.indexerClient.SelectNodes(c.ctx, uint(DefaultReplicas), nil, "random", false)
	if err != nil {
		return fmt.Errorf("failed to select storage nodes: %v", err)
	}

	downloader, err := transfer.NewDownloader(nodes.Trusted)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
	defer cancel()

	if err := downloader.Download(ctx, rootHash, outputPath, true); err != nil {
		return fmt.Errorf("download failed: %v", err)
	}

	return nil
}

// splitFile splits the input file into multiple chunks
func splitFile() ([]string, error) {
	// Open input file
	input, err := os.Open(InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %v", err)
	}
	defer input.Close()

	// Get file info
	info, err := input.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	fmt.Printf("Input file: %s\n", InputFile)
	fmt.Printf("File size: %d bytes (%.2f GB)\n", info.Size(), float64(info.Size())/(1024*1024*1024))
	fmt.Printf("Chunk size: %d bytes (%.2f MB)\n", ChunkSize, float64(ChunkSize)/(1024*1024))
	fmt.Printf("Total chunks: %d\n\n", TotalChunks)

	// Create output directory
	if err := os.MkdirAll(ChunksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Split file into chunks
	buffer := make([]byte, ChunkSize)
	var chunkPaths []string

	for i := 1; i <= TotalChunks; i++ {
		chunkName := fmt.Sprintf(OutputPattern, i)
		chunkPath := filepath.Join(ChunksDir, chunkName)

		n, err := io.ReadFull(input, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk %d: %v", i, err)
		}

		if n == 0 {
			fmt.Printf("Reached end of file at chunk %d\n", i)
			break
		}

		// Write chunk to file
		if err := os.WriteFile(chunkPath, buffer[:n], 0644); err != nil {
			return nil, fmt.Errorf("failed to write chunk %d: %v", i, err)
		}

		fmt.Printf("Created: %s (%d bytes)\n", chunkPath, n)
		chunkPaths = append(chunkPaths, chunkPath)
	}

	fmt.Println("\nFile split completed successfully!")
	return chunkPaths, nil
}

// uploadChunks uploads all chunk files and returns their root hashes
func uploadChunks(client *StorageClient, chunkPaths []string) (map[string]string, error) {
	results := make(map[string]string)

	fmt.Println("\n========== Starting Upload ==========")
	for i, chunkPath := range chunkPaths {
		fmt.Printf("\nUploading chunk %d/%d: %s\n", i+1, len(chunkPaths), chunkPath)

		txHash, rootHash, err := client.UploadFile(chunkPath)
		if err != nil {
			return results, fmt.Errorf("failed to upload %s: %v", chunkPath, err)
		}

		results[chunkPath] = rootHash
		fmt.Printf("Success! TxHash: %s\n", txHash)
		fmt.Printf("         RootHash: %s\n", rootHash)
	}

	fmt.Println("\n========== Upload Completed ==========")
	return results, nil
}

// downloadChunks downloads files using their root hashes
func downloadChunks(client *StorageClient, rootHashes map[string]string) error {
	// Create download directory
	if err := os.MkdirAll(DownloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %v", err)
	}

	fmt.Println("\n========== Starting Download ==========")
	i := 0
	for originalPath, rootHash := range rootHashes {
		i++
		// Extract filename from original path
		filename := filepath.Base(originalPath)
		downloadPath := filepath.Join(DownloadDir, filename)

		fmt.Printf("\nDownloading %d/%d: %s\n", i, len(rootHashes), filename)
		fmt.Printf("RootHash: %s\n", rootHash)

		if err := client.DownloadFile(rootHash, downloadPath); err != nil {
			return fmt.Errorf("failed to download %s: %v", filename, err)
		}

		fmt.Printf("Success! Saved to: %s\n", downloadPath)
	}

	fmt.Println("\n========== Download Completed ==========")
	return nil
}

func main() {
	// Set proxy
	os.Setenv("HTTP_PROXY", "http://localhost:7890")
	os.Setenv("HTTPS_PROXY", "http://localhost:7890")

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		log.Fatal("PRIVATE_KEY environment variable is required. Please add it to .env file")
	}

	// Check command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <command>")
		fmt.Println("Commands:")
		fmt.Println("  split    - Split the 4GB file into 10 chunks")
		fmt.Println("  upload   - Upload all chunks to 0G Storage")
		fmt.Println("  download - Download chunks from 0G Storage (requires root hashes)")
		fmt.Println("  all      - Split, upload, and download all in one")
		os.Exit(1)
	}

	command := os.Args[1]

	// Initialize storage client for upload/download operations
	var client *StorageClient
	var err error

	if command == "upload" || command == "download" || command == "all" {
		ctx := context.Background()
		client, err = NewStorageClient(ctx, privateKey, true)
		if err != nil {
			log.Fatalf("Failed to initialize storage client: %v", err)
		}
		defer client.Close()
	}

	switch command {
	case "split":
		_, err := splitFile()
		if err != nil {
			log.Fatalf("Split failed: %v", err)
		}

	case "upload":
		// Get list of chunk files
		var chunkPaths []string
		for i := 1; i <= TotalChunks; i++ {
			chunkPath := filepath.Join(ChunksDir, fmt.Sprintf(OutputPattern, i))
			if _, err := os.Stat(chunkPath); err == nil {
				chunkPaths = append(chunkPaths, chunkPath)
			}
		}

		if len(chunkPaths) == 0 {
			log.Fatal("No chunk files found. Please run 'split' command first.")
		}

		results, err := uploadChunks(client, chunkPaths)
		if err != nil {
			log.Fatalf("Upload failed: %v", err)
		}

		// Print summary
		fmt.Println("\n========== Upload Summary ==========")
		for path, hash := range results {
			fmt.Printf("%s -> %s\n", filepath.Base(path), hash)
		}

	case "download":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go download <root_hash> [output_filename]")
			fmt.Println("Example: go run main.go download 0x123... downloaded.dat")
			os.Exit(1)
		}

		rootHash := os.Args[2]
		outputFile := "downloaded.dat"
		if len(os.Args) >= 4 {
			outputFile = os.Args[3]
		}

		outputPath := filepath.Join(DownloadDir, outputFile)
		if err := os.MkdirAll(DownloadDir, 0755); err != nil {
			log.Fatalf("Failed to create download directory: %v", err)
		}

		fmt.Printf("Downloading file with root hash: %s\n", rootHash)
		if err := client.DownloadFile(rootHash, outputPath); err != nil {
			log.Fatalf("Download failed: %v", err)
		}
		fmt.Printf("Success! File saved to: %s\n", outputPath)

	case "all":
		// Step 1: Split file
		fmt.Println("Step 1: Splitting file...")
		chunkPaths, err := splitFile()
		if err != nil {
			log.Fatalf("Split failed: %v", err)
		}

		// Step 2: Upload chunks
		fmt.Println("\nStep 2: Uploading chunks...")
		results, err := uploadChunks(client, chunkPaths)
		if err != nil {
			log.Fatalf("Upload failed: %v", err)
		}

		// Step 3: Download chunks for verification
		fmt.Println("\nStep 3: Downloading chunks for verification...")
		if err := downloadChunks(client, results); err != nil {
			log.Fatalf("Download failed: %v", err)
		}

		// Print final summary
		fmt.Println("\n========== Final Summary ==========")
		fmt.Println("Root Hashes:")
		for path, hash := range results {
			fmt.Printf("  %s -> %s\n", filepath.Base(path), hash)
		}
		fmt.Println("\nAll operations completed successfully!")

	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
