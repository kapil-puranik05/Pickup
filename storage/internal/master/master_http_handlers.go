package master

import (
	"encoding/json"
	"net/http"
	"storage/internal/shared"
)

var (
	globalClusterLayout = &ClusterLayout{}
)

type Config struct {
	Epoch uint64 `json:"epoch"`
}

type LayoutDto struct {
	Epoch       uint64 `json:"epoch"`
	HeadAddress string `json:"headAddress"`
	TailAddress string `json:"tailAddress"`
}

func (m *MasterNodeRegistry) HandleRegisterNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var dto shared.NodeMetaDataDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "The node registration payload is invalid")
		return
	}
	m.registerNode(&dto)
	w.WriteHeader(http.StatusOK)
}

func (m *MasterNodeRegistry) HandleGetLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts GET requests")
		return
	}
	globalClusterLayout.LayoutMutex.RLock()
	defer globalClusterLayout.LayoutMutex.RUnlock()
	payload := &LayoutDto{
		HeadAddress: globalClusterLayout.HeadAddress,
		TailAddress: globalClusterLayout.TailAddress,
		Epoch:       globalClusterLayout.Epoch,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		writeError(w, http.StatusInternalServerError, "The master could not send the cluster layout")
	}
}

func (m *MasterNodeRegistry) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "This endpoint only accepts POST requests")
		return
	}
	var dto shared.NodeMetaDataDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeError(w, http.StatusBadRequest, "The heartbeat payload is invalid")
		return
	}
	err := m.updateLastSeen(dto)
	if err != nil {
		writeError(w, http.StatusNotFound, "The sending node is not registered with this master")
		return
	}
	w.WriteHeader(http.StatusOK)
}
