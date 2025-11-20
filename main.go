package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// File split configuration
const (
	InputFile     = "test-4gb.dat"
	ChunkSize     = 400 * 1024 * 1024 // 400MB
	TotalChunks   = 10
	OutputPattern = "test-4gb-part-%02d.dat"
)

func main() {
	// Open input file
	input, err := os.Open(InputFile)
	if err != nil {
		log.Fatalf("Failed to open input file: %v", err)
	}
	defer input.Close()

	// Get file info
	info, err := input.Stat()
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}

	fmt.Printf("Input file: %s\n", InputFile)
	fmt.Printf("File size: %d bytes (%.2f GB)\n", info.Size(), float64(info.Size())/(1024*1024*1024))
	fmt.Printf("Chunk size: %d bytes (%.2f MB)\n", ChunkSize, float64(ChunkSize)/(1024*1024))
	fmt.Printf("Total chunks: %d\n\n", TotalChunks)

	// Create output directory
	outputDir := "chunks"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Split file into chunks
	buffer := make([]byte, ChunkSize)

	for i := 1; i <= TotalChunks; i++ {
		chunkName := fmt.Sprintf(OutputPattern, i)
		chunkPath := filepath.Join(outputDir, chunkName)

		n, err := io.ReadFull(input, buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Fatalf("Failed to read chunk %d: %v", i, err)
		}

		if n == 0 {
			fmt.Printf("Reached end of file at chunk %d\n", i)
			break
		}

		// Write chunk to file
		if err := os.WriteFile(chunkPath, buffer[:n], 0644); err != nil {
			log.Fatalf("Failed to write chunk %d: %v", i, err)
		}

		fmt.Printf("Created: %s (%d bytes)\n", chunkPath, n)
	}

	fmt.Println("\nFile split completed successfully!")
}
