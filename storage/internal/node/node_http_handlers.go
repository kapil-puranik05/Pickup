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
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts GET requests")
		return
	}
	w.Write([]byte("Server is running"))
}

func NodeReconfigurationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var cmd shared.ReConfigCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, "The reconfiguration request payload is invalid")
		return
	}
	if err := node.reconfigure(cmd); err != nil {
		if err.Error() == "Stale Epoch" {
			writeError(w, http.StatusConflict, "The request was rejected because the epoch is outdated")
			return
		}
		writeError(w, http.StatusInternalServerError, "The node could not apply the reconfiguration request")
		return
	}
	response := &NodeReconfigurationResponse{
		Message:    "Node reconfigured successfully",
		StatusCode: http.StatusOK,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The node could not confirm the reconfiguration")
		return
	}
}

func WriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req shared.WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Decode error: %v", err)
		writeError(w, http.StatusBadRequest, "The write request payload is invalid")
		return
	}
	val, err := node.write(req)
	if err != nil {
		if err.Error() == "Stale Epoch" {
			writeError(w, http.StatusConflict, "The write request was rejected because the epoch is outdated")
			return
		}
		writeError(w, http.StatusInternalServerError, "The node could not complete the write request")
		return
	}
	response := &WriteResponse{
		IsWritten: val,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The node could not confirm the write request")
		return
	}
}

func ReadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req ReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The read request payload is invalid")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Chunk streaming is not supported on this connection")
		return
	}
	encoder := json.NewEncoder(w)
	if err := node.read(filepath.Join(node.nodeId, req.ObjectId), encoder, flusher); err != nil {
		log.Printf("Read error: %v", err)
		writeError(w, http.StatusInternalServerError, "The node could not read the requested object")
		return
	}
}

func AcknowlegementHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req shared.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The acknowledgement payload is invalid")
		return
	}
	if err := node.acknowledge(req); err != nil {
		if err.Error() == "Stale Epoch" {
			writeError(w, http.StatusConflict, "The acknowledgement was rejected because the epoch is outdated")
			return
		}
		writeError(w, http.StatusInternalServerError, "The node could not process the acknowledgement")
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
