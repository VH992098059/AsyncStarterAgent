package trigger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignHMAC 计算 HMAC-SHA256 签名（测试用）
func SignHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC 验证 HMAC-SHA256 签名（恒定时间比较）
func VerifyHMAC(secret string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// NormalizeTodoist 将 Todoist webhook payload 标准化为 TriggerEvent
func NormalizeTodoist(eventName, eventID string, payload map[string]interface{}) TriggerEvent {
	normalized := map[string]interface{}{
		"raw_event_name": eventName,
	}
	if data, ok := payload["event_data"].(map[string]interface{}); ok {
		for k, v := range data {
			normalized[k] = v
		}
	}
	return NewWebhookEvent(SourceTodoist, eventName, eventID, normalized)
}

// NormalizeFeishu 飞书 webhook 标准化
func NormalizeFeishu(eventType, eventID string, payload map[string]interface{}) TriggerEvent {
	return NewWebhookEvent(SourceFeishu, eventType, eventID, payload)
}
