package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWantsEmbedBufferedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		query  string
		accept string
		want   bool
	}{
		{name: "stream=0", query: "stream=0", want: true},
		{name: "accept json", accept: "application/json", want: true},
		{name: "accept event-stream", accept: "text/event-stream", want: false},
		{name: "json and event-stream prefers stream", accept: "application/json, text/event-stream", want: false},
		{name: "default", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/api/v1/embed/ch/agent-chat/s"
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodPost, url, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			c.Request = req
			assert.Equal(t, tc.want, wantsEmbedBufferedJSON(c))
		})
	}
}
