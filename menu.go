package plugin

import (
    "context"
    "strings"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "google.golang.org/protobuf/proto"
)

func init() {
    Register(Plugin{
        Command: "menu",
        Desc:    "Menampilkan menu",
        Run: func(client *whatsmeow.Client, m *events.Message, _ []string) {
            var b strings.Builder
            b.WriteString("📜 *MENU BOT*\n\n")

            for _, p := range Plugins {
                b.WriteString("• !" + p.Command + " — " + p.Desc + "\n")
            }

            client.SendMessage(
                context.Background(),
                m.Info.Chat,
                &waProto.Message{
                    Conversation: proto.String(b.String()),
                },
            )
        },
    })
}