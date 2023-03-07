package tg

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sydney/provider"
	"sydney/sydney"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

func NewBot(logger *zap.Logger, cfg provider.TelegramBotConfig) *Bot {
	ctx, cancel := context.WithCancel(context.Background())
	b := Bot{
		logger:    logger,
		cfg:       cfg,
		bingData:  cache.New(5*time.Minute, 10*time.Second),
		processed: make(map[string]bool),
		ctx:       ctx,
		cancel:    cancel,
	}
	b.bingData.OnEvicted(func(tgUserId string, i interface{}) {
		b.DeleteProcessed(tgUserId)
		b.logger.Debug("conversation timeout", zap.String("tgUserId", tgUserId))
	})
	return &b
}

type Bot struct {
	logger *zap.Logger

	cfg provider.TelegramBotConfig

	bingData *cache.Cache
	bingLock sync.Mutex

	tgBot *tgbotapi.BotAPI

	processed     map[string]bool
	processedLock sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

func (b *Bot) GetSydney(tgUserId string, logger *zap.Logger) (*sydney.Sydney, error) {
	b.bingLock.Lock()
	defer b.bingLock.Unlock()
	var bing *sydney.Sydney

	val, ok := b.bingData.Get(tgUserId)
	if ok {
		bing = val.(*sydney.Sydney)
	} else {
		bing = sydney.NewSydney(b.cfg.BingU, logger)
		err := bing.CreateConversation()
		if err != nil {
			return nil, err
		}
	}
	b.bingData.Set(tgUserId, bing, time.Minute*5)

	return bing, nil
}

func (b *Bot) Start() (err error) {
	var tgBot *tgbotapi.BotAPI
	if b.cfg.UseProxy {
		proxy, err := url.Parse(b.cfg.HttpProxy)
		if err != nil {
			return err
		}
		tr := &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}

		tgBot, err = tgbotapi.NewBotAPIWithClient(b.cfg.ApiKey, tgbotapi.APIEndpoint, &http.Client{
			Transport: tr,
			Timeout:   time.Second * 30,
		})
		if err != nil {
			return err
		}
	} else {
		tgBot, err = tgbotapi.NewBotAPI(b.cfg.ApiKey)
		if err != nil {
			return err
		}
	}

	b.logger.Debug("Authorized", zap.String("account", tgBot.Self.UserName))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	b.tgBot = tgBot

	go func() {
		updates := tgBot.GetUpdatesChan(u)
		logger := b.logger

		logger.Debug("bot start")
		defer logger.Debug("bot stopped")

		for {
			select {
			case <-b.ctx.Done():
				return
			case update := <-updates:
				if update.Message == nil {
					continue
				}
				message := update.Message
				from := message.From.String()
				tgUserId := strconv.FormatInt(message.From.ID, 10)

				if message.Text == "" {
					continue
				}

				// Reply to other user's messages will not be processed
				if message.ReplyToMessage != nil && message.ReplyToMessage.From.UserName != tgBot.Self.UserName {
					continue
				}

				if b.GetProcessed(tgUserId) {
					msg := tgbotapi.NewMessage(message.Chat.ID, b.cfg.RepeatedAnswer)
					msg.ReplyToMessageID = message.MessageID
					b.Send(msg)
					continue
				}

				b.SetProcessed(tgUserId, true)
				go b.handleUserMessage(tgUserId, from, message)
			}
		}
	}()
	return nil
}

func (b *Bot) handleUserMessage(tgUserId string, from string, message *tgbotapi.Message) {
	defer b.SetProcessed(tgUserId, false)

	loggerName := fmt.Sprintf(`[%s:%s]`, tgUserId, from)
	logger := b.logger.Named(loggerName)
	logger.Info("request", zap.String("message", message.Text))

	msg := tgbotapi.NewMessage(message.Chat.ID, "")
	msg.ReplyToMessageID = message.MessageID

	chat, err := b.GetSydney(tgUserId, logger)
	if err != nil {
		msg.Text = b.cfg.FallbackAnswer
		b.Send(msg)
		return
	}

	if message.IsCommand() {
		switch message.Command() {
		case "start":
			msg.Text = b.cfg.CommandStartAnswer
		case "reset":
			chat.Reset()
			if err := chat.CreateConversation(); err != nil {
				logger.Error("create conversation", zap.String("err", err.Error()))
				msg.Text = "I can't create conversation with AI."
			} else {
				msg.Text = b.cfg.CommandResetAnswer
			}
		case "help":
			msg.Text = b.cfg.CommandHelpAnswer
		default:
			msg.Text = "I can't understand your command."
		}
	} else {
		answers, err := chat.Ask(message.Text)
		if err != nil {
			msg.Text = b.cfg.FallbackAnswer
			logger.Error("ask", zap.NamedError("err", err))
		} else {
			for {
				answer, ok := <-answers
				if !ok {
					break
				}
				msg.Text = answer
			}
		}
		if msg.Text == "" {
			logger.Error("answer is empty")
			msg.Text = b.cfg.FallbackAnswer
		}
	}
	logger.Info("replay", zap.String("message", msg.Text))
	b.Send(msg)
}

func (b *Bot) SetProcessed(userId string, p bool) {
	b.processedLock.Lock()
	defer b.processedLock.Unlock()
	b.processed[userId] = p
}

func (b *Bot) GetProcessed(userId string) bool {
	b.processedLock.Lock()
	defer b.processedLock.Unlock()
	return b.processed[userId]
}

func (b *Bot) DeleteProcessed(userId string) {
	b.processedLock.Lock()
	defer b.processedLock.Unlock()
	delete(b.processed, userId)
}

func (b *Bot) Send(msg tgbotapi.MessageConfig) {
	_, err := b.tgBot.Send(msg)
	if err != nil {
		b.logger.Error("tg send", zap.NamedError("err", err))
	}
}

func (b *Bot) Stop() error {
	b.logger.Debug("bot stop")
	b.cancel()
	return nil
}
