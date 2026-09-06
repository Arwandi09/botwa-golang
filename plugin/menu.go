package plugin

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// menuCategory memetakan nama command ke kategori tampilan di menu.
// Command yang tidak ada di daftar ini otomatis masuk ke kategori "Lainnya".
// Kalau nanti nambah plugin baru, tinggal tambahkan satu baris di sini
// supaya tidak nyasar ke "Lainnya".
var menuCategory = map[string]string{
	// Download
	"ig":       "📥 Download",
	"igpp":     "📥 Download",
	"ytsearch": "📥 Download",
	"yta":      "📥 Download",
	"ytv":      "📥 Download",
	"rvo":      "📥 Download",
	"tiktok":   "📥 Download",

	// Grup & Fun
	"hidetag": "🎉 Grup & Fun",
	"brat":    "🎉 Grup & Fun",

	// Jadibot
	"jadibot":          "🤖 Jadibot",
	"aktifkanjadibot":  "🤖 Jadibot",
	"stopjadibot":      "🤖 Jadibot",
	"stopsemuajadibot": "🤖 Jadibot",

	// Owner / Maintenance
	"restart":   "🛠️ Owner & Maintenance",
	"pluginadd": "🛠️ Owner & Maintenance",
	"pluginedit": "🛠️ Owner & Maintenance",
	"pluginrm":  "🛠️ Owner & Maintenance",
	"mkdir":     "🛠️ Owner & Maintenance",
	"mkfile":    "🛠️ Owner & Maintenance",

	// Umum
	"menu":       "ℹ️ Umum",
	"ping":       "ℹ️ Umum",
	"info":       "ℹ️ Umum",
	"antidelete": "ℹ️ Umum",
}

// menuCategoryOrder menentukan urutan tampil tiap kategori di menu.
var menuCategoryOrder = []string{
	"ℹ️ Umum",
	"📥 Download",
	"🎉 Grup & Fun",
	"🤖 Jadibot",
	"🛠️ Owner & Maintenance",
	"❓ Lainnya",
}

func init() {
	Register(Plugin{
		Command: "menu",
		Desc:    "Menampilkan menu",
		Run:     showMenu,
	})
}

func showMenu(client *whatsmeow.Client, m *events.Message, _ []string) {
	// Kelompokkan semua plugin berdasarkan kategori
	grouped := map[string][]Plugin{}
	for _, p := range Plugins {
		cat, ok := menuCategory[p.Command]
		if !ok {
			cat = "❓ Lainnya"
		}
		grouped[cat] = append(grouped[cat], p)
	}

	// Urutkan command secara alfabetis di dalam masing-masing kategori
	for cat := range grouped {
		sort.Slice(grouped[cat], func(i, j int) bool {
			return grouped[cat][i].Command < grouped[cat][j].Command
		})
	}

	var b strings.Builder
	b.WriteString("📜 *MENU BOT*\n")
	b.WriteString("-------------------\n")

	for _, cat := range menuCategoryOrder {
		items, ok := grouped[cat]
		if !ok || len(items) == 0 {
			continue
		}

		b.WriteString("\n*" + cat + "*\n")
		for _, p := range items {
			b.WriteString("  • !" + p.Command + " — " + p.Desc + "\n")
		}
	}

	b.WriteString("\n_Ketik salah satu command di atas untuk mencobanya._")

	_, err := client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			Conversation: proto.String(b.String()),
		},
	)
	if err != nil {
		fmt.Printf("[MENU] gagal kirim balasan ke %s: %v\n", m.Info.Chat.String(), err)
	}
}
