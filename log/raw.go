package log

import (
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

const (
	Reset   = "\x1b[0m"
	Red     = "\x1b[31m"
	Green   = "\x1b[32m"
	Yellow  = "\x1b[33m"
	Blue    = "\x1b[34m"
	Magenta = "\x1b[35m"
	Cyan    = "\x1b[36m"
	Gray    = "\x1b[90m"
)

// Raw menerima prefix sebagai parameter
func Raw(m *events.Message, prefix string) {
	// 1. Ambil Waktu & Tanggal
	timeStr := time.Now().Format("15:04:05")
	dateStr := time.Now().Format("02/01/06")

	// 2. Ambil Nomor WA & Nama WA
	fromID := m.Info.Sender.User 
	name := m.Info.PushName
	if name == "" {
		name = "No Name"
	}
	
	chatJID := m.Info.Chat.String()

	// 3. Ambil Isi Pesan
	text := ""
	if m.Message.GetConversation() != "" {
		text = m.Message.GetConversation()
	} else if m.Message.GetExtendedTextMessage() != nil {
		text = m.Message.GetExtendedTextMessage().GetText()
	} else {
		text = "[Media / Non-Text Message]"
	}

	// 4. Cek jenis log (MSG atau CMD)
	logType := "MSG"
	typeColor := Green
	if prefix != "" && strings.HasPrefix(text, prefix) {
		logType = "CMD"
		typeColor = Cyan
	}

	// 5. Tentukan Ruang Chat (Grup / Privat)
	chatTarget := ""
	if m.Info.IsGroup {
		chatTarget = fmt.Sprintf("%s[GROUP: %s]%s", Magenta, chatJID, Reset)
	} else {
		chatTarget = fmt.Sprintf("%s[PRIVATE]%s", Blue, Reset)
	}

	// 6. Cetak ke terminal (Sudah diperbaiki posisinya agar Nama dan Nomor tidak tertukar)
	fmt.Printf(
		"%s[%s]%s %s %s │ %s%-15s%s │ %s%-15s%s │ %s │ %s\n",
		typeColor, logType, Reset,       // Kolom 1: [MSG] / [CMD]
		timeStr, Gray+dateStr+Reset,     // Kolom 2: Jam & Tanggal
		Yellow, name, Reset,             // Kolom 3: Nama WhatsApp (Kuning)
		Gray, fromID, Reset,             // Kolom 4: Nomor WhatsApp (Abu-abu)
		chatTarget,                      // Kolom 5: [PRIVATE] / [GROUP]
		text,                            // Kolom 6: Isi Pesan
	)
}
