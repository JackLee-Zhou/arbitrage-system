// orchestrator/adapters/trading_client — gRPC 客户端 → trading_agent:9090
// 封装对 trading_agent 的 RPC 调用。
// 注：当前为接口骨架。proto 代码生成后，所有 RPC 方法切到类型安全的 stub。
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
	"google.golang.org/grpc/status"
)

// Client trading_agent gRPC 客户端
type Client struct {
	conn   *grpc.ClientConn
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

	// 启动时验证连通性
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
	return &Client{conn: conn, target: target}, nil
}

// Ping 检查 gRPC 连接状态（检查当前连接状态，不等待变化）
func (c *Client) Ping(ctx context.Context) error {
	state := c.conn.GetState()
	switch state {
	case connectivity.Ready, connectivity.Idle:
		return nil
	default:
		return fmt.Errorf("grpc connection state: %s", state)
	}
}

// Close 关闭 gRPC 连接
func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("[trading_client] close error: %v", err)
	}
	log.Println("[trading_client] connection closed")
}

// ============================================================
// RPC 调用
// 当前为 JSON→gRPC 骨架实现，等 contracts/ proto 生成代码后：
//   1. 生成 trading_agent 的 gRPC stub（go package + service client）
//   2. 替换所有 invoke() 调用为生成的 client 方法
//   3. 删除 invoke() 内部方法
// ============================================================

// GetStatus 查询交易引擎状态
func (c *Client) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/GetStatus",
		map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetPositions 查询当前持仓
func (c *Client) GetPositions(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/GetPositions",
		map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// EmergencyClose 紧急平仓
func (c *Client) EmergencyClose(ctx context.Context, reason, symbol string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/EmergencyClose",
		map[string]interface{}{"reason": reason, "symbol": symbol}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// PauseStrategy 暂停策略
func (c *Client) PauseStrategy(ctx context.Context, reason string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/PauseStrategy",
		map[string]interface{}{"reason": reason}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ResumeStrategy 恢复策略
func (c *Client) ResumeStrategy(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/ResumeStrategy",
		map[string]interface{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UpdateRiskParams 更新风控参数
func (c *Client) UpdateRiskParams(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.invoke(ctx, "/arbitrage.v1.OrchestratorService/UpdateRiskParams",
		params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ============================================================
// invoke：通过 gRPC Conn 调用远程方法（JSON 编解码）。
// 注意：此方法依赖 proto 生成的服务端支持 JSON codec。
// 标准 gRPC 使用 protobuf codec，JSON 调用需服务端注册 JSON codec 或使用
// grpc.CallContentSubtype("json") 选项。
// 等 proto 生成后，所有 RPC 切到 generated stub → 删除此方法。
// ============================================================

func (c *Client) invoke(ctx context.Context, method string, req, resp interface{}) error {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("grpc marshal %s: %w", method, err)
	}

	var respBytes []byte
	// 使用 RawCodec 透传字节，避免 proto codec 对 []byte 做二次序列化
	opts := []grpc.CallOption{
		grpc.ForceCodec(rawCodec{}),
	}
	if err := c.conn.Invoke(ctx, method, reqBytes, &respBytes, opts...); err != nil {
		s, _ := status.FromError(err)
		return fmt.Errorf("grpc call %s: code=%s msg=%s", method, s.Code(), s.Message())
	}

	if err := json.Unmarshal(respBytes, resp); err != nil {
		return fmt.Errorf("grpc unmarshal %s: %w", method, err)
	}
	return nil
}

// rawCodec 是一个透传 codec，不编解码直接传递字节。
// 等 proto 生成后删除。
type rawCodec struct{}

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	switch b := v.(type) {
	case []byte:
		return b, nil
	default:
		return json.Marshal(v)
	}
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	switch p := v.(type) {
	case *[]byte:
		*p = data
		return nil
	default:
		return json.Unmarshal(data, v)
	}
}

func (rawCodec) Name() string { return "raw" }
