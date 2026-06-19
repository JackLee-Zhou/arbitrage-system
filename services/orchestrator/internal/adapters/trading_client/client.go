// orchestrator/adapters/trading_client — gRPC 客户端 → trading_agent:9090
//
// 使用 protoc 生成的 arbiragev1 包中的 OrchestratorServiceClient。
// 为兼容现有 HTTP handler（返回 map），通过 protojson 中转。
package trading_client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/arbitrage-system/contracts/generated/go/arbitragev1"
)

// Client trading_agent gRPC 客户端
type Client struct {
	conn   *grpc.ClientConn
	stub   arbitragev1.OrchestratorServiceClient
	target string
}

// NewClient 创建 gRPC 客户端并连接 trading_agent
func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc new client %s: %w", target, err)
	}

	conn.Connect()
	var state connectivity.State
	deadline := time.After(5 * time.Second)
	for {
		state = conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if state == connectivity.TransientFailure || state == connectivity.Shutdown {
			_ = conn.Close()
			return nil, fmt.Errorf("grpc dial %s: connection %s", target, state)
		}
		select {
		case <-deadline:
			_ = conn.Close()
			return nil, fmt.Errorf("grpc dial %s: context deadline exceeded", target)
		case <-time.After(100 * time.Millisecond):
		}
	}

	log.Printf("[trading_client] connected to %s", target)
	return &Client{
		conn:   conn,
		stub:   arbitragev1.NewOrchestratorServiceClient(conn),
		target: target,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	state := c.conn.GetState()
	switch state {
	case connectivity.Ready, connectivity.Idle:
		return nil
	default:
		return fmt.Errorf("grpc connection state: %s", state)
	}
}

func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("[trading_client] close error: %v", err)
	}
	log.Println("[trading_client] connection closed")
}

// ============================================================
// RPC 调用 — 使用 proto 生成的 stub，返回 map 兼容 HTTP handler
// ============================================================

func (c *Client) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.stub.GetStatus(ctx, &arbitragev1.StatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetStatus: %w", err)
	}
	return protoToMap(resp), nil
}

func (c *Client) GetPositions(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.stub.GetPositions(ctx, &arbitragev1.PositionRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetPositions: %w", err)
	}
	return protoToMap(resp), nil
}

func (c *Client) EmergencyClose(ctx context.Context, reason, symbol string) (map[string]interface{}, error) {
	resp, err := c.stub.EmergencyClose(ctx, &arbitragev1.EmergencyCloseRequest{
		Reason: reason,
		Symbol: symbol,
	})
	if err != nil {
		return nil, fmt.Errorf("EmergencyClose: %w", err)
	}
	return protoToMap(resp), nil
}

func (c *Client) PauseStrategy(ctx context.Context, reason string) (map[string]interface{}, error) {
	resp, err := c.stub.PauseStrategy(ctx, &arbitragev1.PauseStrategyRequest{
		RequestBy: reason,
	})
	if err != nil {
		return nil, fmt.Errorf("PauseStrategy: %w", err)
	}
	return protoToMap(resp), nil
}

func (c *Client) ResumeStrategy(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.stub.ResumeStrategy(ctx, &arbitragev1.ResumeStrategyRequest{})
	if err != nil {
		return nil, fmt.Errorf("ResumeStrategy: %w", err)
	}
	return protoToMap(resp), nil
}

func (c *Client) UpdateRiskParams(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	param, _ := params["risk_param"].(string)
	val, _ := params["new_value"].(float64)
	resp, err := c.stub.UpdateRiskParams(ctx, &arbitragev1.UpdateRiskRequest{
		RiskParam: param,
		NewValue:  val,
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateRiskParams: %w", err)
	}
	return protoToMap(resp), nil
}

// protoToMap proto 消息 → map[string]interface{}（protojson 中转，兼容 HTTP handler）
func protoToMap(m proto.Message) map[string]interface{} {
	j, _ := protojson.Marshal(m)
	var out map[string]interface{}
	_ = json.Unmarshal(j, &out)
	return out
}
