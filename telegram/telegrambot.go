package telegram

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	tgBot "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/thank243/StairUnlocker-Bot/config"
	"github.com/thank243/StairUnlocker-Bot/user"
	"github.com/thank243/StairUnlocker-Bot/utils"
)

func Updates(buf *chan *user.User, userMap *map[int64]*user.User) (err error) {
	start := time.Now()
	bot, err := tgBot.NewBotAPI(config.BotCfg.TelegramToken)
	if err != nil {
		panic(err)
	}
	if config.BotCfg.LogLevel == 0 {
		bot.Debug = true
	}
	log.Infoln("Authorized on account %s", bot.Self.UserName)
	// initial command list
	preCommands, _ := bot.GetMyCommands()
	currCommands := []tgBot.BotCommand{
		{"url", "获取节点解锁状态。"},
		{"ip", "获取真实 IP 信息。"},
		{"stat", "显示最新的检查结果。"},
		{"version", "显示版本。"},
	}
	if fmt.Sprint(preCommands) != fmt.Sprint(currCommands) {
		_, err = bot.Request(tgBot.SetMyCommandsConfig{Commands: currCommands})
		if err != nil {
			log.Errorln(err.Error())
		}
	}

	updateCfg := tgBot.NewUpdate(0)
	updateCfg.Timeout = 60
	updates := bot.GetUpdatesChan(updateCfg)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		// If user is not exist, initial User struct
		usr, exist := (*userMap)[update.Message.Chat.ID]
		if !exist {
			(*userMap)[update.Message.Chat.ID] = &user.User{
				ID:  update.Message.Chat.ID,
				Bot: bot,
			}
			usr = (*userMap)[update.Message.Chat.ID]
		}
		// delete user privacy info
		_ = usr.DeleteMessage(update.Message.MessageID)

		// select telegram cmd.
		switch {
		case update.Message.Text == "/start":
			cmdList, _ := bot.GetMyCommands()
			var str string
			for i := range cmdList {
				str += fmt.Sprintf("/%s - %s\n", cmdList[i].Command, cmdList[i].Description)
			}
			str += "使用后，机器人将使用最新的子 URL 进行测试。"
			_, _ = usr.SendMessage(str, false)

		case update.Message.Text == "/stat":
			if usr.Data.CheckInfo == "" {
				_, _ = usr.SendMessage("找不到状态信息。 请使用[/url subURL]命令一次。", false)
			} else {
				_, _ = usr.SendMessage((*userMap)[usr.ID].Data.CheckInfo, false)
			}

		case strings.HasPrefix(update.Message.Text, "/url") || strings.HasPrefix(update.Message.Text, "/ip"):
			if len(*buf) > config.BotCfg.MaxOnline {
				_, _ = usr.SendMessage("连接太多，请稍后再试.", false)
				continue
			}
			// forbid double-checking
			if usr.IsCheck {
				_, _ = usr.SendMessage("之前的测试没有完成！ 请稍后再试.", false)
				continue
			}

			if strings.HasPrefix(update.Message.Text, "/url") {
				var subURL *url.URL
				trimStr := strings.TrimSpace(strings.ReplaceAll(update.Message.Text, "/url", ""))
				subURL, err = url.Parse(trimStr)
				if err != nil || (usr.Data.SubURL == "" && trimStr == "") || (subURL.Scheme == "" && trimStr != "") {
					_, _ = usr.SendMessage("无效的网址。 请检查您的 subURL 或使用[/url subURL]命令一次。", false)
				} else if usr.UserOutInternal(config.BotCfg.Internal) {
					if trimStr != "" {
						usr.Data.SubURL = subURL.String()
					}
					*buf <- usr
				}
			} else {
				if usr.UserOutInternal(config.BotCfg.Internal) {
					go usr.RealIP(update.Message.Text)
				}
			}

		case update.Message.Text == "/version":
			todayUser := 0
			for _, v := range *userMap {
				if time.Now().Unix()-v.Data.LastCheck < int64(24*time.Hour.Seconds()) {
					todayUser++
				}
			}
			uptime := utils.FormatTime(time.Since(start))
			_, _ = usr.SendMessage(fmt.Sprintf("StairUnlocker Bot %s\nUsers: (%d/%d) \nUptime: %s", C.Version, todayUser, len(*userMap), uptime), false)
			_, _ = usr.SendMessage("无效命令", false)
		}
		log.Debugln("Telegram Bot: [ID: %d], Text: %s", update.Message.From.ID, update.Message.Text)
	}
	return
}
