package wechat_oa

// RelayEvent is the Cloud→instance normalized WeChat OA message payload.
type RelayEvent struct {
	RelayEventID    string `json:"relay_event_id"`
	MsgID           string `json:"msg_id"`
	AuthorizerAppID string `json:"authorizer_appid"`
	FromUser        string `json:"from_user"` // openid
	MsgType         string `json:"msg_type"`  // text|image|voice|event|...
	Content         string `json:"content"`
	CreateTime      int64  `json:"create_time"`
	Event           string `json:"event,omitempty"`
	EventKey        string `json:"event_key,omitempty"`
}
