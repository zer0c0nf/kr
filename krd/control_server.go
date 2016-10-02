package main

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/agrinman/kr"
)

type ControlServer struct {
	enclaveClient EnclaveClientI
}

func NewControlServer() *ControlServer {
	cs := &ControlServer{UnpairedEnclaveClient()}
	return cs
}

func (cs *ControlServer) HandleControlHTTP(listener net.Listener) (err error) {
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/pair", cs.handlePair)
	httpMux.HandleFunc("/enclave", cs.handleEnclave)
	err = http.Serve(listener, httpMux)
	return
}

func (cs *ControlServer) handlePair(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cs.handleGetPair(w, r)
		return
	case http.MethodPut:
		cs.handlePutPair(w, r)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

//	check if pairing completed
func (cs *ControlServer) handleGetPair(w http.ResponseWriter, r *http.Request) {
	meResponse, err := cs.enclaveClient.RequestMe()
	if err == nil && meResponse != nil {
		err = json.NewEncoder(w).Encode(meResponse.Me)
		if err != nil {
			log.Error(err)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		if err != nil {
			log.Error(err)
		}
		return
	}
}

//	initiate new pairing (clearing any existing)
func (cs *ControlServer) handlePutPair(w http.ResponseWriter, r *http.Request) {
	pairingSecret, err := cs.enclaveClient.Pair()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Error(err)
		return
	}
	err = json.NewEncoder(w).Encode(pairingSecret)
	if err != nil {
		log.Error(err)
		return
	}
}

//	route request to enclave
func (cs *ControlServer) handleEnclave(w http.ResponseWriter, r *http.Request) {
	cachedMe := cs.enclaveClient.GetCachedMe()
	if cachedMe == nil {
		//	not paired
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var enclaveRequest kr.Request
	err := json.NewDecoder(r.Body).Decode(&enclaveRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if enclaveRequest.MeRequest != nil {
		response := kr.Response{
			MeResponse: &kr.MeResponse{
				Me: *cachedMe,
			},
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			log.Error(err)
			return
		}
		return
	}

	if enclaveRequest.SignRequest != nil {
		if enclaveRequest.SignRequest.Command == nil {
			enclaveRequest.SignRequest.Command = getLastCommand()
		}
		signResponse, err := cs.enclaveClient.RequestSignature(*enclaveRequest.SignRequest)
		if err != nil {
			log.Error("signature request error:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if signResponse != nil {
			response := kr.Response{
				RequestID:    enclaveRequest.RequestID,
				SignResponse: signResponse,
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				log.Error(err)
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	if enclaveRequest.ListRequest != nil {
		listResponse, err := cs.enclaveClient.RequestList(*enclaveRequest.ListRequest)
		if err != nil {
			log.Error("list request error:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if listResponse != nil {
			response := kr.Response{
				RequestID:    enclaveRequest.RequestID,
				ListResponse: listResponse,
			}
			err = json.NewEncoder(w).Encode(response)
			if err != nil {
				log.Error(err)
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	w.WriteHeader(http.StatusBadRequest)
}