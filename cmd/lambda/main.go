// PicoClaw - AWS Lambda serverless handler
// Receives webhook events from Telegram (via API Gateway) and processes them synchronously.
//
// Environment variables:
//   PICOCLAW_CONFIG_JSON       - Full config JSON (alternative to config file)
//   PICOCLAW_WORKSPACE         - Workspace path (default: /tmp/picoclaw)
//   PICOCLAW_TELEGRAM_TOKEN    - Telegram bot token (overrides config)
//   PICOCLAW_WEBHOOK_SECRET    - Optional secret token for webhook verification

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
)

var (
	agentLoop *agent.AgentLoop
	bot       *tgbotapi.BotAPI
	cfg       *config.Config
	allowList []string
	initOnce  sync.Once
	initErr   error
)

func initialize() error {
	initOnce.Do(func() {
		initErr = doInit()
	})
	return initErr
}

func doInit() error {
	var err error
	cfg, err = loadLambdaConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override workspace for Lambda
	workspace := os.Getenv("PICOCLAW_WORKSPACE")
	if workspace == "" {
		workspace = "/tmp/picoclaw"
	}
	cfg.Agents.Defaults.Workspace = workspace
	os.MkdirAll(workspace, 0755)

	// Telegram token override
	if token := os.Getenv("PICOCLAW_TELEGRAM_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
		cfg.Channels.Telegram.Enabled = true
	}

	if cfg.Channels.Telegram.Token == "" {
		return fmt.Errorf("PICOCLAW_TELEGRAM_TOKEN or config telegram token required")
	}

	// Init Telegram bot (for sending replies only, no polling)
	bot, err = tgbotapi.NewBotAPI(cfg.Channels.Telegram.Token)
	if err != nil {
		return fmt.Errorf("creating telegram bot: %w", err)
	}

	allowList = cfg.Channels.Telegram.AllowFrom

	// Init LLM provider
	provider, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	// Init agent loop (used synchronously, no goroutines)
	msgBus := bus.NewMessageBus()
	agentLoop = agent.NewAgentLoop(cfg, msgBus, provider)

	log.Printf("Lambda initialized: model=%s, workspace=%s", cfg.Agents.Defaults.Model, workspace)
	return nil
}

func loadLambdaConfig() (*config.Config, error) {
	// Try loading from JSON env var first
	if cfgJSON := os.Getenv("PICOCLAW_CONFIG_JSON"); cfgJSON != "" {
		cfg := config.DefaultConfig()
		if err := json.Unmarshal([]byte(cfgJSON), cfg); err != nil {
			return nil, fmt.Errorf("parsing PICOCLAW_CONFIG_JSON: %w", err)
		}
		return cfg, nil
	}

	// Fall back to config file
	configPath := os.Getenv("PICOCLAW_CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}
	return config.LoadConfig(configPath)
}

func isAllowed(senderID string) bool {
	if len(allowList) == 0 {
		return true
	}
	userID := senderID
	if idx := strings.Index(senderID, "|"); idx != -1 {
		userID = senderID[:idx]
	}
	for _, allowed := range allowList {
		if senderID == allowed || userID == allowed {
			return true
		}
	}
	return false
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	if err := initialize(); err != nil {
		log.Printf("Init error: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// Verify webhook secret if configured
	if secret := os.Getenv("PICOCLAW_WEBHOOK_SECRET"); secret != "" {
		headerSecret := request.Headers["x-telegram-bot-api-secret-token"]
		if headerSecret != secret {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
		}
	}

	// Parse Telegram update
	var update tgbotapi.Update
	if err := json.Unmarshal([]byte(request.Body), &update); err != nil {
		log.Printf("Failed to parse update: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, nil
	}

	// Only handle text messages for now
	if update.Message == nil || update.Message.Text == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: `{"ok":true}`}, nil
	}

	msg := update.Message
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	senderID := fmt.Sprintf("%d|%s", msg.From.ID, msg.From.UserName)

	if !isAllowed(senderID) {
		log.Printf("Rejected message from %s", senderID)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: `{"ok":true}`}, nil
	}

	log.Printf("Processing message from %s: %s", senderID, msg.Text)

	// Process synchronously through the agent
	sessionKey := fmt.Sprintf("telegram:%s", chatID)
	response, err := agentLoop.ProcessDirectWithChannel(ctx, msg.Text, sessionKey, "telegram", chatID)
	if err != nil {
		log.Printf("Agent error: %v", err)
		response = "Sorry, something went wrong processing your message."
	}

	// Send reply via Telegram API
	if response != "" {
		reply := tgbotapi.NewMessage(msg.Chat.ID, response)
		reply.ParseMode = tgbotapi.ModeHTML
		if _, err := bot.Send(reply); err != nil {
			// Retry without HTML parsing
			reply.ParseMode = ""
			if _, retryErr := bot.Send(reply); retryErr != nil {
				log.Printf("Failed to send reply: %v", retryErr)
			}
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"ok":true}`,
	}, nil
}

func main() {
	lambda.Start(handler)
}
