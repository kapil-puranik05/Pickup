package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

const (
	bufferSize int = 64 * 1024 * 1024
)

type UploadRequest struct {
	Key       string `json:"key"`
	Size      uint64 `json:"size"`
	ChunkSize int64  `json:"chunkSize"`
}

type UploadResponse struct {
	ObjectId string   `json:"objectId"`
	Chains   []*Chain `json:"chains"`
}

type RetrievalInitializationRequest struct {
	Key string `json:"key"`
}

type RetrievalInitializationResponse struct {
	ObjectId       string   `json:"objectId"`
	Chains         []*Chain `json:"chains"`
	NumberOfChunks uint64   `json:"numChunks"`
}

type Chain struct {
	HeadAddress   string `json:"headAddress"`
	TailAddress   string `json:"tailAddress"`
	MasterAddress string `json:"masterAddress"`
	ChainId       string `json:"chainId"`
	Epoch         uint64 `json:"epoch"`
}

type Chunk struct {
	ID   uint64 `json:"id"`
	Data []byte `json:"data"`
}

type ChunkUploadRequest struct {
	Epoch          uint64 `json:"epoch"`
	SequenceNumber uint64 `json:"sequenceNumber"`
	ObjectId       string `json:"objectId"`
	Data           []byte `json:"data"`
	ChunkId        uint64 `json:"chunkId"`
}

type ChunkUploadResponse struct {
	IsWritten bool `json:"isWritten"`
}

type ChunkRetrievalRequest struct {
	ObjectId string `json:"objectId"`
}

type Config struct {
	Epoch uint64 `json:"epoch"`
}

type UploadCompleteNotification struct {
	ObjectId string `json:"objectId"`
}

type ChunkIndex struct {
	ID   uint64
	path string
}

func ReceiveChunks(req *RetrievalInitializationResponse) ([]*ChunkIndex, string, error) {
	baseDir := filepath.Join(req.ObjectId, "dump")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, "", fmt.Errorf("error occurred while creating temporary storage for chunks")
	}
	var (
		chunks []*ChunkIndex
		wg     sync.WaitGroup
		mu     sync.Mutex
		errCh  = make(chan error, len(req.Chains))
	)
	for _, chain := range req.Chains {
		wg.Add(1)
		go func(chain *Chain) {
			defer wg.Done()
			request := &ChunkRetrievalRequest{
				ObjectId: req.ObjectId,
			}
			body, err := json.Marshal(request)
			if err != nil {
				errCh <- fmt.Errorf("failed to marshal chunk retrieval request: %v", err)
				return
			}
			url := fmt.Sprintf("http://%s/read", chain.TailAddress)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				errCh <- fmt.Errorf("failed to send retrieval request: %v", err)
				return
			}
			defer resp.Body.Close()
			decoder := json.NewDecoder(resp.Body)
			for {
				var chunk Chunk
				err := decoder.Decode(&chunk)
				if err == io.EOF {
					break
				}
				if err != nil {
					errCh <- fmt.Errorf("Error occurred while receiving chunk %d: %v", chunk.ID, err)
					return
				}
				path := filepath.Join(baseDir, strconv.FormatUint(chunk.ID, 10))
				if err := os.WriteFile(path, chunk.Data, 0644); err != nil {
					errCh <- fmt.Errorf("Error occurred while writing chunk %d: %v", chunk.ID, err)
					return
				}
				mu.Lock()
				chunks = append(chunks, &ChunkIndex{
					ID:   chunk.ID,
					path: path,
				})
				mu.Unlock()
			}
		}(chain)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, "", err
		}
	}
	if len(chunks) != int(req.NumberOfChunks) {
		return nil, "", fmt.Errorf("Completeness Check Failed")
	}
	sort.Slice(chunks, func(i int, j int) bool {
		return chunks[i].ID < chunks[j].ID
	})
	return chunks, "", nil
}

func AssembleFile(chunks []*ChunkIndex, key string, baseDir string) error {
	outputPath := filepath.Join(key)
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("Failed to create output file: %v", err)
	}
	defer out.Close()
	for _, chunk := range chunks {
		in, err := os.Open(chunk.path)
		if err != nil {
			return fmt.Errorf("Failed to open chunk %d: %v", chunk.ID, err)
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			return fmt.Errorf("Failed to append chunk %d: %v", chunk.ID, err)
		}
		in.Close()
		if err := os.Remove(chunk.path); err != nil {
			return fmt.Errorf("Failed to delete temporary chunk %d: %v", chunk.ID, err)
		}
	}
	if err := os.Remove(baseDir); err != nil {
		return fmt.Errorf("Failed to remove temporary directory: %v", err)
	}
	return nil
}

func ProcessFileInChunks(filename string, bufferSize int, processor func(Chunk) error) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open the file: %v", err)
	}
	defer file.Close()
	buffer := make([]byte, bufferSize)
	var chunkId uint64 = 0
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])
			chunk := Chunk{
				ID:   chunkId,
				Data: data,
			}
			if err := processor(chunk); err != nil {
				return err
			}
			chunkId++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read the file: %v", err)
		}
	}
	return nil
}

func GetFileSize(filename string) (uint64, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return uint64(info.Size()), nil
}

func UploadFile(filename string) error {
	size, err := GetFileSize(filename)
	if err != nil {
		return fmt.Errorf("Failed to fetch file size: %v", err)
	}
	request := &UploadRequest{
		Key:       filename,
		Size:      size,
		ChunkSize: (64 * 1024 * 1024),
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("Failed to marshal metadata: %v", err)
	}
	resp, err := http.Post("http://localhost:8000/upload", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Server returned: %s", resp.Status)
	}
	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return fmt.Errorf("Failed to decode response: %v", err)
	}
	log.Printf("Chains received: %d", len(uploadResp.Chains))
	epochs := make([]uint64, 0)
	for _, chain := range uploadResp.Chains {
		configUrl := fmt.Sprintf("http://%s/layout", chain.MasterAddress)
		var epochResponse Config
		resp, er := http.Get(configUrl)
		if er != nil {
			return fmt.Errorf("Failed to GET epoch")
		}
		if er = json.NewDecoder(resp.Body).Decode(&epochResponse); er != nil {
			return fmt.Errorf("Failed to decode epoch response")
		}
		resp.Body.Close()
		epochs = append(epochs, epochResponse.Epoch)
	}
	log.Printf("Epochs fetched: %d", len(epochs))
	nextIndex := 0
	n := len(uploadResp.Chains)
	if err := ProcessFileInChunks(filename, bufferSize, func(c Chunk) error {
		chain := uploadResp.Chains[nextIndex]
		// Note: The sequence number that we use here does not represent the ID of the chunk. It represents the sequence number that the client has sent to the chain for replication.
		chunkUploadRequest := &ChunkUploadRequest{
			Epoch:          epochs[nextIndex],
			SequenceNumber: 0,
			ObjectId:       uploadResp.ObjectId,
			Data:           c.Data,
			ChunkId:        c.ID,
		}
		nextIndex = (nextIndex + 1) % n
		log.Printf("Sending chunk %d with epoch %d", c.ID, epochs[nextIndex])
		body, er := json.Marshal(chunkUploadRequest)
		if er != nil {
			return fmt.Errorf("Failed to marshal chunk: %v", err)
		}
		url := fmt.Sprintf("http://%s/write", chain.HeadAddress)
		resp, er = http.Post(url, "application/json", bytes.NewBuffer(body))
		if er != nil {
			return fmt.Errorf("Failed to upload chunk")
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("write failed: %s: %s", resp.Status, string(body))
		}
		var chunkUploadResponse ChunkUploadResponse
		if er = json.NewDecoder(resp.Body).Decode(&chunkUploadResponse); er != nil {
			log.Println(resp.Status)
			fmt.Println(er)
			resp.Body.Close()
			return fmt.Errorf("Failed to decode chunk upload response")
		}
		resp.Body.Close()
		if !chunkUploadResponse.IsWritten {
			return fmt.Errorf("Failed to write chunk %d", c.ID)
		}
		log.Printf("Chunk %d written successfully", c.ID)
		return nil
	}); err != nil {
		return err
	}
	completeUrl := "http://localhost:8000/upload-complete"
	notification := &UploadCompleteNotification{
		ObjectId: uploadResp.ObjectId,
	}
	body, err = json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("Failed to marshal upload notification request: %v", err)
	}
	resp, err = http.Post(completeUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Error occurred while sending upload completion notification: %v", err)
	}
	resp.Body.Close()
	return nil
}

func DeleteFile(filename string) {

}

func RetrieveFile(filename string) error {
	request := &RetrievalInitializationRequest{
		Key: filename,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("Failed to marshal retrieval request: %v", err)
	}
	url := "http://localhost:8000/retrieve"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Failed to send retrieval initialization request: %v", err)
	}
	var response RetrievalInitializationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("Error occurred while decoding retrieval initialization response: %v", err)
	}
	chunks, baseDir, err := ReceiveChunks(&response)
	if err != nil {
		return err
	}
	if err := AssembleFile(chunks, filename, baseDir); err != nil {
		return err
	}
	return nil
}
