package bot

import (
	"context"
	"digitalocean-tgbot/internal/db"
	"digitalocean-tgbot/internal/do"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	Bot *tgbotapi.BotAPI
	DB  *db.DB
}

func NewHandler(bot *tgbotapi.BotAPI, database *db.DB) *Handler {
	return &Handler{Bot: bot, DB: database}
}

func (h *Handler) HandleCommand(m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		h.sendMainMenu(m.Chat.ID, "欢迎使用 DigitalOcean 管理机器人。\n请使用下方菜单进行操作：")
	}
}

func (h *Handler) sendMainMenu(chatID int64, text string) {
	markup := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("➕ 添加账号"),
			tgbotapi.NewKeyboardButton("📋 账号列表"),
			tgbotapi.NewKeyboardButton("🚀 创建实例"),
		),
	)
	markup.ResizeKeyboard = true
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

type CreateState struct {
	AccountID  int64
	Region     string
	Size       string
	Image      string
	Name       string
}

var userStates = make(map[int64]*CreateState)

func (h *Handler) addAccountStep1(m *tgbotapi.Message) {
	h.sendText(m.Chat.ID, "请发送 DO 账号 Token:")
}

func (h *Handler) listAccounts(m *tgbotapi.Message) {
	accounts, _ := h.DB.ListAccounts()
	if len(accounts) == 0 {
		h.sendText(m.Chat.ID, "暂无账号")
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>账号列表:</b>\n\n")
	markup := tgbotapi.NewInlineKeyboardMarkup()

	for _, acc := range accounts {
		sb.WriteString(fmt.Sprintf("- %s\n", acc.Email))
		btn := tgbotapi.NewInlineKeyboardButtonData(acc.Email, fmt.Sprintf("acc_info:%d", acc.ID))
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

func (h *Handler) ProcessMessage(m *tgbotapi.Message) {
	if m.IsCommand() {
		return
	}

	// 处理键盘按钮
	switch m.Text {
	case "➕ 添加账号":
		h.addAccountStep1(m)
		return
	case "📋 账号列表":
		h.listAccounts(m)
		return
	case "🚀 创建实例":
		h.createDropletStep1(m)
		return
	}

	state, exists := userStates[m.From.ID]
	if exists && state.Image != "" && state.Name == "" {
		state.Name = m.Text
		h.confirmCreate(m.From.ID, m.Chat.ID)
		return
	}

	token := strings.TrimSpace(m.Text)
	if len(token) > 20 {
		h.saveAccount(m, token)
	}
}

func (h *Handler) confirmCreate(userID int64, chatID int64) {
	state := userStates[userID]
	acc, _ := h.DB.GetAccount(state.AccountID)

	text := fmt.Sprintf("<b>确认创建</b>\n\n账号: %s\n地区: %s\n配置: %s\n镜像: %s\n名称: %s",
		acc.Email, state.Region, state.Size, state.Image, state.Name)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("确认创建", "cr_confirm"),
			tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

func (h *Handler) saveAccount(m *tgbotapi.Message, token string) {
	client := do.NewClient(token)
	acc, err := client.GetAccount(context.Background())
	if err != nil {
		h.sendText(m.Chat.ID, "Token 验证失败")
		return
	}

	err = h.DB.AddAccount(acc.Email, token)
	if err != nil {
		h.sendText(m.Chat.ID, "账号保存失败 (可能已存在)")
		return
	}
	h.sendText(m.Chat.ID, fmt.Sprintf("账号添加成功: %s", acc.Email))
}

func (h *Handler) HandleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	parts := strings.Split(data, ":")
	cmd := parts[0]

	// 回调反馈
	h.Bot.Request(tgbotapi.NewCallback(query.ID, ""))

	switch cmd {
	case "acc_info":
		id, _ := strconv.ParseInt(parts[1], 10, 64)
		h.showAccountInfo(query, id)
	case "acc_del":
		id, _ := strconv.ParseInt(parts[1], 10, 64)
		h.deleteAccount(query, id)
	case "cr_acc":
		id, _ := strconv.ParseInt(parts[1], 10, 64)
		h.createDropletStep2(query, id)
	case "cr_reg":
		h.createDropletStep3(query, parts[1])
	case "cr_size":
		h.createDropletStep4(query, parts[1])
	case "cr_img":
		h.createDropletStep5(query, parts[1])
	case "cr_back":
		h.handleCreateBack(query, parts[1])
	case "cr_confirm":
		h.executeCreate(query)
	case "cr_cancel":
		delete(userStates, query.From.ID)
		h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "❌ 已取消创建"))
	}
}

func (h *Handler) handleCreateBack(query *tgbotapi.CallbackQuery, target string) {
	switch target {
	case "step1":
		h.createDropletStep1Edit(query)
	case "step2":
		state := userStates[query.From.ID]
		h.createDropletStep2(query, state.AccountID)
	case "step3":
		state := userStates[query.From.ID]
		h.createDropletStep3(query, state.Region)
	case "step4":
		state := userStates[query.From.ID]
		h.createDropletStep4(query, state.Size)
	}
}

func (h *Handler) executeCreate(query *tgbotapi.CallbackQuery) {
	state := userStates[query.From.ID]
	acc, _ := h.DB.GetAccount(state.AccountID)
	client := do.NewClient(acc.Token)

	password := h.generatePassword(12)
	userData := fmt.Sprintf("#!/bin/bash\necho root:%s | chpasswd", password)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "🚀 正在创建实例，请稍候..."))

	droplet, err := client.CreateDroplet(context.Background(), state.Name, state.Region, state.Size, state.Image, userData)
	if err != nil {
		h.sendText(query.From.ID, "创建失败: "+err.Error())
		return
	}

	delete(userStates, query.From.ID)

	// 异步轮询状态
	go func(dID int, p string) {
		for {
			time.Sleep(5 * time.Second)
			d, err := client.GetDroplet(context.Background(), dID)
			if err != nil {
				break
			}
			if d.Status == "active" {
				var ip string
				for _, net := range d.Networks.V4 {
					if net.Type == "public" {
						ip = net.IPAddress
						break
					}
				}
				msg := fmt.Sprintf("✅ <b>实例创建完成</b>\n\n名称: <code>%s</code>\nIP: <code>%s</code>\n密码: <code>%s</code>", d.Name, ip, p)
				res := tgbotapi.NewMessage(query.From.ID, msg)
				res.ParseMode = "HTML"
				h.Bot.Send(res)
				break
			}
		}
	}(droplet.ID, password)
}

func (h *Handler) generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (h *Handler) createDropletStep1(m *tgbotapi.Message) {
	accounts, _ := h.DB.ListAccounts()
	if len(accounts) == 0 {
		h.sendText(m.Chat.ID, "请先添加账号")
		return
	}

	markup := tgbotapi.NewInlineKeyboardMarkup()
	for _, acc := range accounts {
		btn := tgbotapi.NewInlineKeyboardButtonData(acc.Email, fmt.Sprintf("cr_acc:%d", acc.ID))
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, "<b>创建实例</b>\n请选择账号:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

func (h *Handler) createDropletStep1Edit(query *tgbotapi.CallbackQuery) {
	accounts, _ := h.DB.ListAccounts()
	markup := tgbotapi.NewInlineKeyboardMarkup()
	for _, acc := range accounts {
		btn := tgbotapi.NewInlineKeyboardButtonData(acc.Email, fmt.Sprintf("cr_acc:%d", acc.ID))
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
	}

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请选择账号:")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) createDropletStep2(query *tgbotapi.CallbackQuery, accID int64) {
	userStates[query.From.ID] = &CreateState{AccountID: accID}
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	regions, _ := client.ListRegions(context.Background())
	markup := tgbotapi.NewInlineKeyboardMarkup()
	var row []tgbotapi.InlineKeyboardButton
	for i, reg := range regions {
		if !reg.Available {
			continue
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(reg.Slug, fmt.Sprintf("cr_reg:%s", reg.Slug))
		row = append(row, btn)
		if len(row) == 2 || i == len(regions)-1 {
			markup.InlineKeyboard = append(markup.InlineKeyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 上一步", "cr_back:step1"),
		tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "cr_cancel"),
	))

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请选择地区:")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) createDropletStep3(query *tgbotapi.CallbackQuery, region string) {
	state := userStates[query.From.ID]
	state.Region = region

	acc, _ := h.DB.GetAccount(state.AccountID)
	client := do.NewClient(acc.Token)
	sizes, _ := client.ListSizes(context.Background())

	markup := tgbotapi.NewInlineKeyboardMarkup()
	var row []tgbotapi.InlineKeyboardButton
	count := 0
	for _, s := range sizes {
		found := false
		for _, r := range s.Regions {
			if r == region {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(s.Slug, fmt.Sprintf("cr_size:%s", s.Slug))
		row = append(row, btn)
		count++
		if len(row) == 2 {
			markup.InlineKeyboard = append(markup.InlineKeyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
		if count > 10 {
			break
		}
	}
	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 上一步", "cr_back:step2"),
		tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "cr_cancel"),
	))

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请选择配置:")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) createDropletStep4(query *tgbotapi.CallbackQuery, size string) {
	state := userStates[query.From.ID]
	state.Size = size

	acc, _ := h.DB.GetAccount(state.AccountID)
	client := do.NewClient(acc.Token)
	images, _ := client.ListDistributions(context.Background())

	markup := tgbotapi.NewInlineKeyboardMarkup()
	for _, img := range images {
		if img.Distribution != "Ubuntu" && img.Distribution != "CentOS" && img.Distribution != "Debian" {
			continue
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %s", img.Distribution, img.Name), fmt.Sprintf("cr_img:%s", img.Slug))
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
	}
	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 上一步", "cr_back:step3"),
		tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "cr_cancel"),
	))

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请选择镜像:")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) createDropletStep5(query *tgbotapi.CallbackQuery, image string) {
	state := userStates[query.From.ID]
	state.Image = image

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 上一步", "cr_back:step4"),
			tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "cr_cancel"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请输入实例名称（直接回复消息）：")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) showAccountInfo(query *tgbotapi.CallbackQuery, id int64) {
	acc, err := h.DB.GetAccount(id)
	if err != nil {
		return
	}

	text := fmt.Sprintf("<b>账号详情</b>\n\n邮箱: <code>%s</code>", acc.Email)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("删除账号", fmt.Sprintf("acc_del:%d", id)),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) deleteAccount(query *tgbotapi.CallbackQuery, id int64) {
	h.DB.DeleteAccount(id)
	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "账号已删除")
	h.Bot.Send(edit)
}

func (h *Handler) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	h.Bot.Send(msg)
}
