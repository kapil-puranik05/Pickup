package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"router/internal/database"
	"router/internal/metadata"
	"router/internal/registry"
	"router/internal/repositories"

	"github.com/google/uuid"
)

var (
	repo *repositories.StorageObjectRepository
	reg  = &registry.Registry{}
)

func Init() {
	reg.InitializeRegistry()
	repo = repositories.NewStorageObjectRepository(database.DB)
}

type UploadIniitializationRequest struct {
	Key       string `json:"key"`
	Size      uint64 `json:"size"`
	ChunkSize int64  `json:"chunkSize"`
}

type UploadInitializationResponse struct {
	ObjectId string            `json:"objectId"`
	Chains   []*registry.Chain `json:"chains"`
}

type ChainRegistrationResponse struct {
	Registered bool `json:"registered"`
}

type RetrievalInitializationRequest struct {
	Key string `json:"key"`
}

type DeleteInitializationRequest struct {
	Key string `json:"key"`
}

type DeleteInitializationResponse struct {
	ObjectId string            `json:"objectId"`
	Chains   []*registry.Chain `json:"chains"`
}

type RetrievalInitializationResponse struct {
	ObjectId       string            `json:"objectId"`
	Chains         []*registry.Chain `json:"chains"`
	NumberOfChunks uint64            `json:"numChunks"`
}

type UploadCompleteNotification struct {
	ObjectId string `json:"objectId"`
}

type UploadCompleteResponse struct {
	Success bool `json:"success"`
}

type DeleteCompletionNotification struct {
	ObjectId string `json:"objectId"`
}

type DeleteCompletionResponse struct {
	Success bool `json:"success"`
}

func UploadInitializationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req UploadIniitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	object := &metadata.StorageObject{
		ID:        uuid.NewString(),
		Key:       req.Key,
		Size:      req.Size,
		ChunkSize: req.ChunkSize,
		Status:    metadata.ObjectUploading,
	}
	if err := repo.Create(object); err != nil {
		log.Printf("Error occurred while saving the object metadata: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	topology := reg.CopyTopology()
	response := &UploadInitializationResponse{
		ObjectId: object.ID,
		Chains:   topology.Chains,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func UploadCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req UploadCompleteNotification
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	object, err := repo.FindByID(req.ObjectId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	object.Status = metadata.ObjectReady
	repo.Update(object)
	response := UploadCompleteResponse{
		Success: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func RetrievalInitializationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RetrievalInitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	obj, err := repo.FindByKey(req.Key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if obj.Status != metadata.ObjectReady {
		http.Error(w, fmt.Sprintf("Error: Object with Object Id: %s was not found", req.Key), http.StatusNotFound)
		return
	}
	topology := reg.CopyTopology()
	var numChunks uint64
	if obj.Size%uint64(obj.ChunkSize) == 0 {
		numChunks = (obj.Size / uint64(obj.ChunkSize))
	} else {
		numChunks = (obj.Size / uint64(obj.ChunkSize)) + 1
	}
	response := &RetrievalInitializationResponse{
		ObjectId:       obj.ID,
		Chains:         topology.Chains,
		NumberOfChunks: numChunks,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func DeleteInitializationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DeleteInitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	obj, err := repo.FindByKey(req.Key)
	if err != nil {
		http.Error(w, "Object does not exist", http.StatusNotFound)
		return
	}
	if obj.Status != metadata.ObjectReady {
		http.Error(w, "Object does not exist", http.StatusNotFound)
		return
	}
	topology := reg.CopyTopology()
	response := &DeleteInitializationResponse{
		ObjectId: obj.ID,
		Chains:   topology.Chains,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func DeleteCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DeleteCompletionNotification
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	object, err := repo.FindByID(req.ObjectId)
	if err != nil {
		return
	}
	if err := repo.Delete(object.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := &DeleteCompletionResponse{
		Success: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ChainRegistrationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Error: Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req registry.ChainRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error: Unable to parse the request", http.StatusBadRequest)
		return
	}
	reg.RegisterChain(req)
	response := &ChainRegistrationResponse{
		Registered: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
