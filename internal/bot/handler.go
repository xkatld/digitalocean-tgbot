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
			tgbotapi.NewKeyboardButton("添加账号"),
			tgbotapi.NewKeyboardButton("账号列表"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("创建实例"),
			tgbotapi.NewKeyboardButton("实例列表"),
		),
	)
	markup.ResizeKeyboard = true
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

type CreateState struct {
	AccountID int64
	Region    string
	Size      string
	Image     string
	Name      string
	Count     int
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

	switch m.Text {
	case "添加账号":
		h.addAccountStep1(m)
		return
	case "账号列表":
		h.listAccounts(m)
		return
	case "创建实例":
		h.createDropletStep1(m)
		return
	case "实例列表":
		h.listDropletsSelector(m)
		return
	}

	state, exists := userStates[m.From.ID]
	if exists {
		if state.Image != "" && state.Name == "" {
			state.Name = m.Text
			h.createDropletStep6(m)
			return
		}
		if state.Name != "" && state.Count == 0 {
			count, err := strconv.Atoi(m.Text)
			if err != nil || count < 1 || count > 10 {
				h.sendText(m.Chat.ID, "请输入有效的数字 (1-10)：")
				return
			}
			state.Count = count
			h.confirmCreate(m.From.ID, m.Chat.ID)
			return
		}
	}

	token := strings.TrimSpace(m.Text)
	if len(token) > 20 {
		h.saveAccount(m, token)
	}
}

func (h *Handler) confirmCreate(userID int64, chatID int64) {
	state := userStates[userID]
	acc, _ := h.DB.GetAccount(state.AccountID)

	text := fmt.Sprintf("<b>确认创建</b>\n\n账号: %s\n地区: %s\n配置: %s\n镜像: %s\n名称: %s\n数量: %d",
		acc.Email, state.Region, state.Size, state.Image, state.Name, state.Count)

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

	if cmd != "dr_pass" {
		h.Bot.Request(tgbotapi.NewCallback(query.ID, ""))
	}

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
		h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[注意] 已取消创建"))
	case "dr_list":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.listDroplets(query, accID)
	case "dr_info":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.showDropletInfo(query, accID, drID)
	case "dr_pass":
		drID, _ := strconv.Atoi(parts[1])
		h.showDropletPassword(query, drID)
	case "dr_del":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.confirmDeleteDroplet(query, accID, drID)
	case "dr_del_conf":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		action := parts[3]
		var ipToDelete string
		if len(parts) > 5 && parts[4] == "del_ip" {
			action = "del_ip"
			ipToDelete = parts[5]
		}
		h.executeDeleteDroplet(query, accID, drID, action, ipToDelete)
	case "dr_rip":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.listReservedIPs(query, accID, drID)
	case "rip_assign":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.executeAssignIP(query, accID, drID, parts[3])
	case "rip_create":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.executeCreateIP(query, accID, drID, parts[3])
	case "rip_unassign":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.executeUnassignIP(query, accID, drID, parts[3])
	case "rip_del":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.confirmDeleteIP(query, accID, drID, parts[3])
	case "rip_del_conf":
		accID, _ := strconv.ParseInt(parts[1], 10, 64)
		drID, _ := strconv.Atoi(parts[2])
		h.executeDeleteIP(query, accID, drID, parts[3])
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
	case "step5":
		state := userStates[query.From.ID]
		h.createDropletStep5(query, state.Image)
	}
}

func (h *Handler) executeCreate(query *tgbotapi.CallbackQuery) {
	state := userStates[query.From.ID]
	acc, _ := h.DB.GetAccount(state.AccountID)
	client := do.NewClient(acc.Token)

	count := state.Count
	baseName := state.Name
	region := state.Region
	size := state.Size
	image := state.Image

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, fmt.Sprintf("[注意] 正在创建 %d 个实例，请稍候...", count)))
	delete(userStates, query.From.ID)

	go func() {
		for i := 1; i <= count; i++ {
			name := baseName
			if count > 1 {
				name = fmt.Sprintf("%s-%d", baseName, i)
			}
			password := h.generatePassword(16)
			userData := fmt.Sprintf("#!/bin/bash\necho root:%s | chpasswd", password)

			droplet, err := client.CreateDroplet(context.Background(), name, region, size, image, userData)
			if err != nil {
				h.sendText(query.From.ID, fmt.Sprintf("创建失败 (%s): %v", name, err))
				continue
			}

			h.DB.SaveDroplet(droplet.ID, acc.ID, name, password, "", "new")

			go func(dID int, p, n string) {
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

						h.DB.SaveDroplet(dID, acc.ID, n, p, ip, "active")

						msg := fmt.Sprintf("[正确] <b>实例创建完成</b>\n\n名称: <code>%s</code>\nIP: <code>%s</code>\n密码: <code>%s</code>", n, ip, p)
						res := tgbotapi.NewMessage(query.From.ID, msg)
						res.ParseMode = "HTML"
						h.Bot.Send(res)
						break
					}
				}
			}(droplet.ID, password, name)

			if i < count {
				time.Sleep(2 * time.Second)
			}
		}
	}()
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
		tgbotapi.NewInlineKeyboardButtonData("返回 上一步", "cr_back:step1"),
		tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
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
		if len(row) == 2 {
			markup.InlineKeyboard = append(markup.InlineKeyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		markup.InlineKeyboard = append(markup.InlineKeyboard, row)
	}
	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("返回 上一步", "cr_back:step2"),
		tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
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
		tgbotapi.NewInlineKeyboardButtonData("返回 上一步", "cr_back:step3"),
		tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
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
			tgbotapi.NewInlineKeyboardButtonData("返回 上一步", "cr_back:step4"),
			tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "<b>创建实例</b>\n请输入实例名称（直接回复消息）：")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) createDropletStep6(m *tgbotapi.Message) {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回 上一步", "cr_back:step5"),
			tgbotapi.NewInlineKeyboardButtonData("取消", "cr_cancel"),
		),
	)
	msg := tgbotapi.NewMessage(m.Chat.ID, "<b>创建实例</b>\n请输入批量创建数量 (1-10)：")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

func (h *Handler) showAccountInfo(query *tgbotapi.CallbackQuery, id int64) {
	acc, err := h.DB.GetAccount(id)
	if err != nil {
		return
	}

	text := fmt.Sprintf("<b>账号详情</b>\n\n邮箱: <code>%s</code>", acc.Email)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("管理实例", fmt.Sprintf("dr_list:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("删除账号", fmt.Sprintf("acc_del:%d", id)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回 账号列表", "cr_back:step1"),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) listDropletsSelector(m *tgbotapi.Message) {
	accounts, _ := h.DB.ListAccounts()
	if len(accounts) == 0 {
		h.sendText(m.Chat.ID, "请先添加账号")
		return
	}
	markup := tgbotapi.NewInlineKeyboardMarkup()
	for _, acc := range accounts {
		btn := tgbotapi.NewInlineKeyboardButtonData(acc.Email, fmt.Sprintf("dr_list:%d", acc.ID))
		markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
	}
	msg := tgbotapi.NewMessage(m.Chat.ID, "<b>管理实例</b>\n请选择账号:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	h.Bot.Send(msg)
}

func (h *Handler) listDroplets(query *tgbotapi.CallbackQuery, accID int64) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)
	list, err := client.ListDroplets(context.Background())
	if err != nil {
		h.sendText(query.From.ID, "获取实例列表失败: "+err.Error())
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>实例列表 (%s):</b>\n\n", acc.Email))
	markup := tgbotapi.NewInlineKeyboardMarkup()

	if len(list) == 0 {
		sb.WriteString("暂无活跃实例")
	} else {
		for _, dr := range list {
			btn := tgbotapi.NewInlineKeyboardButtonData(dr.Name, fmt.Sprintf("dr_info:%d:%d", accID, dr.ID))
			markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(btn))
		}
	}

	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("返回 账号详情", fmt.Sprintf("acc_info:%d", accID)),
	))

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, sb.String())
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) showDropletInfo(query *tgbotapi.CallbackQuery, accID int64, drID int) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)
	dr, err := client.GetDroplet(context.Background(), drID)
	if err != nil {
		h.sendText(query.From.ID, "获取实例详情失败: "+err.Error())
		return
	}

	var ip string
	for _, net := range dr.Networks.V4 {
		if net.Type == "public" {
			ip = net.IPAddress
			break
		}
	}

	var reservedIP string
	ips, err := client.ListReservedIPs(context.Background())
	if err == nil {
		for _, rip := range ips {
			if rip.Droplet != nil && rip.Droplet.ID == drID {
				reservedIP = rip.IP
				break
			}
		}
	}

	text := fmt.Sprintf("<b>实例详情</b>\n\n名称: <code>%s</code>\n主 IP: <code>%s</code>", dr.Name, ip)
	if reservedIP != "" {
		text += fmt.Sprintf("\n附加 IP: <code>%s</code> (Reserved)", reservedIP)
	}

	if localDr, err := h.DB.GetDroplet(drID); err == nil && localDr.Password != "" {
		text += fmt.Sprintf("\n密码: <code>%s</code>", localDr.Password)
	}

	text += fmt.Sprintf("\n地区: %s\n配置: %s\n状态: %s", dr.Region.Slug, dr.SizeSlug, dr.Status)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("查看密码", fmt.Sprintf("dr_pass:%d", drID)),
			tgbotapi.NewInlineKeyboardButtonData("更换 IP", fmt.Sprintf("dr_rip:%d:%d", accID, drID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("删除实例", fmt.Sprintf("dr_del:%d:%d", accID, drID)),
		),

		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回 实例列表", fmt.Sprintf("dr_list:%d", accID)),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) listReservedIPs(query *tgbotapi.CallbackQuery, accID int64, drID int) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)
	
	dr, err := client.GetDroplet(context.Background(), drID)
	if err != nil {
		h.sendText(query.From.ID, "获取实例信息失败: "+err.Error())
		return
	}

	ips, err := client.ListReservedIPs(context.Background())
	if err != nil {
		h.sendText(query.From.ID, "获取保留 IP 列表失败: "+err.Error())
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>更换附加 IP (Region: %s)</b>\n\n当前实例: %s\n", dr.Region.Slug, dr.Name))

	markup := tgbotapi.NewInlineKeyboardMarkup()

	for _, ip := range ips {
		if ip.Region.Slug != dr.Region.Slug {
			continue
		}

		var btnText, btnData string
		if ip.Droplet != nil {
			if ip.Droplet.ID == drID {
				btnText = fmt.Sprintf("[正确] %s (本机已绑定)", ip.IP)
				btnData = fmt.Sprintf("rip_unassign:%d:%d:%s", accID, drID, ip.IP)
				markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(btnText, "ignore"),
					tgbotapi.NewInlineKeyboardButtonData("[解绑]", btnData),
				))
			} else {
				btnText = fmt.Sprintf("[错误] %s (占: %s)", ip.IP, ip.Droplet.Name)
				btnData = fmt.Sprintf("rip_assign:%d:%d:%s", accID, drID, ip.IP)
				markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(btnText, btnData),
					tgbotapi.NewInlineKeyboardButtonData("[抢占]", btnData),
				))
			}
		} else {
			btnText = fmt.Sprintf("[注意] %s (空闲)", ip.IP)
			btnData = fmt.Sprintf("rip_assign:%d:%d:%s", accID, drID, ip.IP)
			markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(btnText, btnData),
				tgbotapi.NewInlineKeyboardButtonData("[绑定]", btnData),
				tgbotapi.NewInlineKeyboardButtonData("[删除]", fmt.Sprintf("rip_del:%d:%d:%s", accID, drID, ip.IP)),
			))
		}
	}

	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("申请新 IP 并绑定", fmt.Sprintf("rip_create:%d:%d:%s", accID, drID, dr.Region.Slug)),
	))

	markup.InlineKeyboard = append(markup.InlineKeyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("返回 实例详情", fmt.Sprintf("dr_info:%d:%d", accID, drID)),
	))

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, sb.String())
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) executeAssignIP(query *tgbotapi.CallbackQuery, accID int64, drID int, ip string) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, fmt.Sprintf("[注意] 正在绑定 IP %s...", ip)))

	err := client.AssignReservedIP(context.Background(), ip, drID)
	if err != nil {
		h.sendText(query.From.ID, "绑定失败: "+err.Error())
		time.Sleep(2 * time.Second)
		h.listReservedIPs(query, accID, drID)
		return
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[正确] IP 绑定成功！"))
	time.Sleep(1 * time.Second)
	h.listReservedIPs(query, accID, drID)
}

func (h *Handler) executeCreateIP(query *tgbotapi.CallbackQuery, accID int64, drID int, region string) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[注意] 正在申请新 IP..."))

	resIP, err := client.CreateReservedIP(context.Background(), region)
	if err != nil {
		h.sendText(query.From.ID, "申请 IP 失败: "+err.Error())
		time.Sleep(2 * time.Second)
		h.listReservedIPs(query, accID, drID)
		return
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, fmt.Sprintf("[注意] IP %s 申请成功，正在绑定...", resIP.IP)))

	err = client.AssignReservedIP(context.Background(), resIP.IP, drID)
	if err != nil {
		h.sendText(query.From.ID, fmt.Sprintf("绑定失败 (%s): %v", resIP.IP, err))
		time.Sleep(2 * time.Second)
		h.listReservedIPs(query, accID, drID)
		return
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[正确] 新 IP 绑定成功！"))
	time.Sleep(1 * time.Second)
	h.listReservedIPs(query, accID, drID)
}

func (h *Handler) executeUnassignIP(query *tgbotapi.CallbackQuery, accID int64, drID int, ip string) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, fmt.Sprintf("[注意] 正在解绑 IP %s...", ip)))

	err := client.UnassignReservedIP(context.Background(), ip)
	if err != nil {
		h.sendText(query.From.ID, "解绑失败: "+err.Error())
		time.Sleep(2 * time.Second)
		h.listReservedIPs(query, accID, drID)
		return
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[正确] IP 解绑成功！"))
	time.Sleep(1 * time.Second)
	h.listReservedIPs(query, accID, drID)
}

func (h *Handler) confirmDeleteIP(query *tgbotapi.CallbackQuery, accID int64, drID int, ip string) {
	text := fmt.Sprintf("<b>[注意] 确认删除 IP？</b>\n\nIP: <code>%s</code>\n此操作将永久释放该资源，不可恢复。", ip)
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("[注意] 确认删除 IP", fmt.Sprintf("rip_del_conf:%d:%d:%s", accID, drID, ip)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("返回", fmt.Sprintf("dr_rip:%d:%d", accID, drID)),
		),
	)

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) executeDeleteIP(query *tgbotapi.CallbackQuery, accID int64, drID int, ip string) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, fmt.Sprintf("[注意] 正在删除 IP %s...", ip)))

	err := client.DeleteReservedIP(context.Background(), ip)
	if err != nil {
		h.sendText(query.From.ID, "删除失败: "+err.Error())
		time.Sleep(2 * time.Second)
		h.listReservedIPs(query, accID, drID)
		return
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[正确] IP 已成功释放！"))
	time.Sleep(1 * time.Second)
	h.listReservedIPs(query, accID, drID)
}

func (h *Handler) showDropletPassword(query *tgbotapi.CallbackQuery, drID int) {
	dr, err := h.DB.GetDroplet(drID)
	if err != nil || dr.Password == "" {
		cb := tgbotapi.NewCallback(query.ID, "密码未找到或非本机创建")
		h.Bot.Request(cb)
		return
	}

	cb := tgbotapi.NewCallback(query.ID, fmt.Sprintf("Root Pwd: %s", dr.Password))
	cb.ShowAlert = true
	h.Bot.Request(cb)
}

func (h *Handler) confirmDeleteDroplet(query *tgbotapi.CallbackQuery, accID int64, drID int) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	var attachedIP string
	ips, err := client.ListReservedIPs(context.Background())
	if err == nil {
		for _, ip := range ips {
			if ip.Droplet != nil && ip.Droplet.ID == drID {
				attachedIP = ip.IP
				break
			}
		}
	}

	text := "<b>[注意] 确认删除实例？</b>\n\n此操作不可逆，实例的所有数据将被永久清除。"
	var markup tgbotapi.InlineKeyboardMarkup

	if attachedIP != "" {
		text += fmt.Sprintf("\n\n[注意] 检测到绑定 IP: <code>%s</code>\n未分配的 Reserved IP 将产生费用。", attachedIP)
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("仅删实例 (保留 IP)", fmt.Sprintf("dr_del_conf:%d:%d:keep", accID, drID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("双删 (实例 + IP)", fmt.Sprintf("dr_del_conf:%d:%d:del_ip:%s", accID, drID, attachedIP)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("取消", fmt.Sprintf("dr_info:%d:%d", accID, drID)),
			),
		)
	} else {
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("[注意] 确认删除", fmt.Sprintf("dr_del_conf:%d:%d:keep", accID, drID)),
				tgbotapi.NewInlineKeyboardButtonData("取消", fmt.Sprintf("dr_info:%d:%d", accID, drID)),
			),
		)
	}

	edit := tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, text)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	h.Bot.Send(edit)
}

func (h *Handler) executeDeleteDroplet(query *tgbotapi.CallbackQuery, accID int64, drID int, action string, ipToDelete string) {
	acc, _ := h.DB.GetAccount(accID)
	client := do.NewClient(acc.Token)

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[注意]正在发送删除请求..."))

	err := client.DeleteDroplet(context.Background(), drID)
	if err != nil {
		h.sendText(query.From.ID, "删除实例失败: "+err.Error())
		return
	}

	h.DB.DeleteDroplet(drID)

	msgText := "[正确] 实例删除请求已发送，正在销毁..."

	if action == "del_ip" && ipToDelete != "" {
		go func() {
			time.Sleep(5 * time.Second)
			err := client.DeleteReservedIP(context.Background(), ipToDelete)
			if err != nil {
				h.sendText(query.From.ID, fmt.Sprintf("[错误] IP 删除失败 (%s): %v", ipToDelete, err))
			} else {
				h.sendText(query.From.ID, fmt.Sprintf("[正确] IP 删除成功: %s", ipToDelete))
			}
		}()
		msgText += "\n(关联 IP 正在后台删除)"
	}

	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, msgText))
}

func (h *Handler) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	h.Bot.Send(msg)
}

func (h *Handler) deleteAccount(query *tgbotapi.CallbackQuery, id int64) {
	err := h.DB.DeleteAccount(id)
	if err != nil {
		h.sendText(query.From.ID, "删除账号失败")
		return
	}
	h.Bot.Send(tgbotapi.NewEditMessageText(query.From.ID, query.Message.MessageID, "[正确] 账号已删除"))
}
