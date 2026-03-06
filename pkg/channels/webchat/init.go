package webchat

import (
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
)

func init() {
	channels.RegisterFactory("webchat", func(cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
		return NewWebChatChannel(cfg.Channels.WebChat, b)
	})
}
