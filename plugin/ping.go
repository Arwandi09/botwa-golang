package plugin

import (
    "context"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/types/events"
    waProto "go.mau.fi/whatsmeow/binary/proto"
    "google.golang.org/protobuf/proto"
)

func init() {
    Register(Plugin{
        Command: "ping",
        Desc:    "Tes respon bot",
        Run: func(client *whatsmeow.Client, m *events.Message, _ []string) {
            client.SendMessage(
                context.Background(),
                m.Info.Chat,
                &waProto.Message{
                    Conversation: proto.String("🏓 Pong!"),
                },
            )
        },
    })
}