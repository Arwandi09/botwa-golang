package plugin

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// ================= INIT =================

func init() {
	Register(Plugin{
		Command: "restart",
		Desc:    "Restart bot (khusus owner)",
		Run:     restartBot,
	})
}

// ================= RESTART =================

// restartBot menghentikan lalu menjalankan ulang proses bot dari awal,
// tanpa perlu bantuan process manager eksternal (pm2/systemd/dll).
// Hanya bisa dipanggil oleh owner (lihat isOwner() di jadibot.go).
func restartBot(client *whatsmeow.Client, m *events.Message, args []string) {
	if !isOwner(m) {
		reply(client, m, "❌ Hanya owner yang bisa merestart bot.")
		return
	}

	reply(client, m, "🔄 Bot sedang restart, tunggu beberapa detik...")

	// Restart dijalankan di goroutine terpisah dengan sedikit jeda,
	// supaya pesan "sedang restart" di atas sempat benar-benar terkirim
	// dulu ke WhatsApp sebelum proses digantikan.
	go func() {
		time.Sleep(2 * time.Second)

		exePath, err := os.Executable()
		if err != nil {
			fmt.Println("[RESTART] gagal mengambil path executable:", err)
			os.Exit(1)
		}

		env := os.Environ()

		fmt.Printf("[RESTART] mengganti proses sekarang (PID=%d) -> %s\n", os.Getpid(), exePath)

		// syscall.Exec mengganti proses yang sedang berjalan (PID sama)
		// dengan proses baru dari binary yang sama, seolah-olah bot
		// dijalankan ulang dari awal (fresh start, semua variabel di
		// memori ke-reset).
		if err := syscall.Exec(exePath, os.Args, env); err != nil {
			fmt.Println("[RESTART] gagal exec ulang:", err)
			os.Exit(1)
		}
	}()
}
