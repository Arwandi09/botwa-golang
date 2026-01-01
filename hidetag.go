package plugin

import (
    "context"
    "fmt"
    "strings"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
    waProto "go.mau.fi/whatsmeow/binary/proto"
)

func init() {
    Register(Plugin{
        Command: "hidetag",
        Desc:    "Mention semua member grup secara tersembunyi",
        Run:     HidetagCommand,
    })
}

func HidetagCommand(client *whatsmeow.Client, m *events.Message, args []string) {
    ctx := context.Background()

    // Pastikan pesan dari grup
    if !m.Info.IsGroup {
        client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
            ExtendedTextMessage: &waProto.ExtendedTextMessage{
                Text: StringPtr("❌ Perintah ini hanya bisa digunakan di grup!"),
            },
        })
        return
    }

    // Ambil info grup untuk mendapatkan daftar member
    groupInfo, err := client.GetGroupInfo(ctx, m.Info.Chat)
    if err != nil {
        client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
            ExtendedTextMessage: &waProto.ExtendedTextMessage{
                Text: StringPtr("❌ Gagal mengambil info grup: " + err.Error()),
            },
        })
        return
    }

    // Buat daftar JID untuk mention
    var mentions []string
    for _, participant := range groupInfo.Participants {
        mentions = append(mentions, participant.JID.String())
    }

    // Text yang akan dikirim (bisa custom dari args atau default)
    text := "📢 Announcement untuk semua member!"
    if len(args) > 0 {
        text = strings.Join(args, " ")
    }

    // Kirim pesan dengan mention tersembunyi
    _, err = client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
        ExtendedTextMessage: &waProto.ExtendedTextMessage{
            Text: StringPtr(text),
            ContextInfo: &waProto.ContextInfo{
                MentionedJID: mentions,
            },
        },
    })

    if err != nil {
        fmt.Println("❌ Error mengirim hidetag:", err)
    }
}