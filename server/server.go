package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"hubble/datastore"
	"log"
	"net/http"
	"time"
)

// Server represents the HTTP server
type Server struct {
	Router    *mux.Router
	dataStore *datastore.DataStore
}

// NewServer creates a new HTTP server
func NewServer(dataStore *datastore.DataStore) *Server {
	server := &Server{
		Router:    mux.NewRouter(),
		dataStore: dataStore,
	}
	server.setupRoutes()
	return server
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	s.Router.HandleFunc("/api/flow", s.getFlowHandler).Methods("GET")
}

// getFlowHandler 根据key返回特定的流量信息
func (s *Server) getFlowHandler(w http.ResponseWriter, req *http.Request) {
	namespace := req.URL.Query().Get("namespace")
	pod := req.URL.Query().Get("pod")

	type Flow struct {
		From      string    `json:"from"`
		To        string    `json:"to"`
		Timestamp time.Time `json:"timestamp"`
	}

	var allFlows []Flow
	fromKey := fmt.Sprintf("%s-%s-from", namespace, pod)
	fromPods, err := s.dataStore.GetSetMembers(fromKey)
	if err != nil {
		http.Error(w, "获取流量数据失败", http.StatusInternalServerError)
		return
	}
	for _, toPod := range fromPods {
		flowKey := fmt.Sprintf("%s-%s-%s", namespace, pod, toPod)
		flowData, exists, err := s.dataStore.Get(flowKey)
		if err != nil {
			log.Printf("获取出站流量数据时出错，键[%s]: %v", flowKey, err)
			continue
		}
		if exists {
			timestamp, err := time.Parse(time.RFC3339, flowData)
			if err != nil {
				timestamp = time.Now()
			}
			flow := Flow{
				From:      pod,
				To:        toPod,
				Timestamp: timestamp,
			}
			allFlows = append(allFlows, flow)
		} else {
			log.Printf("未找到出站流量数据: %s -> %s", pod, toPod)
		}
	}
	toKey := fmt.Sprintf("%s-%s-to", namespace, pod)
	log.Printf("查询入站连接键: %s", toKey)
	toPods, err := s.dataStore.GetSetMembers(toKey)
	if err != nil {
		log.Printf("获取入站Pod列表时出错: %v", err)
		http.Error(w, "获取流量数据失败", http.StatusInternalServerError)
		return
	}
	for _, fromPod := range toPods {
		flowKey := fmt.Sprintf("%s-%s-%s", fromPod, namespace, pod)
		log.Printf("查询入站流量数据，键: %s", flowKey)

		flowData, exists, err := s.dataStore.Get(flowKey)
		if err != nil {
			log.Printf("获取入站流量数据时出错，键[%s]: %v", flowKey, err)
			continue
		}
		if exists {
			log.Printf("找到入站流量数据: %s -> %s", fromPod, pod)
			timestamp, err := time.Parse(time.RFC3339, flowData)
			if err != nil {
				log.Printf("解析时间戳失败[%s]，使用当前时间: %v", flowData, err)
				timestamp = time.Now()
			}
			flow := Flow{
				From:      fromPod,
				To:        pod,
				Timestamp: timestamp,
			}
			allFlows = append(allFlows, flow)
		} else {
			log.Printf("未找到入站流量数据: %s -> %s", fromPod, pod)
		}
	}
	response := struct {
		Flows []Flow `json:"flows"`
	}{
		Flows: allFlows,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "编码响应失败", http.StatusInternalServerError)
		return
	}
}
