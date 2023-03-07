package sydney

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sydney/provider"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const DELIMITER byte = 0x1e

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
)

func appendIdentifier(msg map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	data = append(data, DELIMITER)
	return data, nil
}

func NewSydney(logger *zap.Logger, cfg provider.SydneyConfig) *Sydney {
	chat := Sydney{
		logger: logger,
		cfg:    cfg,
	}
	return &chat
}

type Sydney struct {
	logger *zap.Logger

	cfg provider.SydneyConfig

	Conv         *Conversation
	invocationId int

	conn *websocket.Conn
}

func (r *Sydney) Handshake(conn *websocket.Conn) error {
	req, err := appendIdentifier(map[string]interface{}{
		"protocol": "json",
		"version":  1,
	})
	if err != nil {
		return err
	}
	err = conn.WriteMessage(websocket.TextMessage, req)
	if err != nil {
		return err
	}

	var resp struct{}
	if err := conn.ReadJSON(&resp); err != nil {
		return err
	}
	return nil
}

func (r *Sydney) buildQuestion(prompt string) ([]byte, error) {
	question := map[string]interface{}{
		"arguments": []map[string]interface{}{
			{
				"source": "cib",
				"optionsSets": []string{
					"nlu_direct_response_filter",
					"deepleo",
					"disable_emoji_spoken_text",
					"responsible_ai_policy_235",
					"enablemm",
					"harmonyv3",
					"trn8req120",
					"rai253",
					"h3topp",
					"cricinfo",
					"cricinfov2",
					"localtime",
					"dv3sugg",
				},
				"allowedMessageTypes": []string{
					"Chat",
					"InternalSearchQuery",
					"InternalSearchResult",
					"Disengaged",
					"InternalLoaderMessage",
					"RenderCardRequest",
					"AdsQuery",
					"SemanticSerp",
					"GenerateContentQuery",
					"SearchQuery",
				},
				"sliceIds": []string{
					"checkauth",
					"222dtappids0",
					"302limit",
					"302limit",
					"228h3adss0",
					"h3adss0",
					"301rai253",
					"301rai253",
					"303h3topp",
					"225cricinfo",
					"225cricinfo",
					"224local",
					"224local",
				},
				"traceId":          uuid.New().String(),
				"isStartOfSession": r.invocationId == 0,
				"message": map[string]string{
					"author":      "user",
					"inputMethod": "Keyboard",
					"text":        prompt,
					"messageType": "Chat",
					"locale":      "zh-CN",
					"market":      "zh-CN",
					"region":      "TW",
				},
				"timestamp":             time.Now().Format(time.RFC3339),
				"conversationSignature": r.Conv.ConversationSignature,
				"participant": map[string]string{
					"id": r.Conv.ClientId,
				},
				"conversationId": r.Conv.ConversationId,
			},
		},
		"invocationId": strconv.Itoa(r.invocationId),
		"target":       "chat",
		"type":         4,
	}

	r.invocationId += 1

	req, err := appendIdentifier(question)
	if err != nil {
		return nil, err
	}

	return req, err
}

func (r *Sydney) CreateConversation() (err error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return
	}
	u, err := url.Parse("https://www.bing.com/turing/conversation/create")
	if err != nil {
		return err
	}
	jar.SetCookies(u, []*http.Cookie{
		{Name: "_U", Value: r.cfg.CookieU},
	})

	var client *http.Client
	if r.cfg.UseProxy {
		proxy, err := url.Parse(r.cfg.HttpProxy)
		if err != nil {
			return err
		}
		tr := &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{
			Jar:       jar,
			Transport: tr,
			Timeout:   time.Second * 30,
		}
	} else {
		client = &http.Client{
			Jar:     jar,
			Timeout: time.Second * 30,
		}
	}

	target := u.String()

	r.logger.Debug("create conversation", zap.String("url", target))

	defer func() {
		if err != nil {
			r.logger.Error("create conversation failed", zap.String("err", err.Error()))
		} else {
			r.logger.Debug("create conversation success",
				zap.String("ConversationId", r.Conv.ConversationId),
				zap.String("ClientId", r.Conv.ClientId),
				zap.String("ConversationSignature", r.Conv.ConversationSignature),
			)
		}
	}()
	request, err := http.NewRequest("get", target, nil)
	if err != nil {
		return
	}
	request.Header.Set("referer", "https://www.bing.com/search?q=Bing+AI&showconv=1&FORM=hpcodx")
	request.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36 Edg/110.0.1587.49")
	request.Header.Set("accept", "application/json")
	request.Header.Set("x-ms-client-request-id", uuid.New().String())
	request.Header.Set("x-ms-useragent", "azsdk-js-api-client-factory/1.0.0-beta.1 core-rest-pipeline/1.10.0 OS/Win32")

	response, err := client.Do(request)
	if err != nil {
		return
	}
	if response.StatusCode != http.StatusOK {
		err = ErrAuthenticationFailed
		return
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return
	}

	var conv Conversation
	err = json.Unmarshal(data, &conv)
	if err != nil {
		return
	}
	if conv.Result.Value == "UnauthorizedRequest" {
		err = errors.New(conv.Result.Message + " " + conv.Result.Value)
		return
	}

	if conv.ConversationId == "" || conv.ConversationSignature == "" {
		err = errors.New(string(data))
		return
	}

	r.Conv = &conv
	r.logger.Debug("CreateConversation", zap.Any("conv", conv))
	return
}

func (r *Sydney) Reset() {
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
	r.invocationId = 0
}

func (r *Sydney) GetConn() (conn *websocket.Conn, err error) {
	if r.conn == nil {
		u := url.URL{Scheme: "wss", Host: "sydney.bing.com", Path: "/sydney/ChatHub"}
		r.logger.Debug("ws connecting", zap.String("url", u.String()))
		defer func() {
			if err != nil {
				r.logger.Error("ws connect", zap.String("err", err.Error()))
			} else {
				r.logger.Debug("ws connected", zap.String("url", u.String()))
			}
		}()
		r.conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			return
		}

		if err = r.Handshake(r.conn); err != nil {
			return
		}
	}
	return r.conn, nil
}

func (r *Sydney) Ask(prompt string) (answers <-chan string, err error) {
	conn, err := r.GetConn()
	if err != nil {
		return nil, errors.Wrap(err, "get conn")
	}

	question, err := r.buildQuestion(prompt)
	if err != nil {
		return nil, errors.Wrap(err, "build question")
	}
	if err := conn.WriteMessage(websocket.TextMessage, question); err != nil {
		// 写入失败时重试一次，有可能是是因为必应服务端主动把连接断开了
		r.Reset()
		conn, err = r.GetConn()
		if err != nil {
			return nil, errors.Wrap(err, "get conn 2")
		}
		if err := conn.WriteMessage(websocket.TextMessage, question); err != nil {
			return nil, errors.Wrap(err, "write to ws")
		}
		return nil, err
	}
	ch := make(chan string)

	go func() {
		var finished = false
		for !finished {
			_, p, err := conn.ReadMessage()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					r.logger.Error("ws read", zap.String("err", err.Error()))
					ch <- "server internal error, please execute the /reset command"
					break
				}
				conn, err = r.GetConn()
				if err != nil {
					r.logger.Error("ws get", zap.String("err", err.Error()))
					ch <- "server internal error, please execute the /reset command"
					break
				}
				continue
			}
			messages := bytes.Split(p, []byte{DELIMITER})
			for _, message := range messages {
				if len(message) == 0 {
					continue
				}

				//r.logger.Debug("ws", zap.String("message", string(message)))

				result := gjson.Parse(string(message))
				_type := result.Get("type").Int()

				switch _type {
				case 1:

				case 2:
					finished = true

					if !strings.EqualFold(result.Get("item.result.value").String(), "success") {
						errorMessage := result.Get("item.result.message").String()
						r.logger.Error("ws", zap.String("err", errorMessage))
						ch <- errorMessage
					} else {
						answer := result.Get("item.messages.1.text").String()
						ch <- answer
					}
				}
			}
		}

		close(ch)
	}()
	return ch, nil
}
