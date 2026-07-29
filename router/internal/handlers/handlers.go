package handlers

import (
	"encoding/json"
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
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req UploadIniitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The upload request payload is invalid")
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
		writeError(w, http.StatusInternalServerError, "The router could not create metadata for this object")
		return
	}
	topology := reg.CopyTopology()
	response := &UploadInitializationResponse{
		ObjectId: object.ID,
		Chains:   topology.Chains,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not send the upload response")
		return
	}
}

func UploadCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req UploadCompleteNotification
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The upload completion payload is invalid")
		return
	}
	object, err := repo.FindByID(req.ObjectId)
	if err != nil {
		writeError(w, http.StatusNotFound, "The uploaded object could not be found")
		return
	}
	object.Status = metadata.ObjectReady
	repo.Update(object)
	response := UploadCompleteResponse{
		Success: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not confirm upload completion")
		return
	}
}

func RetrievalInitializationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req RetrievalInitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The retrieval request payload is invalid")
		return
	}
	obj, err := repo.FindByKey(req.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "No object was found for the requested key")
		return
	}
	if obj.Status != metadata.ObjectReady {
		writeError(w, http.StatusNotFound, "The requested object is not available for retrieval")
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
		writeError(w, http.StatusInternalServerError, "The router could not send the retrieval response")
		return
	}
}

func DeleteInitializationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req DeleteInitializationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The delete request payload is invalid")
		return
	}
	obj, err := repo.FindByKey(req.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "No object was found for the requested key")
		return
	}
	if obj.Status != metadata.ObjectReady {
		writeError(w, http.StatusNotFound, "The requested object is not available for deletion")
		return
	}
	topology := reg.CopyTopology()
	response := &DeleteInitializationResponse{
		ObjectId: obj.ID,
		Chains:   topology.Chains,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not send the delete response")
		return
	}
}

func DeleteCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req DeleteCompletionNotification
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The delete completion payload is invalid")
		return
	}
	object, err := repo.FindByID(req.ObjectId)
	if err != nil {
		writeError(w, http.StatusNotFound, "The object scheduled for deletion could not be found")
		return
	}
	if err := repo.Delete(object.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not remove the object metadata")
		return
	}
	response := &DeleteCompletionResponse{
		Success: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not confirm delete completion")
		return
	}
}

func ChainRegistrationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var req registry.ChainRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "The chain registration payload is invalid")
		return
	}
	reg.RegisterChain(req)
	response := &ChainRegistrationResponse{
		Registered: true,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		writeError(w, http.StatusInternalServerError, "The router could not confirm chain registration")
		return
	}
}
