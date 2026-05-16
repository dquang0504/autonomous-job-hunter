package reporter

import (
	"fmt"
	"go-version/internal/config"
	"go-version/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramReporter struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// NewTelegramReporter creates a new instance of the Telegram reporter using the provided configuration.
func NewTelegramReporter(cfg *config.Config) (*TelegramReporter, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to init telegram bot: %w", err)
	}

	//turn this on in case of debug
	//bot.Debug = true

	return &TelegramReporter{
		bot:    bot,
		chatID: cfg.TelegramChatID,
	}, nil
}

func (t *TelegramReporter) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(t.chatID, text)
	msg.ParseMode = "HTML" //use HTML for bold/italic
	_, err := t.bot.Send(msg)
	return err
}

func (t *TelegramReporter) SendJob(job scraper.Job) error {
	text := fmt.Sprintf(
		"🔥 <b>%s</b>\n"+
			"🏢 %s\n"+
			"💰 %s\n"+
			"📍 %s\n"+
			"🛠 %s\n"+
			"🔗 <a href=\"%s\">Apply Now</a>",
		job.Title,
		job.Company,
		job.Salary,
		job.Location,
		job.Techstack,
		job.URL,
	)
	return t.SendMessage(text)
}

func (t *TelegramReporter) SendError(errReq error) error {
	text := fmt.Sprintf("⚠️ <b>OpenClaw Error</b>:\n%v", errReq)
	return t.SendMessage(text)
}
