// watchdog/adapters — 钉钉 Webhook 推送
package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// DingTalkAlert 钉钉告警推送器
type DingTalkAlert struct {
	webhookURL string
	client     *http.Client
}

// NewDingTalkAlert 创建钉钉告警推送器
func NewDingTalkAlert(webhookURL string, timeout time.Duration) *DingTalkAlert {
	return &DingTalkAlert{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Send 发送 Markdown 格式告警消息
func (d *DingTalkAlert) Send(title, text string) {
	if d.webhookURL == "" {
		log.Printf("[dingtalk] webhook URL not configured, skipping alert: %s", title)
		return
	}

	msg := dingTalkMessage{
		MsgType: "markdown",
		Markdown: dingTalkMarkdown{
			Title: title,
			Text:  fmt.Sprintf("## %s\n\n%s\n\n> 时间: %s", title, text, time.Now().Format("2006-01-02 15:04:05")),
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[dingtalk] marshal error: %v", err)
		return
	}

	resp, err := d.client.Post(d.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[dingtalk] send error: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[dingtalk] unexpected status: %d", resp.StatusCode)
	}
}

type dingTalkMessage struct {
	MsgType  string           `json:"msgtype"`
	Markdown dingTalkMarkdown `json:"markdown"`
}

type dingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
