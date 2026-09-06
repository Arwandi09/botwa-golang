package plugin

import (
    "strconv"
    "strings"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

func init() {
    Register(Plugin{
        Command: "stopsemuajadibot",
        Desc:    "[Owner] Hentikan seluruh sesi jadibot yang sedang aktif",
        Run: func(client *whatsmeow.Client, m *events.Message, _ []string) {
            if !isOwner(m) {
                reply(client, m, "❌ Perintah ini khusus owner bot.")
                return
            }

            jadibotMu.Lock()
            numbers := make([]string, 0, len(JadibotConns))
            for n := range JadibotConns {
                numbers = append(numbers, n)
            }
            jadibotMu.Unlock()

            if len(numbers) == 0 {
                reply(client, m, "⚠️ Tidak ada sesi jadibot yang sedang aktif.")
                return
            }

            var stopped []string
            for _, n := range numbers {
                if StopJadibotSession(n) {
                    stopped = append(stopped, n)
                }
            }

            list := "• " + strings.Join(stopped, "\n• ")
            reply(client, m, "🛑 "+strconv.Itoa(len(stopped))+" sesi jadibot dihentikan:\n"+list+"\n\nSemua file sesi tetap aman.")
        },
    })
}