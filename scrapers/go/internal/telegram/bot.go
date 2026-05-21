package telegram

import (
	"context"
	"fmt"
	"go-version/internal/database"
	"go-version/internal/scraper"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	fallbackChatID int64 // Used when DB is unavailable
}

func NewBot(token string, fallbackChatID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		api:            api,
		fallbackChatID: fallbackChatID,
	}, nil
}

// PollAndRegisterSubscribers fetches pending Telegram updates and registers any new
// users who sent any message (including /start) to the bot. After processing, it
// advances the update offset so the same updates are not re-processed on the next run.
// This is safe to call on every scraper run — existing users are silently ignored (ON CONFLICT DO NOTHING).
func (b *Bot) PollAndRegisterSubscribers(ctx context.Context, repo *database.Repository) {
	if repo == nil {
		log.Println("⚠️ Telegram polling skipped: no DB connection")
		return
	}

	log.Println("🔄 Polling Telegram for new subscribers...")

	updates, err := b.api.GetUpdates(tgbotapi.UpdateConfig{
		Offset:  0,
		Limit:   100,
		Timeout: 0,
	})
	if err != nil {
		log.Printf("⚠️ Telegram getUpdates failed: %v", err)
		return
	}

	if len(updates) == 0 {
		log.Println("ℹ️ No new Telegram updates.")
		return
	}

	var maxUpdateID int
	newUsers := 0
	for _, update := range updates {
		if update.UpdateID > maxUpdateID {
			maxUpdateID = update.UpdateID
		}

		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		username := update.Message.From.UserName
		if username == "" {
			username = update.Message.From.FirstName
		}

		if err := repo.RegisterSubscriber(ctx, chatID, username); err != nil {
			log.Printf("⚠️ Failed to register subscriber %d (%s): %v", chatID, username, err)
		} else {
			log.Printf("👤 Registered subscriber: %s (chat_id=%d)", username, chatID)
			newUsers++
		}
	}

	log.Printf("✅ Polling done. Processed %d updates, registered/verified %d users.", len(updates), newUsers)

	// Advance offset so next poll doesn't re-process these updates
	if maxUpdateID > 0 {
		_, _ = b.api.GetUpdates(tgbotapi.UpdateConfig{
			Offset:  maxUpdateID + 1,
			Limit:   1,
			Timeout: 0,
		})
	}
}

// BroadcastJob sends a job notification to ALL registered subscribers.
// If DB is not available, it falls back to the single fallbackChatID.
func (b *Bot) BroadcastJob(ctx context.Context, repo *database.Repository, job scraper.Job, jobID string) {
	var chatIDs []int64

	if repo != nil {
		ids, err := repo.GetAllSubscriberChatIDs(ctx)
		if err != nil {
			log.Printf("⚠️ Failed to fetch subscriber list, falling back to owner: %v", err)
		} else {
			chatIDs = ids
		}
	}

	// Fallback: if DB unavailable or no subscribers, send to owner only
	if len(chatIDs) == 0 {
		chatIDs = []int64{b.fallbackChatID}
	}

	log.Printf("📨 Broadcasting job '%s' to %d subscribers...", job.Title, len(chatIDs))
	for _, id := range chatIDs {
		if err := b.sendJob(id, job, jobID); err != nil {
			log.Printf("⚠️ Failed to send job to chat %d: %v", id, err)
		}
		time.Sleep(200 * time.Millisecond) // avoid Telegram flood limits
	}
}

// sendJob sends a single job notification to a specific chatID.
func (b *Bot) sendJob(chatID int64, job scraper.Job, jobID string) error {
	msgText := fmt.Sprintf("🏢 *%s*\n", b.escapeMarkdown(job.Company))
	msgText += fmt.Sprintf("🔗 [View Job](%s)\n", job.URL)
	if job.Salary != "" {
		msgText += fmt.Sprintf("💰 %s\n", b.escapeMarkdown(job.Salary))
	}

	tech := job.Techstack
	if tech == "" {
		tech = "N/A"
	}
	msgText += fmt.Sprintf("📝 %s\n", b.escapeMarkdown(tech))

	loc := job.Location
	if loc == "" {
		loc = "N/A"
	}
	msgText += fmt.Sprintf("📍 %s\n", b.escapeMarkdown(loc))

	if job.PostedDate != "" {
		msgText += fmt.Sprintf("📅 %s\n", b.escapeMarkdown(job.PostedDate))
	}

	if (job.Source == "Facebook" || job.Source == "LinkedIn (Post)") && job.Description != "" {
		desc := job.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		msgText += fmt.Sprintf("📄 %s\n", b.escapeMarkdown(desc))
	}

	msgText += fmt.Sprintf("🤖 Match Score: %d/10\n", job.MatchScore)
	msgText += fmt.Sprintf("🔖 Source: %s\n", b.escapeMarkdown(job.Source))

	var refineCVBtn tgbotapi.InlineKeyboardButton
	if jobID != "" {
		refineCVBtn = tgbotapi.NewInlineKeyboardButtonData("🛠️ Refine CV", "refine_cv:"+jobID)
	} else {
		refineCVBtn = tgbotapi.NewInlineKeyboardButtonURL("🛠️ View Job", job.URL)
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			refineCVBtn, tgbotapi.NewInlineKeyboardButtonURL("🔗 View Job", job.URL),
		),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "MarkdownV2"
	msg.ReplyMarkup = keyboard

	_, err := b.api.Send(msg)
	return err
}

// SendError sends an error message to the owner's chat only.
func (b *Bot) SendError(err error) error {
	msg := tgbotapi.NewMessage(b.fallbackChatID, fmt.Sprintf("❌ Error: %v", err))
	_, sendErr := b.api.Send(msg)
	return sendErr
}

// SendStatus sends a status message to the owner's chat only.
func (b *Bot) SendStatus(message string) error {
	msg := tgbotapi.NewMessage(b.fallbackChatID, "ℹ️ "+message)
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "(", "\\(",
		")", "\\)", "~", "\\~", "`", "\\`", ">", "\\>", "#", "\\#",
		"+", "\\+", "-", "\\-", "=", "\\=", "|", "\\|", "{", "\\{",
		"}", "\\}", ".", "\\.", "!", "\\!",
	)
	return replacer.Replace(text)
}

// SendMarkdownToOwner sends a message with MarkdownV2 formatting to the fallback owner chat ID.
func (b *Bot) SendMarkdownToOwner(message string) error {
msg := tgbotapi.NewMessage(b.fallbackChatID, message)
msg.ParseMode = "MarkdownV2"
_, err := b.api.Send(msg)
return err
}

// SendPhotoToOwner sends a photo and caption to the owner chat ID.
func (b *Bot) SendPhotoToOwner(photoPath string, caption string) error {
photo := tgbotapi.NewPhoto(b.fallbackChatID, tgbotapi.FilePath(photoPath))
photo.Caption = caption
_, err := b.api.Send(photo)
return err
}
