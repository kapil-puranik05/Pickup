package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"storage/internal/shared"
)

var (
	node    = &Node{}
	Address string
)

type ReadRequest struct {
	ObjectId string `json:"objectId"`
}

type WriteResponse struct {
	IsWritten bool `json:"isWritten"`
}

type NodeReconfigurationResponse struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

func InitializeNode() {
	node.initializeNode()
	Address = node.address
}

func SendRegistrationRequest() {
	node.sendRegistrationRequest()
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Write([]byte("Server is running"))
}

func NodeReconfigurationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var cmd shared.ReConfigCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	if err := node.reconfigure(cmd); err != nil {
		if err.Error() == "Stale Epoch" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := &NodeReconfigurationResponse{
		Message:    "Node reconfigured successfully",
		StatusCode: http.StatusOK,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func WriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req shared.WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Decode error: %v", err)
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	val, err := node.write(req)
	if err != nil {
		if err.Error() == "Stale Epoch" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := &WriteResponse{
		IsWritten: val,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ReadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	encoder := json.NewEncoder(w)
	if err := node.read(filepath.Join(node.nodeId, req.ObjectId), encoder, flusher); err != nil {
		log.Printf("Read error: %v", err)
		return
	}
}

func AcknowlegementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req shared.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	if err := node.acknowledge(req); err != nil {
		if err.Error() == "Stale Epoch" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func SendHeartbeat() {
	hb := &shared.NodeMetaDataDto{
		Address: node.address,
		NodeId:  node.nodeId,
	}
	data, err := json.Marshal(&hb)
	if err != nil {
		log.Printf("Error occurred while parsing heartbeat: %v", err)
		return
	}
	url := fmt.Sprintf("http://%s/heartbeat", node.masterAddress)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error occurred while sending heartbeat: %v", err)
		return
	}
	resp.Body.Close()
}
