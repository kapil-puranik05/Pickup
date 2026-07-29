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
	Command        string `json:"command"`
}

type ChunkUploadResponse struct {
	IsWritten bool `json:"isWritten"`
}

type ChunkRetrievalRequest struct {
	ObjectId string `json:"objectId"`
}

type DeleteInitializationRequest struct {
	Key string `json:"key"`
}

type DeleteInitializationResponse struct {
	ObjectId string   `json:"objectId"`
	Chains   []*Chain `json:"chains"`
}

type ChunksDeletionRequest struct {
	Epoch    uint64 `json:"epoch"`
	ObjectId string `json:"objectId"`
	Command  string `json:"command"`
}

type ChunksDeletionResponse struct {
	IsWritten bool `json:"isWritten"`
}

type Config struct {
	Epoch uint64 `json:"epoch"`
}

type UploadCompleteNotification struct {
	ObjectId string `json:"objectId"`
}

type DeleteCompletionNotification struct {
	ObjectId string `json:"objectId"`
}

type ChunkIndex struct {
	ID   uint64
	path string
}

func ReceiveChunks(req *RetrievalInitializationResponse) ([]*ChunkIndex, string, error) {
	baseDir := filepath.Join(req.ObjectId, "dump")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, "", fmt.Errorf("Could not prepare temporary storage for retrieval")
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
				errCh <- fmt.Errorf("Could not prepare the retrieval request")
				return
			}
			url := fmt.Sprintf("http://%s/read", chain.TailAddress)
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
			if err != nil {
				errCh <- fmt.Errorf("Could not retrieve data from storage")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errCh <- readAPIError(resp, "Could not retrieve data from storage")
				return
			}
			decoder := json.NewDecoder(resp.Body)
			for {
				var chunk Chunk
				err := decoder.Decode(&chunk)
				if err == io.EOF {
					break
				}
				if err != nil {
					errCh <- fmt.Errorf("Could not read chunk %d from storage", chunk.ID)
					return
				}
				path := filepath.Join(baseDir, strconv.FormatUint(chunk.ID, 10))
				if err := os.WriteFile(path, chunk.Data, 0644); err != nil {
					errCh <- fmt.Errorf("Could not store temporary chunk %d", chunk.ID)
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
		return nil, "", fmt.Errorf("Retrieved data is incomplete")
	}
	sort.Slice(chunks, func(i int, j int) bool {
		return chunks[i].ID < chunks[j].ID
	})
	return chunks, baseDir, nil
}

func AssembleFile(chunks []*ChunkIndex, key string, baseDir string) error {
	outputPath := filepath.Join(key)
	objectDir := filepath.Dir(baseDir)
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("Could not create the output file")
	}
	defer out.Close()
	for _, chunk := range chunks {
		in, err := os.Open(chunk.path)
		if err != nil {
			return fmt.Errorf("Could not open temporary chunk %d", chunk.ID)
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			return fmt.Errorf("Could not assemble the retrieved file")
		}
		in.Close()
		if err := os.Remove(chunk.path); err != nil {
			return fmt.Errorf("Could not clean up temporary chunk %d", chunk.ID)
		}
	}
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("Could not clean up temporary retrieval data")
	}
	if objectDir != "." && objectDir != "" {
		if err := os.Remove(objectDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Could not remove the temporary object directory")
		}
	}
	return nil
}

func RemoveChunks(req *DeleteInitializationResponse) error {
	epochs := make([]uint64, 0)
	for _, chain := range req.Chains {
		configURL := fmt.Sprintf("http://%s/layout", chain.MasterAddress)
		var epochResponse Config
		resp, er := http.Get(configURL)
		if er != nil {
			return fmt.Errorf("Could not fetch the current cluster layout")
		}
		if er = json.NewDecoder(resp.Body).Decode(&epochResponse); er != nil {
			resp.Body.Close()
			return fmt.Errorf("Received an invalid cluster layout response")
		}
		resp.Body.Close()
		epochs = append(epochs, epochResponse.Epoch)
	}
	for index, chain := range req.Chains {
		request := &ChunksDeletionRequest{
			Epoch:    epochs[index],
			ObjectId: req.ObjectId,
			Command:  "DELETE",
		}
		body, err := json.Marshal(request)
		if err != nil {
			return fmt.Errorf("Could not prepare the delete request")
		}
		url := fmt.Sprintf("http://%s/write", chain.HeadAddress)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("Could not send the delete request to storage")
		}
		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return readAPIError(resp, "Storage rejected the delete request")
		}
		var response ChunksDeletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			resp.Body.Close()
			return fmt.Errorf("Received an invalid delete response from storage")
		}
		resp.Body.Close()
		if !response.IsWritten {
			return fmt.Errorf("Storage could not delete the object from chain %s", chain.ChainId)
		}
	}
	return nil
}

func ProcessFileInChunks(filename string, bufferSize int, processor func(Chunk) error) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Could not open the file")
	}
	defer file.Close()
	buffer := make([]byte, bufferSize)
	var chunkID uint64 = 0
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])
			chunk := Chunk{
				ID:   chunkID,
				Data: data,
			}
			if err := processor(chunk); err != nil {
				return err
			}
			chunkID++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Could not read the file")
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
		return fmt.Errorf("Could not read the selected file")
	}
	request := &UploadRequest{
		Key:       filename,
		Size:      size,
		ChunkSize: (64 * 1024 * 1024),
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("Could not prepare the upload request")
	}
	resp, err := http.Post("http://localhost:8000/upload", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Could not reach the router service")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return readAPIError(resp, "Upload could not be started")
	}
	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return fmt.Errorf("Received an invalid upload response from the router")
	}
	log.Printf("Chains received: %d", len(uploadResp.Chains))
	epochs := make([]uint64, 0)
	for _, chain := range uploadResp.Chains {
		configURL := fmt.Sprintf("http://%s/layout", chain.MasterAddress)
		var epochResponse Config
		resp, er := http.Get(configURL)
		if er != nil {
			return fmt.Errorf("Could not fetch the current cluster layout")
		}
		if er = json.NewDecoder(resp.Body).Decode(&epochResponse); er != nil {
			resp.Body.Close()
			return fmt.Errorf("Received an invalid cluster layout response")
		}
		resp.Body.Close()
		epochs = append(epochs, epochResponse.Epoch)
	}
	log.Printf("Epochs fetched: %d", len(epochs))
	nextIndex := 0
	n := len(uploadResp.Chains)
	if err := ProcessFileInChunks(filename, bufferSize, func(c Chunk) error {
		chain := uploadResp.Chains[nextIndex]
		chunkUploadRequest := &ChunkUploadRequest{
			Epoch:          epochs[nextIndex],
			SequenceNumber: 0,
			ObjectId:       uploadResp.ObjectId,
			Data:           c.Data,
			ChunkId:        c.ID,
			Command:        "SET",
		}
		nextIndex = (nextIndex + 1) % n
		log.Printf("Sending chunk %d with epoch %d", c.ID, epochs[nextIndex])
		body, er := json.Marshal(chunkUploadRequest)
		if er != nil {
			return fmt.Errorf("Could not prepare file data for upload")
		}
		url := fmt.Sprintf("http://%s/write", chain.HeadAddress)
		resp, er = http.Post(url, "application/json", bytes.NewBuffer(body))
		if er != nil {
			return fmt.Errorf("Could not upload file data to storage")
		}
		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			return readAPIError(resp, "Storage rejected the upload request")
		}
		var chunkUploadResponse ChunkUploadResponse
		if er = json.NewDecoder(resp.Body).Decode(&chunkUploadResponse); er != nil {
			log.Println(resp.Status)
			fmt.Println(er)
			resp.Body.Close()
			return fmt.Errorf("Received an invalid response from storage")
		}
		resp.Body.Close()
		if !chunkUploadResponse.IsWritten {
			return fmt.Errorf("Storage could not save chunk %d", c.ID)
		}
		log.Printf("Chunk %d written successfully", c.ID)
		return nil
	}); err != nil {
		return err
	}
	completeURL := "http://localhost:8000/upload-complete"
	notification := &UploadCompleteNotification{
		ObjectId: uploadResp.ObjectId,
	}
	body, err = json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("Could not prepare the upload completion request")
	}
	resp, err = http.Post(completeURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Upload finished, but completion could not be confirmed")
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return readAPIError(resp, "Upload finished, but completion could not be confirmed")
	}
	resp.Body.Close()
	return nil
}

func DeleteFile(filename string) error {
	request := &DeleteInitializationRequest{
		Key: filename,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("Could not prepare the delete request")
	}
	url := "http://localhost:8000/delete"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Could not reach the router service")
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return readAPIError(resp, "Delete could not be started")
	}
	var response DeleteInitializationResponse
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		resp.Body.Close()
		return fmt.Errorf("Received an invalid delete response from the router")
	}
	resp.Body.Close()
	if err := RemoveChunks(&response); err != nil {
		return err
	}
	notification := &DeleteCompletionNotification{
		ObjectId: response.ObjectId,
	}
	body, err = json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("Could not prepare the delete completion request")
	}
	url = "http://localhost:8000/delete-complete"
	resp, err = http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Delete finished, but completion could not be confirmed")
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return readAPIError(resp, "Delete finished, but completion could not be confirmed")
	}
	resp.Body.Close()
	return nil
}

func RetrieveFile(filename string) error {
	request := &RetrievalInitializationRequest{
		Key: filename,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("Could not prepare the retrieval request")
	}
	url := "http://localhost:8000/retrieve"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("Could not reach the router service")
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return readAPIError(resp, "Retrieval could not be started")
	}
	var response RetrievalInitializationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		resp.Body.Close()
		return fmt.Errorf("Received an invalid retrieval response from the router")
	}
	resp.Body.Close()
	chunks, baseDir, err := ReceiveChunks(&response)
	if err != nil {
		return err
	}
	if err := AssembleFile(chunks, filename, baseDir); err != nil {
		return err
	}
	return nil
}
