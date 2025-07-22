package hubble

import (
	"context"
	"fmt"
	"hubble/datastore"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/cilium/cilium/api/v1/flow"
	observer "github.com/cilium/cilium/api/v1/observer"
)

// Collector 从Hubble收集流量数据
type Collector struct {
	hubbleAddr string               // Hubble服务地址
	dataStore  *datastore.DataStore // 数据存储
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewCollector(hubbleAddr string, dataStore *datastore.DataStore) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		hubbleAddr: hubbleAddr,
		dataStore:  dataStore,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func extractKey(flow *pb.Flow) (string, string, string, string, bool) {
	if flow.GetSource() == nil || flow.GetDestination() == nil {
		return "", "", "", "", false
	}
	srcNs := flow.GetSource().GetNamespace()
	srcPod := flow.GetSource().GetPodName()
	dstNs := flow.GetDestination().GetNamespace()
	dstPod := flow.GetDestination().GetPodName()
	if srcNs == "" || srcPod == "" || dstNs == "" || dstPod == "" {
		return "", "", "", "", false
	}

	// 只允许ns-开头的namespace
	if !strings.HasPrefix(srcNs, "ns-") || !strings.HasPrefix(dstNs, "ns-") {
		return "", "", "", "", false
	}

	return srcNs, srcPod, dstNs, dstPod, true
}

// Start 开始从Hubble收集流量数据
func (hc *Collector) Start() {
	// 连接到Hubble服务
	conn, err := grpc.Dial(
		hc.hubbleAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("连接Hubble失败: %v", err)
	}
	defer conn.Close()

	// 创建Hubble观察者客户端
	client := observer.NewObserverClient(conn)

	// 创建请求，使用跟踪模式
	req := &observer.GetFlowsRequest{
		Follow: true,
	}

	// 获取流式数据
	stream, err := client.GetFlows(hc.ctx, req)
	if err != nil {
		log.Fatalf("获取流量数据失败: %v", err)
	}

	// 处理流式数据
	for {
		select {
		case <-hc.ctx.Done():
			return
		default:
			resp, err := stream.Recv()
			if err != nil {
				log.Printf("流关闭或发生错误: %v", err)
				return
			}

			flow := resp.GetFlow()
			if flow == nil {
				continue
			}

			srcNs, srcPod, dstNs, dstPod, ok := extractKey(flow)
			if !ok {
				continue
			}

			timestamp := time.Now()
			if flow.GetTime() != nil {
				timestamp = flow.GetTime().AsTime()
			}
			err = hc.UpdateFlowRelationships(srcNs, srcPod, dstNs, dstPod, timestamp)
			if err != nil {
				log.Printf("Error updating flow relationships: %v", err)
			}
		}
	}
}

// Stop 停止收集器
func (hc *Collector) Stop() {
	hc.cancel()
}

// UpdateFlowRelationships 更新流量关系并记录时间
func (hc *Collector) UpdateFlowRelationships(srcNs, srcPod, dstNs, dstPod string, timestamp time.Time) error {
	// 构建键名
	srcToKey := fmt.Sprintf("%s-%s-to", srcNs, srcPod)
	dstFromKey := fmt.Sprintf("%s-%s-from", dstNs, dstPod)
	connectionKey := fmt.Sprintf("%s-%s-%s-%s", srcNs, srcPod, dstNs, dstPod)

	// 目标值
	dstValue := fmt.Sprintf("%s-%s", dstNs, dstPod)
	srcValue := fmt.Sprintf("%s-%s", srcNs, srcPod)
	timeValue := timestamp.Format(time.RFC3339)

	// 更新源到目标集合
	if err := hc.dataStore.AddToSet(srcToKey, dstValue); err != nil {
		return fmt.Errorf("failed to update source-to-destination set: %w", err)
	}

	// 更新目标到源集合
	if err := hc.dataStore.AddToSet(dstFromKey, srcValue); err != nil {
		return fmt.Errorf("failed to update destination-from-source set: %w", err)
	}

	// 记录连接时间
	if err := hc.dataStore.Set(connectionKey, timeValue); err != nil {
		return fmt.Errorf("failed to set connection timestamp: %w", err)
	}
	return nil
}
