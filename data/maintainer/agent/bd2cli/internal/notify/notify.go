// Package notify 把运行结果推给用户：Qmsg（QQ）、通用 Webhook、文件、或什么都不做。
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Notifier 是通知出口。
type Notifier interface {
	Notify(title, body string) error
}

// Nop 什么也不做。
type Nop struct{}

// Notify 实现 Notifier。
func (Nop) Notify(string, string) error { return nil }

// File 把通知追加写进一个文本文件。
type File struct {
	Path string
}

// Notify 实现 Notifier。
func (f File) Notify(title, body string) error {
	if dir := filepath.Dir(f.Path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	fh, err := os.OpenFile(f.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fmt.Fprintf(fh, "[%s] %s\n%s\n\n", time.Now().Format("2006-01-02 15:04:05"), title, body)
	return err
}

// Qmsg 通过 qmsg.zendee.cn 发 QQ 私聊消息。
type Qmsg struct {
	Key string
	QQ  string
}

// Notify 实现 Notifier。
func (q Qmsg) Notify(title, body string) error {
	if q.Key == "" {
		return fmt.Errorf("qmsg_key 为空")
	}
	form := url.Values{}
	form.Set("msg", title+"\n"+body)
	if q.QQ != "" {
		form.Set("qq", q.QQ)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm("https://qmsg.zendee.cn/send/"+url.PathEscape(q.Key), form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qmsg 返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// Webhook 把 JSON POST 到任意地址（企业微信机器人、自建服务等）。
type Webhook struct {
	URL     string
	Payload map[string]any // 可选：额外的固定字段
}

// Notify 实现 Notifier。
func (w Webhook) Notify(title, body string) error {
	if w.URL == "" {
		return fmt.Errorf("webhook_url 为空")
	}
	payload := map[string]any{
		"title": title,
		"text":  title + "\n" + body,
		"msgtype": "text",
		"text_content": map[string]any{"content": title + "\n" + body},
	}
	for k, v := range w.Payload {
		payload[k] = v
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(w.URL, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// Config 是 config/bd2cli.json 里的 notify 段。
type Config struct {
	Provider   string `json:"provider"` // qmsg | webhook | file | none
	QmsgKey    string `json:"qmsg_key"`
	QmsgQQ     string `json:"qmsg_qq"`
	WebhookURL string `json:"webhook_url"`
	File       string `json:"file"`
}

// Load 读取 root/config/bd2cli.json 并构造通知器；没有配置时返回 Nop。
func Load(root string) (Notifier, string) {
	path := filepath.Join(root, "config", "bd2cli.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return Nop{}, "未配置（config/bd2cli.json 不存在）"
	}
	var wrapper struct {
		Notify Config `json:"notify"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return Nop{}, "配置解析失败: " + err.Error()
	}
	c := wrapper.Notify
	switch c.Provider {
	case "qmsg":
		if c.QmsgKey == "" {
			return Nop{}, "qmsg 缺少 qmsg_key"
		}
		return Qmsg{Key: c.QmsgKey, QQ: c.QmsgQQ}, "qmsg"
	case "webhook":
		if c.WebhookURL == "" {
			return Nop{}, "webhook 缺少 webhook_url"
		}
		return Webhook{URL: c.WebhookURL}, "webhook"
	case "file":
		p := c.File
		if p == "" {
			p = filepath.Join(root, "logs", "notify.log")
		} else if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return File{Path: p}, "file(" + p + ")"
	case "", "none":
		return Nop{}, "已关闭"
	default:
		return Nop{}, "未知 provider: " + c.Provider
	}
}
