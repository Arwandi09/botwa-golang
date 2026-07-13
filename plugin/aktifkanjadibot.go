package plugin

import (
    "strconv"
    "strings"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
)

func init() {
    Register(Plugin{
        Command: "aktifkanjadibot",
        Desc:    "[Owner] Aktifkan kembali seluruh sesi jadibot yang tersimpan",
        Run: func(client *whatsmeow.Client, m *events.Message, _ []string) {
            if !isOwner(m) {
                reply(client, m, "❌ Perintah ini khusus owner bot.")
                return
            }

            reply(client, m, "⏳ Mengaktifkan seluruh sesi jadibot yang tersimpan...")

            go func() {
                activated := ActivateAllJadibot(client, m.Info.Chat)

                if len(activated) == 0 {
                    sendText(client, m.Info.Chat, "⚠️ Tidak ada sesi baru untuk diaktifkan (mungkin semua sudah aktif atau belum ada sesi tersimpan).")
                    return
                }

                list := "• " + strings.Join(activated, "\n• ")
                sendText(client, m.Info.Chat, "✅ Berhasil mengaktifkan "+strconv.Itoa(len(activated))+" sesi jadibot:\n"+list)
            }()
        },
    })
}