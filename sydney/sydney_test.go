package sydney

import (
	"go.uber.org/zap"
	"os"
	"sydney/provider"
	"testing"
)

func TestSydney_CreateConversation(t *testing.T) {
	sydney := NewSydney(zap.L(), provider.SydneyConfig{
		CookieU:   os.Getenv("BING_U"),
		UseProxy:  false,
		HttpProxy: "",
	})
	err := sydney.CreateConversation()
	if err != nil {
		t.Error(err)
	}
}
