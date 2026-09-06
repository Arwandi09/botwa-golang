package plugin

import (
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

func init() {
    Register(Plugin{
        Command: "stopjadibot",
        Desc:    "[Owner] Hentikan sesi jadibot tertentu (file sesi tidak dihapus)",
        Run: func(client *whatsmeow.Client, m *events.Message, args []string) {
            if !isOwner(m) {
                reply(client, m, "❌ Perintah ini khusus owner bot.")
                return
            }

            if len(args) == 0 {
                reply(client, m, "❌ Masukkan nomor yang valid! Contoh: .stopjadibot 628xxxxxxxxxx")
                return
            }

            targetNumber := digitsOnly(args[0])
            if len(targetNumber) < 9 {
                reply(client, m, "❌ Nomor tidak valid!")
                return
            }

            if !StopJadibotSession(targetNumber) {
                reply(client, m, "⚠️ Tidak ada sesi aktif untuk "+targetNumber+".")
                return
            }

            reply(client, m, "🛑 Sesi "+targetNumber+" dihentikan. File sesi tetap aman, pakai .aktifkanjadibot untuk mengaktifkan lagi.")
        },
    })
}