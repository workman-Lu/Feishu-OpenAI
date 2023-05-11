package main

import (
	"context"
	"log"

	"start-feishubot/handlers"
	"start-feishubot/initialization"
	"start-feishubot/services/openai"

	"github.com/gin-gonic/gin"
	sdkginext "github.com/larksuite/oapi-sdk-gin"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/spf13/pflag"
)

var (
	cfg = pflag.StringP("config", "c", "./config.yaml", "apiserver config file path.")
)

type Event struct {
	UserID          string `json:"user_id"`
	TextWithoutAtBot string `json:"text_without_at_bot"`
}

type RequestData struct {
	Event Event `json:"event"`
}

func main() {
	initialization.InitRoleList()
	pflag.Parse()
	config := initialization.LoadConfig(*cfg)
	initialization.LoadLarkClient(*config)
	gpt := openai.NewChatGPT(*config)
	handlers.InitHandlers(gpt, *config)

	eventHandler := dispatcher.NewEventDispatcher(
		config.FeishuAppVerificationToken, config.FeishuAppEncryptKey).
		OnP2MessageReceiveV1(handlers.Handler).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			return handlers.ReadHandler(ctx, event)
		})

	cardHandler := larkcard.NewCardActionHandler(
		config.FeishuAppVerificationToken, config.FeishuAppEncryptKey,
		handlers.CardHandler())
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		start := time.Now()
		var requestData RequestData

		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(400, gin.H{"warming": "Invalid request data"})
		}

		userid := requestData.Event.UserID
		content := requestData.Event.TextWithoutAtBot
		defer func() {
			fmt.Sprintf("[SEASUN] %v | %3d | %13v | %15s |%s %-7s\n",
				start.Format("2006/01/02 - 15:04:05"),
				c.Writer.Status(),
				time.Now().Sub(start),
				c.Request.RemoteAddr,
				c.Request.Method,
				c.Request.URL.Path,
				userid,
				content,
			)
		}()
		c.Next()
	})
	
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.POST("/webhook/event",
		sdkginext.NewEventHandlerFunc(eventHandler))
	r.POST("/webhook/card",
		sdkginext.NewCardActionHandlerFunc(
			cardHandler))

	if err := initialization.StartServer(*config, r); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
