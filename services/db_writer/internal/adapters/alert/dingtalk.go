// db_writer/adapters/alert -- 钉钉告警推送
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// DingtalkMessage 钉钉消息体
type DingtalkMessage struct {
	MsgType  string           `json:"msgtype"`
	Markdown DingtalkMarkdown `json:"markdown"`
}

// DingtalkMarkdown 钉钉 Markdown 消息
type DingtalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Alerter 钉钉告警推送器
type Alerter struct {
	webhookURL string
	client     *http.Client
}

// NewDingtalkAlerter 创建钉钉告警推送器
// webhookURL 为空时返回 noop alerter（不推送）
func NewDingtalkAlerter(webhookURL string) *Alerter {
	return &Alerter{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SendAlert 推送钉钉告警
// 使用 Markdown 格式发送告警消息
func (a *Alerter) SendAlert(ctx context.Context, title, text string) error {
	if a.webhookURL == "" {
		log.Printf("[alert] dingtalk webhook not configured, skipping alert: %s", title)
		return nil
	}

	msg := DingtalkMessage{
		MsgType: "markdown",
		Markdown: DingtalkMarkdown{
			Title: title,
			Text:  text,
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dingtalk message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}

	log.Printf("[alert] dingtalk alert sent: title=%s", title)
	return nil
}
